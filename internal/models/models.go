package models

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/floss-fund/go-funding-json/common"
	v1 "github.com/floss-fund/go-funding-json/schemas/v1"
	"github.com/jmoiron/sqlx/types"
	"github.com/lib/pq"
	"gopkg.in/volatiletech/null.v6"
)

const maxURISize = 40
const maxURLLen = 200
const uriWellKnown = "/.well-known/funding-manifest-urls"

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
	// These are not in the table and are added by the get-manifest query.
	FundingRaw types.JSONText `db:"funding_raw" json:"-"`
	Entity     Entity         `db:"-" json:"entity"`
	Projects   Projects       `db:"-" json:"projects"`
	Funding    v1.Funding     `db:"-" json:"funding"`

	Channels map[string]v1.Channel `db:"-" json:"-"`

	ID            int            `db:"id" json:"id"`
	GUID          string         `db:"guid" json:"guid"`
	Version       string         `db:"version" json:"version"`
	URLStr        string         `db:"url" json:"url"`
	URLobj        *url.URL       `db:"-" json:"-"`
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
	GUID        string         `json:"guid" db:"guid"`
	Name        string         `json:"name" db:"name"`
	Description string         `json:"description" db:"description"`
	Licenses    pq.StringArray `json:"licenses" db:"licenses"`
	Tags        pq.StringArray `json:"tags" db:"tags"`

	// URLs.
	WebpageURLStr       string      `json:"webpage_url" db:"webpage_url"`
	WebpageWellKnownStr null.String `json:"webpage_wellknown" db:"webpage_wellknown"`
	WebpageURL          v1.URL      `json:"-" db:"-"`

	RepositoryURLStr       string      `json:"repository_url" db:"repository_url"`
	RepositoryWellKnownStr null.String `json:"repository_wellknown" db:"repository_wellknown"`
	RepositoryURL          v1.URL      `json:"-" db:"-"`

	// Entity.
	EntityRaw json.RawMessage `json:"-" db:"entity"`
	Entity    Entity          `json:"entity" db:"-"`

	ID        int       `db:"id" json:"id"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
	Total     int       `db:"total" json:"-"`
}

//easyjson:json
type Projects []Project

//easyjson:json
type Entity struct {
	// Manifest fields.
	ManifestID     int    `json:"manifest_id" db:"manifest_id"`
	ManifestGUID   string `json:"manifest_guid" db:"manifest_guid"`
	ManifestURLStr string `json:"manifest_url" db:"manifest_url"`
	NumProjects    int    `json:"num_projects" db:"num_projects"`

	Type                string      `json:"type" db:"type"`
	Role                string      `json:"role" db:"role"`
	Name                string      `json:"name" db:"name"`
	Email               string      `json:"email" db:"email"`
	Phone               string      `json:"phone" db:"phone"`
	Description         string      `json:"description" db:"description"`
	WebpageURLStr       string      `json:"webpage_url" db:"webpage_url"`
	WebpageWellKnownStr null.String `json:"webpage_wellknown" db:"webpage_wellknown"`

	// Added by Parse()
	WebpageURL       v1.URL `json:"-" db:"-"`
	ManifestURL      v1.URL `json:"-" db:"-"`
	WebpageURLStatus bool   `json:"-" db:"-"`

	Total int `db:"total" json:"-"`
}

func (p *Project) Parse() error {
	// Parse the entity first, so that the manifest URL is available for well-known checks.
	if err := p.Entity.Parse(); err != nil {
		return err
	}

	// Parse webpage and repository URLs.
	{
		if u, err := common.IsURL("url", p.WebpageURLStr, maxURLLen); err != nil {
			return err
		} else {
			p.WebpageURL = v1.URL{URL: p.WebpageURLStr, URLobj: u}
		}

		// If there's a wellKnown, parse and validate it.
		if p.WebpageWellKnownStr.Valid {
			var wellKnown *url.URL
			if u, err := common.IsURL("webpage_wellknown", p.WebpageWellKnownStr.String, maxURLLen); err == nil {
				wellKnown = u
			}
			_, err := common.WellKnownURL("webpage_wellknown", p.Entity.ManifestURL.URLobj, p.WebpageURL.URLobj, wellKnown, uriWellKnown)
			if err == nil {
				p.WebpageURL.WellKnown = p.WebpageWellKnownStr.String
				p.WebpageURL.WellKnownObj = wellKnown
			}
		}
	}
	{
		if u, err := common.IsURL("reposistory_url", p.RepositoryURLStr, maxURLLen); err != nil {
			return err
		} else {
			p.RepositoryURL = v1.URL{URL: p.RepositoryURLStr, URLobj: u}
		}

		if p.RepositoryWellKnownStr.Valid {
			var wellKnown *url.URL
			if u, err := common.IsURL("repository_wellknown", p.RepositoryWellKnownStr.String, maxURLLen); err == nil {
				wellKnown = u
			}
			_, err := common.WellKnownURL("repository_wellknown", p.Entity.ManifestURL.URLobj, p.RepositoryURL.URLobj, wellKnown, uriWellKnown)
			if err == nil {
				p.RepositoryURL.WellKnown = p.RepositoryWellKnownStr.String
				p.RepositoryURL.WellKnownObj = wellKnown
			}
		}
	}

	return nil
}

// ToSchema converts models.Entity to v1.Entity (go-funding-json schema).
func (e Entity) ToSchema() v1.Entity {
	return v1.Entity{
		Type:        e.Type,
		Role:        e.Role,
		Name:        e.Name,
		Email:       e.Email,
		Phone:       e.Phone,
		Description: e.Description,
		WebpageURL:  e.WebpageURL,
	}
}

// EntityFromSchema converts v1.Entity (go-funding-json schema) to models.Entity.
func EntityFromSchema(o v1.Entity) Entity {
	return Entity{
		Type:                o.Type,
		Role:                o.Role,
		Name:                o.Name,
		Email:               o.Email,
		Phone:               o.Phone,
		Description:         o.Description,
		WebpageURLStr:       o.WebpageURL.URL,
		WebpageURL:          o.WebpageURL,
		WebpageWellKnownStr: null.NewString(o.WebpageURL.WellKnown, o.WebpageURL.WellKnown != ""),
	}
}

func (e *Entity) Parse() error {
	if u, err := common.IsURL("manifest_url", e.ManifestURLStr, maxURLLen); err != nil {
		return err
	} else {
		e.ManifestURL = v1.URL{URL: e.ManifestURLStr, URLobj: u}
	}

	if u, err := common.IsURL("webpage_url", e.WebpageURLStr, maxURLLen); err != nil {
		return err
	} else {
		e.WebpageURL = v1.URL{URL: e.WebpageURLStr, URLobj: u}
	}

	if e.WebpageWellKnownStr.Valid {
		var wellKnown *url.URL
		if u, err := common.IsURL("webpage_wellknown", e.WebpageWellKnownStr.String, maxURLLen); err == nil {
			wellKnown = u
		}
		_, err := common.WellKnownURL("webpage_wellknown", e.ManifestURL.URLobj, e.WebpageURL.URLobj, wellKnown, uriWellKnown)
		if err == nil {
			e.WebpageURL.WellKnown = e.WebpageWellKnownStr.String
			e.WebpageURL.WellKnownObj = wellKnown
		}
	}

	return nil
}

// ToSchema converts models.Projects to v1.Projects (go-funding-json schema).
func (ps Projects) ToSchema() v1.Projects {
	out := make(v1.Projects, len(ps))
	for i, p := range ps {
		out[i] = v1.Project{
			GUID:          p.GUID,
			Name:          p.Name,
			Description:   p.Description,
			WebpageURL:    p.WebpageURL,
			RepositoryURL: p.RepositoryURL,
			Licenses:      p.Licenses,
			Tags:          p.Tags,
		}
	}
	return out
}

// ProjectsFromSchema converts v1.Projects (go-funding-json schema) to models.Projects.
func ProjectsFromSchema(vps v1.Projects) Projects {
	projects := make(Projects, len(vps))
	for i, vp := range vps {
		projects[i] = Project{
			GUID:                   vp.GUID,
			Name:                   vp.Name,
			Description:            vp.Description,
			Licenses:               vp.Licenses,
			Tags:                   vp.Tags,
			WebpageURLStr:          vp.WebpageURL.URL,
			WebpageURL:             vp.WebpageURL,
			WebpageWellKnownStr:    null.NewString(vp.WebpageURL.WellKnown, vp.WebpageURL.WellKnown != ""),
			RepositoryURLStr:       vp.RepositoryURL.URL,
			RepositoryURL:          vp.RepositoryURL,
			RepositoryWellKnownStr: null.NewString(vp.RepositoryURL.WellKnown, vp.RepositoryURL.WellKnown != ""),
		}
	}
	return projects
}

// Parse parses project structs fetched from the DB.
func (ps Projects) Parse() error {
	for n, p := range ps {
		if err := p.Entity.UnmarshalJSON(p.EntityRaw); err != nil {
			return fmt.Errorf("error unmarshalling entity in project %s: %w", p.GUID, err)
		}
		p.EntityRaw = nil

		if err := p.Parse(); err != nil {
			return fmt.Errorf("error parsing project %s: %w", p.GUID, err)
		}
		ps[n] = p
	}

	return nil
}
