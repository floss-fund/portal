package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"

	"github.com/floss-fund/go-funding-json/common"
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

// tplData is the data container that is injected
// into public templates for accessing data.
type tplData struct {
	RootURL string
	Data    interface{}
}

type page struct {
	Title       string
	Description string
	MetaTags    string
	Heading     string
	ErrMessage  string
	Message     string
}

func handleIndexPage(c echo.Context) error {
	return c.Render(http.StatusOK, "index", page{Title: "Discover FOSS projects seeking funding"})
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
		_, err := common.IsURL("url", mUrl, 1024)
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
	const title = "Submit funding manifest"

	msg := ""
	if c.Request().Method == http.MethodPost {
		var (
			app  = c.Get("app").(*App)
			mURL = c.FormValue("url")
		)

		u, err := common.IsURL("url", mURL, 1024)
		if err != nil {
			return c.Render(http.StatusBadRequest, "submit", page{Title: title, ErrMessage: err.Error()})
		}

		if !strings.HasSuffix(u.Path, app.consts.ManifestURI) {
			return c.Render(http.StatusBadRequest, "submit", page{Title: title, ErrMessage: fmt.Sprintf("URL must end in %s", app.consts.ManifestURI)})
		}

		// Fetch and validate the manifest.
		m, err := app.crawl.FetchManifest(u)
		if err != nil {
			return c.Render(http.StatusBadRequest, "submit", page{Title: title, ErrMessage: err.Error()})
		}

		// Add it to the database.
		if _, err := app.core.UpsertManifest(m); err != nil {
			return c.Render(http.StatusBadRequest, "submit", page{Title: title, Message: "Error saving manifest to database. Retry later."})
		}
		msg = "done"
	}

	return c.Render(http.StatusOK, "submit", page{Title: title, Message: msg})
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

// Render executes and renders a template for echo.
func (t *tplRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.tpl.ExecuteTemplate(w, name, tplData{
		RootURL: t.RootURL,
		Data:    data,
	})
}
