package main

import (
	"fmt"
	"net/http"
	"strings"

	"floss.fund/portal/internal/validations"
	"github.com/labstack/echo/v4"
)

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

	u, err := validations.IsURL("url", mURL, 1024)
	if err != nil {
		return err
	}

	if !strings.HasSuffix(u.Path, app.consts.FundingManifestPath) {
		return fmt.Errorf("URI doesn't end in %s", app.consts.FundingManifestPath)
	}

	return c.JSON(http.StatusOK, 200)
}
