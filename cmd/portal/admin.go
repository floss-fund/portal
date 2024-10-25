package main

import (
	"net/http"
	"strconv"

	"github.com/floss-fund/portal/internal/models"
	"github.com/labstack/echo/v4"
)

const (
	paginationRows = 20
)

func handleAdminManifestsListing(c echo.Context) error {
	var (
		app = c.Get("app").(*App)

		fromRaw      = c.QueryParam("from")
		statusFilter = c.QueryParam("status")
		guidFilter   = c.QueryParam("guid")

		m []models.ManifestData
	)

	// Convert the from parameter to an integer. If it's not a valid
	// integer, default to 0.
	from, err := strconv.Atoi(fromRaw)
	if err != nil {
		from = 0
	}

	// If the GUID filter is not set, get all manifests and apply the
	// pagination / status filters. Otherwise, get the manifest by GUID.
	if guidFilter == "" {
		// Get all manifests.
		res, err := app.core.GetManifests(from, paginationRows, statusFilter)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		m = res
	} else {
		// Get the manifest by GUID.
		res, err := app.core.GetManifest(0, guidFilter, "")
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		m = []models.ManifestData{res}
	}

	// Get the last ID.
	lastID := 0
	if len(m) > 0 {
		lastID = m[len(m)-1].ID
	}

	// Previous Page ID
	prevID := 0
	if from > 0 {
		prevID = from - paginationRows
	}

	out := struct {
		Page
		Manifests []models.ManifestData `json:"manifests"`

		// Pagination
		LastID int
		PrevID int
	}{
		Page: Page{
			Title: "Admin - Pending Manifests",
		},
		Manifests: m,

		LastID: lastID,
		PrevID: prevID,
	}

	return c.Render(http.StatusOK, "admin-view", out)
}

func handleAdminManifestsPage(c echo.Context) error {
	// This handler is only accessible to admins. Attach a flag
	// to the context to be able to check that in the template.
	c.Set("is-admin", true)

	return handleManifestPage(c)
}
