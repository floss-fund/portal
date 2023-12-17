package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io/ioutil"
	mrand "math/rand"
	"net/http"
	"os"
	"unicode"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
	"github.com/labstack/echo/v4"
)

func initConstants(ko *koanf.Koanf) Consts {
	c := Consts{
		RootURL: ko.MustString("app.root_url"),
	}

	return c
}

// initDB initializes a database connection.
func initDB(host string, port int, user, pwd, dbName string) *sqlx.DB {
	db, err := sqlx.Connect("postgres",
		fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, pwd, dbName))
	if err != nil {
		lo.Fatalf("error initializing DB: %v", err)
	}

	return db
}

// initFS initializes the stuffbin FileSystem to provide
// access to bunded static assets to the app.
func initFS() stuffbin.FileSystem {
	path, err := os.Executable()
	if err != nil {
		lo.Fatalf("error getting executable path: %v", err)
	}

	fs, err := stuffbin.UnStuff(path)
	if err == nil {
		return fs
	}

	// Running in local mode. Load the required static assets into
	// the in-memory stuffbin.FileSystem.
	lo.Printf("unable to initialize embedded filesystem: %v", err)
	lo.Printf("using local filesystem for static assets")

	files := []string{
		"config.sample.toml",
		"queries.sql",
		"schema.sql",
	}

	fs, err = stuffbin.NewLocalFS("/", files...)
	if err != nil {
		lo.Fatalf("failed to load local static files: %v", err)
	}

	return fs
}

func initHTTPServer(app *App, ko *koanf.Koanf) *echo.Echo {
	srv := echo.New()
	srv.Debug = true
	srv.HideBanner = true

	// Register app (*App) to be injected into all HTTP handlers.
	srv.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("app", app)
			return next(c)
		}
	})

	// Public handlers with no auth.
	p := srv.Group("")

	// Admin handlers and APIs.
	p.GET("/", handleIndexPage)

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

	return srv
}

func generateNewFiles() error {
	if _, err := os.Stat("config.toml"); !os.IsNotExist(err) {
		return errors.New("config.toml exists. Remove it to generate a new one")
	}

	// Initialize the static file system into which all
	// required static assets (.sql, .js files etc.) are loaded.
	fs := initFS()

	// Generate config file.
	b, err := fs.Read("config.sample.toml")
	if err != nil {
		return fmt.Errorf("error reading sample config (is binary stuffed?): %v", err)
	}

	// Inject a random password.
	p := make([]byte, 12)
	rand.Read(p)
	pwd := []byte(fmt.Sprintf("%x", p))

	for i, c := range pwd {
		if mrand.Intn(4) == 1 {
			pwd[i] = byte(unicode.ToUpper(rune(c)))
		}
	}

	b = bytes.Replace(b, []byte("dictpress_admin_password"), pwd, -1)

	if err := ioutil.WriteFile("config.toml", b, 0644); err != nil {
		return err
	}

	return nil
}
