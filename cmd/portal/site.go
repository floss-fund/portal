package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/altcha-org/altcha-lib-go"
	"github.com/floss-fund/go-funding-json/common"
	v1 "github.com/floss-fund/go-funding-json/schemas/v1"
	"github.com/floss-fund/portal/internal/core"
	"github.com/floss-fund/portal/internal/models"
	"github.com/floss-fund/portal/internal/search"
	"github.com/labstack/echo/v4"
)

type okResp struct {
	Data interface{} `json:"data"`
}

// tplRenderer wraps a template.tplRenderer for echo.
type tplRenderer struct {
	tpl      *template.Template
	RootURL  string
	AssetVer string
}

type Query struct {
	Query   string   `query:"q"`
	Type    string   `query:"type"`
	Field   string   `query:"field"`
	License []string `query:"license"`
	Page    int      `query:"page"`
}

// tplData is the data container that is injected
// into public templates for accessing data.
type tplData struct {
	RootURL  string
	AssetVer string
	Data     interface{}
}

type Tab struct {
	ID       string
	URL      string
	Label    string
	Selected bool
}

type Page struct {
	Title         string
	Description   string
	Heading       string
	Tabs          []Tab
	EnableCaptcha bool
	ErrMessage    string
	Message       string
}

var (
	reMultiLines = regexp.MustCompile(`\n\n+`)
	errCaptcha   = errors.New("invalid captcha")
)

func handleIndexPage(c echo.Context) error {
	var (
		app = c.Get("app").(*App)
	)

	// Get top tags.
	tags, _ := app.core.GetTopTags(app.consts.HomeNumTags)
	projects, _ := app.search.GetRecentProjects(app.consts.HomeNumProjects)

	out := struct {
		Page
		Index   bool
		Tags    []string
		Results search.Projects
	}{}
	out.Index = true
	out.Title = "Discover FOSS projects seeking funding"
	out.Tags = tags
	out.Results = projects

	return c.Render(http.StatusOK, "index", out)
}

func handleGetTags(c echo.Context) error {
	var (
		app = c.Get("app").(*App)
	)

	tags, _ := app.core.GetTopTags(500)
	return c.JSON(http.StatusOK, okResp{tags})
}

func handleValidatePage(c echo.Context) error {
	var app = c.Get("app").(*App)

	out := Page{Title: "Validate funding manifest", Heading: "Validate"}
	out.Tabs = []Tab{
		{
			ID:    "submit",
			Label: "Submit",
			URL:   fmt.Sprintf("%s/submit", app.consts.RootURL),
		},
		{
			ID:       "validate",
			Label:    "Validate",
			Selected: true,
			URL:      fmt.Sprintf("%s/validate", app.consts.RootURL),
		},
	}

	// Post request with body to validate.
	if c.Request().Method == http.MethodPost {
		var (
			mUrl = c.FormValue("url")
			body = c.FormValue("body")
		)

		// Validate the URL.
		_, err := common.IsURL("url", mUrl, v1.MaxURLLen)
		if err != nil {
			out.ErrMessage = err.Error()
			return c.Render(http.StatusBadRequest, "validate", out)
		}

		if _, err := app.schema.ParseManifest([]byte(body), mUrl, false); err != nil {
			out.ErrMessage = err.Error()
			return c.Render(http.StatusBadRequest, "validate", out)
		}
	}

	out.Message = "Manifest is valid"
	return c.Render(http.StatusOK, "validate", out)
}

func handleSubmitPage(c echo.Context) error {
	var (
		app  = c.Get("app").(*App)
		mURL = c.FormValue("url")
	)

	out := Page{
		Title:         "Submit funding manifest",
		Heading:       "Submit",
		EnableCaptcha: app.consts.EnableCaptcha,
		Tabs: []Tab{
			{
				ID:       "submit",
				Label:    "Submit",
				Selected: true,
				URL:      fmt.Sprintf("%s/submit", app.consts.RootURL),
			},
			{
				ID:    "validate",
				Label: "Validate",
				URL:   fmt.Sprintf("%s/validate", app.consts.RootURL),
			},
		},
	}

	// Render the page.
	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "submit", out)
	}

	// Process submission.
	// Is Captcha enabled?
	if app.consts.EnableCaptcha {
		if err := validateCaptcha(c.FormValue("altcha"), app.consts.CaptchaKey); err != nil {
			out.ErrMessage = "Invalid captcha"
			return c.Render(http.StatusBadRequest, "submit", out)
		}
	}

	u, err := common.IsURL("url", mURL, v1.MaxURLLen)
	if err != nil {
		out.ErrMessage = err.Error()
		return c.Render(http.StatusBadRequest, "submit", out)
	}

	// Remove any ?query params and #hash fragments
	u.RawQuery = ""
	u.RawFragment = ""

	// Check if the domain is disallowed.
	for _, pattern := range app.consts.DisallowedDomains {
		if matchHostname(u.Host, pattern) {
			out.ErrMessage = fmt.Sprintf("The host %s (CDN URL) is not allowed. Please use a fully qualified domain or a path like github.com/user/project...", pattern)
			return c.Render(http.StatusBadRequest, "submit", out)
		}
	}

	if !strings.HasSuffix(u.Path, app.consts.ManifestURI) {
		out.ErrMessage = fmt.Sprintf("URL must end in %s", app.consts.ManifestURI)
		return c.Render(http.StatusBadRequest, "submit", out)
	}

	// See if the manifest is already in the database.
	if status, err := app.core.GetManifestStatus(u.String()); err != nil {
		out.ErrMessage = "Error checking manifest status. Retry later."
		return c.Render(http.StatusBadRequest, "submit", out)
	} else if status != "" {
		switch status {
		case core.ManifestStatusActive:
			out.ErrMessage = "Manifest is already active."
		case core.ManifestStatusPending:
			out.ErrMessage = "Manifest is already submitted and is pending review."
		case core.ManifestStatusBlocked:
			out.ErrMessage = "Manifest URL is blocked and cannot be submitted at this time."
		}

		if out.ErrMessage != "" {
			return c.Render(http.StatusOK, "submit", out)
		}
	}

	// Fetch and validate the manifest.
	m, err := app.crawl.FetchManifest(u)
	if err != nil {
		out.ErrMessage = err.Error()
		return c.Render(http.StatusBadRequest, "submit", out)
	}

	// Add it to the database.
	m.GUID = core.MakeGUID(m.Manifest.URL.URLobj)
	m.GUID = strings.TrimSuffix(m.GUID, app.consts.ManifestURI)

	if err := app.core.UpsertManifest(m, app.consts.DefaultSubmissionstatus); err != nil {
		app.crawl.Callbacks.OnManifestUpdate(m, app.consts.DefaultSubmissionstatus)
		out.ErrMessage = "Error saving manifest to database. Retry later."
		return c.Render(http.StatusBadRequest, "submit", out)
	}

	out.Message = "success"
	return c.Render(http.StatusOK, "submit", out)
}

func handleValidateManifest(c echo.Context) error {
	var (
		app  = c.Get("app").(*App)
		mUrl = c.FormValue("url")
		body = c.FormValue("body")
	)

	m, err := app.schema.ParseManifest([]byte(body), mUrl, false)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	b, err := m.MarshalJSON()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return c.JSON(http.StatusOK, okResp{json.RawMessage(b)})
}

func handleManifestPage(c echo.Context) error {
	var app = c.Get("app").(*App)

	// Depending on whether the page (tpl) is the main landing page of
	// a manifest (entity/projects) or funding, get the manifest GUID
	// by strips parts off the URI.
	var (
		tpl = ""

		// Manifest guid.
		mGuid = c.Request().URL.Path

		// Project guid.
		pGuid = ""

		// Template response.
		out = struct {
			Page
			Manifest models.ManifestData
			Project  v1.Project
		}{}
	)

	if strings.HasPrefix(mGuid, "/view/funding/") {
		// Funding page.
		mGuid = strings.TrimPrefix(mGuid, "/view/funding/")
		tpl = "funding"
		out.Title = "Funding plans for %s"
		out.Description = "Funding plans for free and open source projects by %s"
	} else if strings.HasPrefix(mGuid, "/view/projects/") {
		// Projects page.
		mGuid = strings.TrimPrefix(mGuid, "/view/projects/")
		tpl = "projects"
		out.Title = "Projects by %s"
		out.Description = "Projects by %s looking for free and open source funding"
	} else if strings.HasPrefix(mGuid, "/view/project/") {
		// Single project.
		tpl = "project"

		// Extract the last part of the URI.
		var (
			path = strings.TrimSuffix(strings.TrimPrefix(mGuid, "/view/project/"), "/")
			i    = strings.LastIndex(path, "/")
		)

		if i == -1 {
			return errPage(c, http.StatusNotFound, "", "Bad request", "Invalid project guid.")
		}

		mGuid = path[:i]
		pGuid = path[i+1:]

	} else if strings.HasPrefix(mGuid, "/view/history/") {
		// History page.
		mGuid = strings.TrimPrefix(mGuid, "/view/history/")
		tpl = "history"
		out.Title = "Financial history of projects by %s"
		out.Description = "Financial and funding history of projects by %s"
	} else {
		// Main entity page.
		tpl = "entity"
		mGuid = strings.TrimPrefix(mGuid, "/view/")
		out.Title = " %s - Project funding"
		out.Description = "Fund free and open source projects by %s"
	}

	// Get the manifest.
	m, err := app.core.GetManifest(0, mGuid)
	if err != nil {
		if err == core.ErrNotFound {
			return errPage(c, http.StatusNotFound, "", "Manifest not found", err.Error())
		}
		return errPage(c, http.StatusInternalServerError, "", "Error", "Error fetching manifest.")
	}

	// If it's a single project's page, get the project.
	var prj v1.Project
	if pGuid != "" {
		idx := slices.IndexFunc(m.Manifest.Projects, func(o v1.Project) bool {
			return o.GUID == pGuid
		})
		if idx < 0 {
			return errPage(c, http.StatusNotFound, "", "Project not found", "Project not found.")
		}
		prj = m.Manifest.Projects[idx]
		out.Title = prj.Name + "by %s"
		out.Description = abbrev(prj.Description, 200)
	}

	out.Manifest = m
	out.Project = prj
	out.Title = fmt.Sprintf(out.Title, m.Manifest.Entity.Name)
	out.Description = fmt.Sprintf(out.Description, m.Manifest.Entity.Name)
	out.Heading = m.Manifest.Entity.Name
	out.Tabs = []Tab{
		{
			ID:       "entity",
			Label:    "Entity",
			Selected: tpl == "entity",
			URL:      fmt.Sprintf("%s/view/%s", app.consts.RootURL, m.GUID),
		},
		{
			ID:       "projects",
			Label:    fmt.Sprintf("Projects (%d)", len(m.Manifest.Projects)),
			Selected: tpl == "projects",
			URL:      fmt.Sprintf("%s/view/projects/%s", app.consts.RootURL, m.GUID),
		},
		{
			ID:       "funding",
			Selected: tpl == "funding",
			Label:    fmt.Sprintf("Funding plans (%d)", len(m.Manifest.Funding.Plans)),
			URL:      fmt.Sprintf("%s/view/funding/%s", app.consts.RootURL, m.GUID),
		},
		{
			ID:       "history",
			Selected: tpl == "history",
			Label:    fmt.Sprintf("History (%d)", len(m.Manifest.Funding.History)),
			URL:      fmt.Sprintf("%s/view/history/%s", app.consts.RootURL, m.GUID),
		},
	}

	// If the view is for a single project, add a tab for that too.
	if pGuid != "" {
		out.Title = fmt.Sprintf("%s by %s - Funding", prj.Name, m.Entity.Name)
		out.Description = fmt.Sprintf("Free and open source funding for %s by %s", prj.Name, m.Entity.Name)
		out.Heading = prj.Name
		out.Tabs = append(out.Tabs, Tab{
			ID:       "project",
			Selected: true,
			Label:    prj.Name,
			URL:      fmt.Sprintf("%s/view/projects/%s/%s", app.consts.RootURL, m.GUID, prj.GUID),
		})

		return c.Render(http.StatusOK, tpl, out)
	}

	return c.Render(http.StatusOK, tpl, out)
}

func handleSearchPage(c echo.Context) error {
	var (
		app = c.Get("app").(*App)
	)

	var q Query
	if err := c.Bind(&q); err != nil {
		return errPage(c, http.StatusBadRequest, "", "Invalid request", "Invalid request.")
	}
	q.Query = strings.TrimSpace(q.Query)

	if q.Query == "" || len(q.Query) > 128 {
		return c.Redirect(http.StatusTemporaryRedirect, app.consts.RootURL)
	}
	if q.Page < 1 {
		q.Page = 1
	}

	var (
		results any
		total   int
	)
	switch q.Type {
	case "entity":
		query := search.EntityQuery{Query: q.Query, Field: q.Field, Page: q.Page}

		o, num, err := app.search.SearchEntities(query)
		if err != nil {
			return errPage(c, http.StatusBadRequest, "", "Error", "An internal error occurred while searching.")
		}
		results = o
		total = num
	case "project":
		query := search.ProjectQuery{Query: q.Query, Field: q.Field, Page: q.Page}
		query.Licenses = []string{}

		for _, l := range c.QueryParams()["license"] {
			query.Licenses = append(query.Licenses, l)
		}

		o, num, err := app.search.SearchProjects(query)
		if err != nil {
			return errPage(c, http.StatusBadRequest, "", "Error", "An internal error occurred while searching.")
		}
		results = o
		total = num
	default:
		return errPage(c, http.StatusBadRequest, "", "Error", "Unknown type.")
	}

	pg := app.pg.NewFromURL(c.Request().URL.Query())
	pg.SetTotal(total)

	out := struct {
		Page
		Pagination template.HTML
		Q          Query
		Total      int
		Results    interface{}
	}{}

	// Additional query params to attach to paginated URLs.
	qp := url.Values{}
	qp.Set("q", q.Query)
	qp.Set("type", q.Type)
	qp.Set("field", q.Field)

	out.Pagination = template.HTML(pg.HTML("", qp))
	out.Title = "Search"
	out.Heading = fmt.Sprintf(`Search "%s"`, q.Query)
	out.Q = q
	out.Total = total
	out.Results = results

	return c.Render(http.StatusOK, "search", out)
}

func handleReport(c echo.Context) error {
	var (
		app    = c.Get("app").(*App)
		reason = c.FormValue("reason")
		mGuid  = c.Param("mguid")
	)

	if len(reason) > 300 {
		return c.Render(http.StatusOK, "report-submit", struct{ ErrMessage string }{"Character limit exceeded. Should be less than 300."})
	}

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "report", struct {
			RootURL       string
			MGUID         string
			EnableCaptcha bool
		}{
			RootURL:       app.consts.RootURL,
			MGUID:         mGuid,
			EnableCaptcha: app.consts.EnableCaptcha,
		})
	}

	if app.consts.EnableCaptcha {
		if err := validateCaptcha(c.FormValue("altcha"), app.consts.CaptchaKey); err != nil {
			return c.Render(http.StatusOK, "report-submit", struct{ ErrMessage string }{"Invalid Captcha"})
		}
	}

	manifest, err := app.core.GetManifest(0, mGuid)
	if err != nil {
		return c.Render(http.StatusOK, "report-submit", struct{ ErrMessage string }{"Could not get manifest"})
	}

	err = app.core.InsertManifestReport(manifest.ID, reason)
	if err != nil {
		return c.Render(http.StatusOK, "report-submit", struct{ ErrMessage string }{"An internal error occurred while submitting the report."})
	}

	return c.Render(http.StatusOK, "report-submit", struct{ ErrMessage string }{})
}

// Render executes and renders a template for echo.
func (t *tplRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.tpl.ExecuteTemplate(w, name, tplData{
		RootURL:  t.RootURL,
		AssetVer: t.AssetVer,
		Data:     data,
	})
}

func errPage(c echo.Context, code int, tpl, title, message string) error {
	if tpl == "" {
		tpl = "message"
	}

	return c.Render(code, tpl, Page{Title: title, ErrMessage: message})
}

func abbrev(str string, ln int) string {
	if len(str) < ln {
		return str
	}

	return str[:ln] + ".."
}

func validateCaptcha(payload string, key string) error {
	ok, err := altcha.VerifySolution(payload, key, false)
	if err != nil {
		return err
	}

	if !ok {
		return errCaptcha
	}

	return nil
}

func matchHostname(host, pattern string) bool {
	if strings.HasPrefix(pattern, "*.") {
		domain := pattern[2:]
		if host == domain {
			return false
		}
		if strings.HasSuffix(host, "."+domain) {
			return true
		}
	} else {
		if host == pattern {
			return true
		}
	}

	return false
}
