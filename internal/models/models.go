package models

import (
	"net/url"
	"time"

	v1 "github.com/floss-fund/go-funding-json/schemas/v1"
	"github.com/jmoiron/sqlx/types"
)

type ManifestJob struct {
	ID           int       `json:"id" db:"id"`
	URL          string    `json:"url" db:"url"`
	LastModified time.Time `json:"updated_at" db:"updated_at"`

	URLobj *url.URL `json:"-" db:"-"`
}

//easyjson:json
type ManifestDB struct {
	v1.Manifest

	// These are not in the table and are added by the get-manifest query.
	EntityRaw   types.JSONText `db:"entity_raw" json:"-"`
	ProjectsRaw types.JSONText `db:"projects_raw" json:"-"`

	ID            int            `db:"id" json:"id"`
	GUID          string         `db:"guid" json:"guid"`
	Version       string         `db:"version" json:"version"`
	URL           string         `db:"url" json:"url"`
	Funding       types.JSONText `db:"funding" json:"funding"`
	Meta          types.JSONText `db:"meta" json:"meta"`
	Status        string         `db:"status" json:"status"`
	StatusMessage *string        `db:"status_message" json:"status_message"`
	CrawlErrors   int            `db:"crawl_errors" json:"crawl_errors"`
	CrawlMessage  *string        `db:"crawl_message" json:"crawl_message"`
	CreatedAt     time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time      `db:"updated_at" json:"updated_at"`
}
