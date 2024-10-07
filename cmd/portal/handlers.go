package main

import (
	"crypto/subtle"
	"net/http"
	"path"
	"strconv"

	"github.com/knadh/koanf/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const isAuthed = "is_authed"

func initHandlers(ko *koanf.Koanf, srv *echo.Echo) {
	g := srv.Group("")
	g.GET("/", handleIndexPage)
	g.GET("/submit", handleSubmitPage)
	g.POST("/submit", handleSubmitPage)
	g.GET("/validate", handleValidatePage)
	g.POST("/validate", handleValidatePage)

	g.POST("/api/validate", handleValidateManifest)
	g.GET("/api/tags", handleGetTags)

	// Authenticated endpoints.
	a := srv.Group("", middleware.BasicAuth(basicAuth))
	a.GET("/api/manifests/:id", handleGetManifest)
	a.PUT("/api/manifests/:id/status", handleUpdateManifestStatus)

	// Static files.
	g.Static("/static", path.Join(ko.MustString("app.template_dir"), "/static"))

	// 404 pages.
	srv.RouteNotFound("/api/*", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "Unknown endpoint")
	})
	srv.RouteNotFound("/*", func(c echo.Context) error {
		return c.Render(http.StatusNotFound, "message", page{
			Title:   "404 Page not found",
			Heading: "404 Page not found",
		})
	})

}

func handleGetManifest(c echo.Context) error {
	var (
		app   = c.Get("app").(*App)
		id, _ = strconv.Atoi(c.Param("id"))
	)

	out, err := app.core.GetManifest(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, okResp{out})
}

func handleUpdateManifestStatus(c echo.Context) error {
	var (
		app   = c.Get("app").(*App)
		id, _ = strconv.Atoi(c.Param("id"))
	)

	if err := app.core.UpdateManifestStatus(id, c.FormValue("status")); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, okResp{})
}

// basicAuth middleware does an HTTP BasicAuth authentication for admin handlers.
func basicAuth(username, password string, c echo.Context) (bool, error) {
	app := c.Get("app").(*App)

	if subtle.ConstantTimeCompare([]byte(username), app.consts.AdminUsername) == 1 &&
		subtle.ConstantTimeCompare([]byte(password), app.consts.AdminPassword) == 1 {
		c.Set(isAuthed, true)
		return true, nil
	}

	return false, nil
}
