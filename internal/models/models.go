package models

import (
	"net/url"
	"time"

	v1 "github.com/floss-fund/go-funding-json/schemas/v1"
	"github.com/jmoiron/sqlx/types"
	"github.com/lib/pq"
)

type ManifestJob struct {
	ID           int       `json:"id" db:"id"`
	URL          string    `json:"url" db:"url"`
	Status       string    `json:"status" db:"status"`
	LastModified time.Time `json:"updated_at" db:"updated_at"`

	URLobj *url.URL `json:"-" db:"-"`
}

//easyjson:json
type ManifestData struct {
	v1.Manifest

	// These are not in the table and are added by the get-manifest query.
	EntityRaw   types.JSONText `db:"entity_raw" json:"-"`
	ProjectsRaw types.JSONText `db:"projects_raw" json:"-"`
	FundingRaw  types.JSONText `db:"funding_raw" json:"-"`

	Channels map[string]v1.Channel `db:"-" json:"-"`

	ID            int            `db:"id" json:"id"`
	GUID          string         `db:"guid" json:"guid"`
	Version       string         `db:"version" json:"version"`
	URL           string         `db:"url" json:"url"`
	Meta          types.JSONText `db:"meta" json:"meta"`
	Status        string         `db:"status" json:"status"`
	StatusMessage *string        `db:"status_message" json:"status_message"`
	CrawlErrors   int            `db:"crawl_errors" json:"crawl_errors"`
	CrawlMessage  *string        `db:"crawl_message" json:"crawl_message"`
	CreatedAt     time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time      `db:"updated_at" json:"updated_at"`
}

//easyjson:json
type EntityURL struct {
	WebpageURL string `json:"webpage_url"`
}

//easyjson:json
type ProjectURL struct {
	WebpageURL    string `json:"webpage_url"`
	RepositoryURL string `json:"repository_url"`
}

//easyjson:json
type ProjectURLs []ProjectURL

//easyjson:json
type Project struct {
	Total int `db:"total" json:"-"`

	ID                string         `db:"id" json:"id"`
	ManifestID        int            `db:"manifest_id" json:"manifest_id"`
	ManifestGUID      string         `db:"manifest_guid" json:"manifest_guid"`
	EntityName        string         `db:"entity_name" json:"entity_name"`
	EntityType        string         `db:"entity_type" json:"entity_type"`
	EntityNumProjects int            `db:"entity_num_projects" json:"entity_num_projects"`
	Name              string         `db:"name" json:"name"`
	Description       string         `db:"description" json:"description"`
	WebpageURL        string         `db:"webpage_url" json:"webpage_url"`
	RepositoryURL     string         `db:"repository_url" json:"repository_url"`
	Licenses          pq.StringArray `db:"licenses" json:"licenses"`
	Tags              pq.StringArray `db:"tags" json:"tags"`
	UpdatedAt         time.Time      `db:"updated_at" json:"updated_at"`
}

//easyjson:json
type Entity struct {
	Total int `db:"total" json:"-"`

	ID           string    `json:"id" db:"id"`
	ManifestID   int       `json:"manifest_id" db:"manifest_id"`
	ManifestGUID string    `json:"manifest_guid" db:"manifest_guid"`
	Type         string    `json:"type" db:"type"`
	Role         string    `json:"role" db:"role"`
	Name         string    `json:"name" db:"name"`
	Description  string    `json:"description" db:"description"`
	WebpageURL   string    `json:"webpage_url" db:"webpage_url"`
	NumProjects  int       `json:"num_projects" db:"num_projects"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}
