package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
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
	Tag     string   `query:"tag"`
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

type page struct {
	Page        string
	Title       string
	Description string
	MetaTags    string
	Heading     string
	Tabs        []Tab
	ErrMessage  string
	Message     string
}

func handleIndexPage(c echo.Context) error {
	var (
		app = c.Get("app").(*App)
	)

	tags, _ := app.core.GetTopTags(25)

	out := struct {
		page
		Tags []string
	}{}
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
	const title = "Validate funding.json manifest"

	// Post request with body to validate.
	if c.Request().Method == http.MethodPost {
		var (
			app  = c.Get("app").(*App)
			mUrl = c.FormValue("url")
			body = c.FormValue("body")
		)

		// Validate the URL.
		_, err := common.IsURL("url", mUrl, v1.MaxURLLen)
		if err != nil {
			return c.Render(http.StatusBadRequest, "validate", page{Title: title, ErrMessage: err.Error()})
		}

		if _, err := app.schema.ParseManifest([]byte(body), mUrl, false); err != nil {
			return c.Render(http.StatusBadRequest, "validate", page{Title: title, ErrMessage: err.Error()})
		}
	}

	return c.Render(http.StatusOK, "validate", page{Title: title, Message: "Manifest is valid."})
}

func handleSubmitPage(c echo.Context) error {
	var (
		app  = c.Get("app").(*App)
		mURL = c.FormValue("url")
	)

	out := page{Title: "Submit funding manifest", Heading: "Submit"}

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
	var (
		app = c.Get("app").(*App)
	)

	// Depending on whether the page (tab) is the main landing page of
	// a manifest (entity/projects) or funding, get the manifest GUID
	// by strips parts off the URI.
	var (
		tab = "projects"

		// Manifest guid.
		mGuid = c.Request().URL.Path

		// Project guid.
		pGuid = ""
	)
	if strings.HasPrefix(mGuid, "/view/funding") {
		mGuid = strings.TrimLeft(mGuid, "/view/funding")
		tab = "funding"
	} else if strings.HasPrefix(mGuid, "/view/projects") {
		path := strings.TrimLeft(mGuid, "/view/projects")
		i := strings.LastIndex(path, "/")
		if i == -1 {
			return c.Render(http.StatusBadRequest, "message",
				page{Title: "Manifest not found", ErrMessage: "Invalid project guid."})
		}
		mGuid = path[:i]
		pGuid = path[i+1:]

		tab = "project"
	} else if strings.HasPrefix(mGuid, "/view/history") {
		mGuid = strings.TrimLeft(mGuid, "/view/history")
		tab = "history"
	} else {
		mGuid = strings.TrimLeft(mGuid, "/view")
	}

	// Get the manifest.
	m, err := app.core.GetManifest(0, mGuid)
	if err != nil {
		if err == core.ErrNotFound {
			return c.Render(http.StatusNotFound, "message",
				page{Title: "Manifest not found", ErrMessage: err.Error()})
		}
		return c.Render(http.StatusBadRequest, "message",
			page{Title: "Error", ErrMessage: "Error fetching manifest."})
	}

	// If it's a single project's page, get the project.
	var prj v1.Project
	if pGuid != "" {
		idx := slices.IndexFunc(m.Manifest.Projects, func(o v1.Project) bool {
			return o.GUID == pGuid
		})
		if idx < 0 {
			return c.Render(http.StatusNotFound, "message",
				page{Title: "Project not found", ErrMessage: "Project not found."})
		}
		prj = m.Manifest.Projects[idx]
	}

	out := struct {
		page
		Tab      string
		Manifest models.ManifestData
		Project  v1.Project
	}{}

	out.Page = "manifest"
	out.Tab = tab
	out.Manifest = m
	out.Project = prj
	out.Title = m.Manifest.Entity.Name
	out.Heading = m.Manifest.Entity.Name
	out.Tabs = []Tab{
		{
			ID:       "projects",
			Label:    fmt.Sprintf("Projects (%d)", len(m.Manifest.Projects)),
			Selected: tab == "projects",
			URL:      fmt.Sprintf("%s/view/%s", app.consts.RootURL, m.GUID),
		},
		{
			ID:       "funding",
			Selected: tab == "funding",
			Label:    fmt.Sprintf("Funding plans (%d)", len(m.Manifest.Funding.Plans)),
			URL:      fmt.Sprintf("%s/view/funding/%s", app.consts.RootURL, m.GUID),
		},
		{
			ID:       "history",
			Selected: tab == "history",
			Label:    fmt.Sprintf("History (%d)", len(m.Manifest.Funding.History)),
			URL:      fmt.Sprintf("%s/view/history/%s", app.consts.RootURL, m.GUID),
		},
	}

	// If the view is for a single project, add a tab for that too.
	if pGuid != "" {
		out.Page = "project"
		out.Title = fmt.Sprintf("%s (%s) funding", prj.Name, m.Entity.Name)
		out.Heading = prj.Name
		out.Tabs = append(out.Tabs, Tab{
			ID:       "project",
			Selected: true,
			Label:    prj.Name,
			URL:      fmt.Sprintf("%s/view/projects/%s/%s", app.consts.RootURL, m.GUID, prj.GUID),
		})

		return c.Render(http.StatusOK, "project", out)
	}

	return c.Render(http.StatusOK, "manifest", out)
}

func handleSearchPage(c echo.Context) error {
	const title = "Search"

	var (
		app = c.Get("app").(*App)
	)

	var q Query
	if err := c.Bind(&q); err != nil {
		return c.String(http.StatusBadRequest, "invalid request.")
	}
	q.Query = strings.TrimSpace(q.Query)

	if q.Query == "" || len(q.Query) > 128 {
		return c.Redirect(http.StatusTemporaryRedirect, app.consts.RootURL)
	}

	var results interface{}
	switch q.Type {
	case "entity":
		query := search.EntityQuery{Query: q.Query}
		query.Type = c.FormValue("entity_type")

		o, err := app.search.SearchEntities(query)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, errors.New("error searching"))
		}
		results = o
	case "project":
		query := search.ProjectQuery{Query: q.Query}
		query.Licenses = []string{}

		for _, l := range c.QueryParams()["license"] {
			query.Licenses = append(query.Licenses, l)
		}

		o, err := app.search.SearchProjects(query)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, errors.New("error searching"))
		}
		results = o
	default:
		return echo.NewHTTPError(http.StatusBadRequest, errors.New("unknown `type`"))
	}

	out := struct {
		page
		Q       Query
		Results interface{}
	}{}
	out.Page = "search"
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
