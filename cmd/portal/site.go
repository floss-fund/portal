package main

import (
	"net/http"

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

// handleIndexPage renders the homepage.
func handleIndexPage(c echo.Context) error {
	return c.Render(http.StatusOK, "index", pageTpl{})
}
