package models

import (
	"encoding/json"
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
type ManifestExport struct {
	ID           int             `db:"id" json:"id"`
	URL          string          `db:"url" json:"url"`
	Status       string          `db:"status" json:"status"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updated_at"`
	ManifestJSON json.RawMessage `db:"manifest_json" json:"manifest_json"`
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
	v1.Project

	// Overrides v1.Project fields.
	Licenses         pq.StringArray `json:"licenses" db:"licenses"`
	Tags             pq.StringArray `json:"tags" db:"tags"`
	WebpageURLRaw    string         `json:"-" db:"webpageUrl"`
	RepositoryURLRaw string         `json:"-" db:"repositoryUrl"`

	// Entity.
	EntityRaw json.RawMessage `json:"-" db:"entity"`
	Entity    Entity          `json:"entity" db:"-"`

	ID        int       `db:"id" json:"id"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
	Total     int       `db:"total" json:"-"`
}

//easyjson:json
type Entity struct {
	// Manifest fields.
	ManifestID   int    `json:"manifest_id" db:"manifest_id"`
	ManifestGUID string `json:"manifest_guid" db:"manifest_guid"`
	NumProjects  int    `json:"num_projects" db:"num_projects"`

	Type        string `json:"type" db:"type"`
	Role        string `json:"role" db:"role"`
	Name        string `json:"name" db:"name"`
	Email       string `json:"email" db:"email"`
	Phone       string `json:"phone" db:"phone"`
	Description string `json:"description" db:"description"`
	WebpageURL  v1.URL `json:"webpageUrl" db:"-"`

	WebpageURLRaw          string `json:"-" db:"webpageUrl"`
	WebpageURLWellKnownRaw string `json:"-" db:"webpageWellknown"`
	Total                  int    `db:"total" json:"-"`
}
