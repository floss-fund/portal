package main

import (
	"net/http"
	"strconv"

	"github.com/floss-fund/portal/internal/models"
	"github.com/labstack/echo/v4"
)

func handleAdminManifestsPage(c echo.Context) error {
	var app = c.Get("app").(*App)

	var (
		pageRaw = c.QueryParam("page")
		page    = 1
	)

	if pageRaw == "" {
		pageRaw = "1"
	} else {
		// Parse the page number.
		pageParsed, err := strconv.Atoi(pageRaw)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid page number")
		}

		page = pageParsed
	}

	var (
		perPage = 20
		limit   = perPage
		offset  = (page - 1) * perPage
	)

	// Get all manifests.
	m, err := app.core.GetPendingManifests(limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	out := struct {
		Page
		Manifests []models.ManifestData `json:"manifests"`
	}{
		Page: Page{
			Title: "Admin - Pending Manifests",
		},
		Manifests: m,
	}

	return c.Render(http.StatusOK, "admin-manifests", out)
}
