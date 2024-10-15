package main

import (
	"crypto/subtle"
	"net/http"
	"path"
	"strconv"

	"github.com/altcha-org/altcha-lib-go"
	"github.com/knadh/koanf/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const (
	isAuthed          = "is_authed"
	captchaComplexity = 75000
)

func initHandlers(ko *koanf.Koanf, srv *echo.Echo) {
	g := srv.Group("")
	g.GET("/", handleIndexPage)
	g.GET("/submit", handleSubmitPage)
	g.POST("/submit", handleSubmitPage)
	g.GET("/validate", handleValidatePage)
	g.POST("/validate", handleValidatePage)
	g.GET("/search", handleSearchPage)
	g.GET("/view/funding", handleManifestPage)
	g.GET("/view/projects", handleManifestPage)
	g.GET("/view/project", handleManifestPage)
	g.GET("/view/*", handleManifestPage)

	g.POST("/api/validate", handleValidateManifest)
	g.GET("/api/tags", handleGetTags)
	g.GET("/api/captcha", handleGenerateCaptcha)

	// Static files.
	g.Static("/static", path.Join(ko.MustString("app.template_dir"), "/static"))

	// Private, authenticated endpoints.
	a := srv.Group("", middleware.BasicAuth(basicAuth))
	a.GET("/api/manifests/:id", handleGetManifest)
	a.PUT("/api/manifests/:id/status", handleUpdateManifestStatus)

	// 404 pages.
	srv.RouteNotFound("/api/*", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "Unknown endpoint")
	})
	srv.RouteNotFound("/*", func(c echo.Context) error {
		return c.Render(http.StatusNotFound, "message", Page{
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

	out, err := app.core.GetManifest(id, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, okResp{out})
}

func handleUpdateManifestStatus(c echo.Context) error {
	var (
		app    = c.Get("app").(*App)
		id, _  = strconv.Atoi(c.Param("id"))
		status = c.FormValue("status")
	)

	// Update the status in the DB.
	if err := app.core.UpdateManifestStatus(id, status); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Delete it from search if the status isn't active.
	if m, err := app.core.GetManifest(id, ""); err == nil {
		app.crawl.Callbacks.OnManifestUpdate(m, status)
	}

	return c.JSON(http.StatusOK, okResp{true})
}

func handleGenerateCaptcha(c echo.Context) error {
	var (
		app = c.Get("app").(*App)
	)

	// Create a new challenge.
	ch, err := altcha.CreateChallenge(altcha.ChallengeOptions{
		HMACKey:   app.consts.CaptchaKey,
		MaxNumber: captchaComplexity,
	})
	if err != nil {
		app.lo.Printf("error generating captcha: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error generating captcha")
	}

	return c.JSON(http.StatusOK, ch)
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
