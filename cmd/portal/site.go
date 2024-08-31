package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/floss-fund/go-funding-json/common"
	"github.com/labstack/echo/v4"
)

type okResp struct {
	Data interface{} `json:"data"`
}

type pageTpl struct {
	PageType string
	PageID   string

	Title       string
	Heading     string
	Description string
	MetaTags    string
}

func handleIndexPagex(c echo.Context) error {
	return c.Render(http.StatusOK, "index", pageTpl{})
}

func handleSubmitPage(c echo.Context) error {
	var (
		app  = c.Get("app").(*App)
		mURL = c.FormValue("url")
	)

	u, err := common.IsURL("url", mURL, 1024)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if !strings.HasSuffix(u.Path, app.consts.ManifestURI) {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("URI doesn't end in %s", app.consts.ManifestURI))
	}

	// Fetch and validate the manifest.
	m, err := app.crawl.FetchManifest(u)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Add it to the database.
	if _, err := app.core.UpsertManifest(m); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error saving manifest to database. Retry later.")
	}

	return c.JSON(http.StatusOK, 200)
}

func handleValidateManifest(c echo.Context) error {
	var (
		app  = c.Get("app").(*App)
		mUrl = c.FormValue("url")
		body = c.FormValue("body")
	)

	if _, err := app.schema.ParseManifest([]byte(body), mUrl, false); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return c.JSON(http.StatusOK, okResp{true})
}
