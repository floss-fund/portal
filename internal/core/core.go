package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/floss-fund/go-funding-json/common"
	v1 "github.com/floss-fund/go-funding-json/schemas/v1"
	"github.com/floss-fund/portal/internal/models"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const maxURLLen = 200

var reGithub = regexp.MustCompile(`^(https://github\.com/([^/]+))/([^/]+)/(blob|raw)/([^/]+)`)

type Opt struct {
}

const (
	ManifestStatusPending  = "pending"
	ManifestStatusActive   = "active"
	ManifestStatusExpiring = "expiring"
	ManifestStatusDisabled = "disabled"
	ManifestStatusBlocked  = "blocked"
)

// Queries contains prepared DB queries.
type Queries struct {
	UpsertManifest        *sqlx.Stmt `query:"upsert-manifest"`
	GetManifests          *sqlx.Stmt `query:"get-manifests"`
	GetManifestStatus     *sqlx.Stmt `query:"get-manifest-status"`
	GetForCrawling        *sqlx.Stmt `query:"get-for-crawling"`
	UpdateManifestStatus  *sqlx.Stmt `query:"update-manifest-status"`
	UpdateManifestDate    *sqlx.Stmt `query:"update-manifest-date"`
	UpdateCrawlError      *sqlx.Stmt `query:"update-crawl-error"`
	DeleteManifest        *sqlx.Stmt `query:"delete-manifest"`
	GetTopTags            *sqlx.Stmt `query:"get-top-tags"`
	InsertReport          *sqlx.Stmt `query:"insert-report"`
	GetRecentProjects     string     `query:"get-recent-projects-snippet"`
	GetProjects           string     `query:"get-projects-snippet"`
	GetProjectsByManifest string     `query:"get-projects-by-manifest-snippet"`
	GetEntities           string     `query:"get-entities"`
	GetEntityByManifest   string     `query:"get-entity-by-manifest-snippet"`
	GetManifestsDump      *sqlx.Stmt `query:"get-manifests-dump"`
	SearchEntities        *sqlx.Stmt `query:"search-entities"`
	QueryProjectsTpl      string     `query:"query-projects-template"`
	SearchProjects        string     `query:"search-projects-snippet"`
}

type Core struct {
	q   *Queries
	db  *sqlx.DB
	log *log.Logger
}

var (
	ErrNotFound = errors.New("not found")
)

func New(q *Queries, db *sqlx.DB, o Opt, lo *log.Logger) *Core {
	return &Core{
		q:   q,
		db:  db,
		log: lo,
	}
}

// GetManifest retrieves a particular manifest.
func (c *Core) GetManifest(id int, guid string, status string) (models.ManifestData, error) {
	out, err := c.getManifests(id, guid, 0, 1, status)
	if err != nil || len(out) == 0 {
		return models.ManifestData{}, ErrNotFound
	}

	return out[0], nil
}

// GetManifests retrieves N manifests.
func (c *Core) GetManifests(lastID, limit int, status string) ([]models.ManifestData, error) {
	out, err := c.getManifests(0, "", lastID, limit, status)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// GetManifestStatus checks whether a given manifest URL exists in the databse.
// If one exists, its status is returned.
func (c *Core) GetManifestStatus(url string) (string, error) {
	var status string
	if err := c.q.GetManifestStatus.Get(&status, url); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}

		c.log.Printf("error checking manifest status: %s: %v", url, err)
		return "", err
	}

	return status, nil
}

// UpsertManifest upserts an entry into the database.
func (c *Core) UpsertManifest(m models.ManifestData, status string) error {

	// Build schema to respond to API.
	manifest := v1.Manifest{
		Version:  m.Version,
		Entity:   m.Entity.ToSchema(),
		Projects: m.Projects.ToSchema(),
		Funding:  m.Funding,
	}
	b, err := manifest.MarshalJSON()
	if err != nil {
		c.log.Printf("error marshalling manifest: %s: %v", m.URLStr, err)
		return err
	}

	if _, err := c.q.UpsertManifest.Exec(json.RawMessage(b), m.URLStr, m.GUID, json.RawMessage("{}"), status, ""); err != nil {
		c.log.Printf("error upsering manifest: %v", err)
		return err
	}

	return nil
}

// GetManifestForCrawling retrieves manifest URLs that need to be crawled again. It returns records in batches of limit length,
// continued from the last processed row ID which is the offsetID.
func (c *Core) GetManifestForCrawling(age string, offsetID, maxCrawlErrors, limit int) ([]models.ManifestJob, error) {
	var out []models.ManifestJob
	if err := c.q.GetForCrawling.Select(&out, offsetID, age, maxCrawlErrors, limit); err != nil {
		c.log.Printf("error fetching URLs for crawling: %v", err)
		return nil, err
	}

	for n, u := range out {
		url, err := common.IsURL("url", u.URL, maxURLLen)
		if err != nil {
			c.log.Printf("error parsing url: %s: %v: ", u.URL, err)
			continue
		}

		u.URLobj = url
		out[n] = u
	}

	return out, nil
}

// UpdateManifestStatus updates a manifest's status.
func (c *Core) UpdateManifestStatus(id int, status string) error {
	if _, err := c.q.UpdateManifestStatus.Exec(id, status); err != nil {
		c.log.Printf("error updating manifest status: %d: %v", id, err)
		return err
	}

	return nil
}

// UpdateManifestDate updates a manifest's "updated_at" date.
func (c *Core) UpdateManifestDate(id int) error {
	if _, err := c.q.UpdateManifestDate.Exec(id); err != nil {
		c.log.Printf("error updating manifest date: %d: %v", id, err)
		return err
	}

	return nil
}

// UpdateManifestCrawlError updates a manifest's crawl error count and sets
// it to 'disabled' if it exceeds the given limit.
func (c *Core) UpdateManifestCrawlError(id int, message string, maxErrors int, disableOnErrors bool) (string, error) {
	var status string
	if err := c.q.UpdateCrawlError.Get(&status, id, message, maxErrors, disableOnErrors); err != nil {
		c.log.Printf("error updating manifest crawl error status: %d: %v", id, err)
		return "", err
	}

	return status, nil
}

// DeleteManifest deletes a manifest and all associated data;
func (c *Core) DeleteManifest(id int, guid string) error {
	if _, err := c.q.DeleteManifest.Exec(id, guid); err != nil {
		c.log.Printf("error deleting manifest: %d: %v", id, err)
		return err
	}

	return nil
}

// GetTopTags returns top N tags referenced across projects.
func (c *Core) GetTopTags(limit int) ([]string, error) {
	res := []struct {
		Tag string `db:"tag"`
	}{}
	if err := c.q.GetTopTags.Select(&res, limit); err != nil {
		c.log.Printf("error fetching top tags: %v", err)
		return nil, err
	}

	tags := make([]string, 0, len(res))
	for _, t := range res {
		tags = append(tags, t.Tag)
	}

	return tags, nil
}

// GetRecentProjects retrieves N recently updated projects.
func (c *Core) GetRecentProjects(limit int) (models.Projects, error) {
	exp := strings.ReplaceAll(c.q.QueryProjectsTpl, "%query%", c.q.GetRecentProjects)

	var out models.Projects
	if err := c.db.Select(&out, exp, limit); err != nil {
		c.log.Printf("error fetching recent projects: %v", err)
		return nil, err
	}

	if err := out.Parse(); err != nil {
		c.log.Printf("error parsing projects: %v", err)
		return nil, err
	}

	return out, nil
}

// InsertManifestReport inserts a flagged report with reason for the manifest
func (c *Core) InsertManifestReport(id int, reason string) error {
	if _, err := c.q.InsertReport.Exec(id, reason); err != nil {
		c.log.Printf("error inserting report for manifest: %d: %v", id, err)
		return err
	}

	return nil
}

// GetProjects retrieves paginated projects optionally sorted by certain fields.
func (c *Core) GetProjects(orderBy, order string, offset, limit int) (models.Projects, error) {
	exp := strings.ReplaceAll(c.q.QueryProjectsTpl, "%query%", fmt.Sprintf(c.q.GetProjects, orderBy+" "+order))

	var out models.Projects
	if err := c.db.Select(&out, exp, offset, limit); err != nil {
		c.log.Printf("error fetching projects by start letter: %v", err)
		return nil, err
	}

	if err := out.Parse(); err != nil {
		c.log.Printf("error parsing projects: %v", err)
		return nil, err
	}

	return out, nil
}

// GetProjects retrieves paginated entities optionally sorted by certain fields.
func (c *Core) GetEntities(orderBy, order string, offset, limit int) ([]models.Entity, error) {
	var out []models.Entity

	if err := c.db.Select(&out, fmt.Sprintf(c.q.GetEntities, orderBy+" "+order), offset, limit); err != nil {
		c.log.Printf("error fetching entities by start letter: %v", err)
		return nil, err
	}

	for n, o := range out {
		if err := o.Parse(); err != nil {
			c.log.Printf("error parsing entity: %s: %v", o.ManifestGUID, err)
			return nil, err
		}
		out[n] = o
	}

	return out, nil
}

// SearchEntities searches entities by keywords.
func (c *Core) SearchEntities(query string, offset, limit int) ([]models.Entity, error) {
	var out []models.Entity

	if err := c.q.SearchEntities.Select(&out, query, offset, limit); err != nil {
		c.log.Printf("error searching entities: %v", err)
		return nil, err
	}

	for n, o := range out {
		if err := o.Parse(); err != nil {
			c.log.Printf("error parsing entity: %s: %v", o.ManifestGUID, err)
			return nil, err
		}
		out[n] = o
	}

	return out, nil
}

// SearchProjects searches projects by keywords.
func (c *Core) SearchProjects(query string, tags, licenses []string, orderBy, order string, offset, limit int) (models.Projects, error) {
	exp := strings.ReplaceAll(c.q.QueryProjectsTpl, "%query%", c.q.SearchProjects)

	var out models.Projects
	if err := c.db.Select(&out, exp, query, pq.Array(tags), pq.Array(licenses), offset, limit); err != nil {
		c.log.Printf("error searching projects: %v", err)
		return nil, err
	}

	// Iterate and unmarshal EntityRaw into Entity struct of each project.
	if err := out.Parse(); err != nil {
		c.log.Printf("error parsing projects: %v", err)
		return nil, err
	}

	return out, nil
}

// GetManifestsDump retrieves N manifests raw dumps for export.
func (c *Core) GetManifestsDump(lastID, limit int) ([]models.ManifestExport, error) {
	var out []models.ManifestExport
	if err := c.q.GetManifestsDump.Select(&out, lastID, limit); err != nil {
		c.log.Printf("error exporting manifests: %v", err)
		return nil, err
	}

	return out, nil
}

// getManifests retrieves one or more manifests.
func (c *Core) getManifests(id int, guid string, lastID, limit int, status string) ([]models.ManifestData, error) {
	var (
		out []models.ManifestData
	)

	// Get the manifest.
	if err := c.q.GetManifests.Select(&out, id, guid, lastID, limit, status); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}

		c.log.Printf("error fetching manifest: %d: %v", id, err)
		return nil, err
	}

	for n, o := range out {
		// Funding.
		if err := o.Funding.UnmarshalJSON(o.FundingRaw); err != nil {
			c.log.Printf("error unmarshalling funding: %s: %v", o.GUID, err)
			return nil, err
		}

		// Create a funding map channel for easy lookups.
		o.Channels = make(map[string]v1.Channel)
		for _, c := range o.Funding.Channels {
			o.Channels[c.GUID] = c
		}

		// Fetch entity for this manifest.
		entity, err := c.getEntityByManifest(o.ID)
		if err != nil && err != ErrNotFound {
			c.log.Printf("error fetching entity for manifest %d: %v", o.ID, err)
			return nil, err
		}
		if err != ErrNotFound {
			o.Entity = entity
		}

		// Fetch projects for this manifest.
		projects, err := c.getProjectsByManifest(o.ID)
		if err != nil {
			c.log.Printf("error fetching projects for manifest %d: %v", o.ID, err)
			return nil, err
		}
		o.Projects = projects

		out[n] = o
	}

	return out, nil
}

// getEntityByManifest retrieves entity for a specific manifest.
func (c *Core) getEntityByManifest(manifestID int) (models.Entity, error) {
	var out models.Entity
	if err := c.db.Get(&out, c.q.GetEntityByManifest, manifestID); err != nil {
		if err == sql.ErrNoRows {
			return models.Entity{}, ErrNotFound
		}
		c.log.Printf("error fetching entity by manifest: %v", err)
		return models.Entity{}, err
	}

	if err := out.Parse(); err != nil {
		c.log.Printf("error parsing entity: %s: %v", out.ManifestGUID, err)
		return models.Entity{}, err
	}

	return out, nil
}

// getProjectsByManifest retrieves projects for a specific manifest.
func (c *Core) getProjectsByManifest(manifestID int) (models.Projects, error) {
	exp := strings.ReplaceAll(c.q.QueryProjectsTpl, "%query%", c.q.GetProjectsByManifest)

	var out models.Projects
	if err := c.db.Select(&out, exp, manifestID); err != nil {
		c.log.Printf("error fetching projects by manifest: %v", err)
		return nil, err
	}
	if err := out.Parse(); err != nil {
		c.log.Printf("error parsing projects: %v", err)
		return nil, err
	}

	return out, nil
}

// MakeGUID takes a URL and creates a string "guid" in the form of
// @$host/$uri (last 3 parts, if there are, capped at 40 chars).
func MakeGUID(u *url.URL) string {
	// Match long GitHub blob URLs and return the project URL.
	match := reGithub.FindStringSubmatch(u.String())
	if len(match) > 1 {
		return "@" + strings.TrimPrefix(match[1], "https://")
	}

	// Get the first few parts (or fewer).
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) > 4 {
		parts = parts[:4]
	}

	// Join the parts.
	uri := strings.Join(parts, "/")

	// If the URI is long, cut it to size from the start.
	if len(uri) > 50 {
		uri = uri[:50] + "/**"
	}

	guid := "@" + path.Join(u.Host, uri)
	return guid
}
