package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"

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
	tpl     *template.Template
	RootURL string
}

type Query struct {
	Query   string   `query:"q"`
	Type    string   `query:"type"`
	Field   string   `query:"field"`
	License []string `query:"license"`
}

// tplData is the data container that is injected
// into public templates for accessing data.
type tplData struct {
	RootURL string
	Data    interface{}
}

type Tab struct {
	ID       string
	URL      string
	Label    string
	Selected bool
}

type Page struct {
	Title       string
	Description string
	Heading     string
	Tabs        []Tab
	ErrMessage  string
	Message     string
}

var (
	reMultiLines = regexp.MustCompile(`\n\n+`)
)

func handleIndexPage(c echo.Context) error {
	var (
		app = c.Get("app").(*App)
	)

	tags, _ := app.core.GetTopTags(25)

	out := struct {
		Page
		Index bool
		Tags  []string
	}{}
	out.Index = true
	out.Title = "Discover FOSS projects seeking funding"
	out.Tags = tags

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

	out := Page{Title: "Submit funding manifest", Heading: "Submit"}
	out.Tabs = []Tab{
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
	}

	// Render the page.
	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "submit", out)
	}

	// Accept submission.
	u, err := common.IsURL("url", mURL, v1.MaxURLLen)
	if err != nil {
		out.ErrMessage = err.Error()
		return c.Render(http.StatusBadRequest, "submit", out)
	}

	// Remove any ?query params and #hash fragments
	u.RawQuery = ""
	u.RawFragment = ""

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
	m.GUID = strings.TrimRight(m.GUID, app.consts.ManifestURI)

	if err := app.core.UpsertManifest(m); err != nil {
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

	if strings.HasPrefix(mGuid, "/view/funding") {
		// Funding page.
		mGuid = strings.TrimLeft(mGuid, "/view/funding")
		tpl = "funding"
		out.Title = "Funding plans for %s"
		out.Description = "Funding plans for free and open source projects by %s"
	} else if strings.HasPrefix(mGuid, "/view/projects") {
		// Projects page.
		mGuid = strings.TrimLeft(mGuid, "/view/projects")
		tpl = "projects"
		out.Title = "Projects by %s"
		out.Description = "Projects by %s looking for free and open source funding"
	} else if strings.HasPrefix(mGuid, "/view/project") {
		// Single project.
		tpl = "project"

		// Extract the last part of the URI.
		var (
			path = strings.TrimLeft(mGuid, "/view/project")
			i    = strings.LastIndex(path, "/")
		)
		if i == -1 {
			return errPage(c, http.StatusNotFound, "", "Manifest not found", "Invalid project guid.")
		}

		mGuid = path[:i]
		pGuid = path[i+1:]
	} else if strings.HasPrefix(mGuid, "/view/history") {
		// History page.
		mGuid = strings.TrimLeft(mGuid, "/view/history")
		tpl = "history"
		out.Title = "Financial history of projects by %s"
		out.Description = "Financial and funding history of projects by %s"
	} else {
		// Main entity page.
		tpl = "entity"
		mGuid = strings.TrimLeft(mGuid, "/view")
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

	var results interface{}
	switch q.Type {
	case "entity":
		query := search.EntityQuery{Query: q.Query, Field: q.Field}

		o, err := app.search.SearchEntities(query)
		if err != nil {
			return errPage(c, http.StatusBadRequest, "", "Error", "An internal error occurred while searching.")
		}
		results = o
	case "project":
		query := search.ProjectQuery{Query: q.Query, Field: q.Field}
		query.Licenses = []string{}

		for _, l := range c.QueryParams()["license"] {
			query.Licenses = append(query.Licenses, l)
		}

		o, err := app.search.SearchProjects(query)
		if err != nil {
			return errPage(c, http.StatusBadRequest, "", "Error", "An internal error occurred while searching.")
		}
		results = o
	default:
		return errPage(c, http.StatusBadRequest, "", "Error", "Unknown type.")
	}

	out := struct {
		Page
		Q       Query
		Results interface{}
	}{}
	out.Title = "Search"
	out.Heading = fmt.Sprintf(`Search "%s"`, q.Query)
	out.Q = q
	out.Results = results

	return c.Render(http.StatusOK, "search", out)
}

// Render executes and renders a template for echo.
func (t *tplRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.tpl.ExecuteTemplate(w, name, tplData{
		RootURL: t.RootURL,
		Data:    data,
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
