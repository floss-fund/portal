package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func initHandlers(srv *echo.Echo) {
	g := srv.Group("")
	g.POST("/submit", handleSubmitPage)

	// 404 pages.
	srv.RouteNotFound("/api/*", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "Unknown endpoint")
	})
	srv.RouteNotFound("/*", func(c echo.Context) error {
		return c.Render(http.StatusNotFound, "message", pageTpl{
			Title:   "404 Page not found",
			Heading: "404 Page not found",
		})
	})

}
