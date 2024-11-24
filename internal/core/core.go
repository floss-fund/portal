package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/floss-fund/go-funding-json/common"
	v1 "github.com/floss-fund/go-funding-json/schemas/v1"
	"github.com/floss-fund/portal/internal/models"
	"github.com/jmoiron/sqlx"
)

const maxURISize = 40
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
	UpsertManifest                *sqlx.Stmt `query:"upsert-manifest"`
	GetManifests                  *sqlx.Stmt `query:"get-manifests"`
	GetManifestStatus             *sqlx.Stmt `query:"get-manifest-status"`
	GetForCrawling                *sqlx.Stmt `query:"get-for-crawling"`
	UpdateManifestStatus          *sqlx.Stmt `query:"update-manifest-status"`
	UpdateManifestDate            *sqlx.Stmt `query:"update-manifest-date"`
	UpdateCrawlError              *sqlx.Stmt `query:"update-crawl-error"`
	DeleteManifest                *sqlx.Stmt `query:"delete-manifest"`
	GetTopTags                    *sqlx.Stmt `query:"get-top-tags"`
	InsertReport                  *sqlx.Stmt `query:"insert-report"`
	GetRecentProjects             *sqlx.Stmt `query:"get-recent-projects"`
	GetProjectsAlphabetically     *sqlx.Stmt `query:"get-projects-alphabetically"`
	GetProjectCountAlphabetically *sqlx.Stmt `query:"get-project-count-alphabetically"`
	GetEntitiesAlphabetically     *sqlx.Stmt `query:"get-entities-alphabetically"`
	GetEntityCountAlphabetically  *sqlx.Stmt `query:"get-entity-count-alphabetically"`
}

type Core struct {
	q   *Queries
	opt Opt
	hc  *http.Client
	log *log.Logger
}

var (
	ErrNotFound = errors.New("not found")
)

func New(q *Queries, o Opt, lo *log.Logger) *Core {
	return &Core{
		q:   q,
		log: lo,
	}
}

// GetManifest retrieves a particular manifest.
func (d *Core) GetManifest(id int, guid string, status string) (models.ManifestData, error) {
	out, err := d.getManifests(id, guid, 0, 1, status)
	if err != nil || len(out) == 0 {
		return models.ManifestData{}, ErrNotFound
	}

	return out[0], nil
}

// GetManifests retrieves N manifests.
func (d *Core) GetManifests(lastID, limit int, status string) ([]models.ManifestData, error) {
	out, err := d.getManifests(0, "", lastID, limit, status)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// GetManifestStatus checks whether a given manifest URL exists in the databse.
// If one exists, its status is returned.
func (d *Core) GetManifestStatus(url string) (string, error) {
	var status string
	if err := d.q.GetManifestStatus.Get(&status, url); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}

		d.log.Printf("error checking manifest status: %s: %v", url, err)
		return "", err
	}

	return status, nil
}

// UpsertManifest upserts an entry into the database.
func (d *Core) UpsertManifest(m models.ManifestData, status string) error {
	body, err := m.Manifest.MarshalJSON()
	if err != nil {
		d.log.Printf("error marshalling manifest: %s: %v", m.URL, err)
		return err
	}

	if _, err := d.q.UpsertManifest.Exec(json.RawMessage(body), m.Manifest.URL.URL, m.GUID, json.RawMessage("{}"), status, ""); err != nil {
		d.log.Printf("error upsering manifest: %v", err)
		return err
	}

	return nil
}

// GetManifestForCrawling retrieves manifest URLs that need to be crawled again. It returns records in batches of limit length,
// continued from the last processed row ID which is the offsetID.
func (d *Core) GetManifestForCrawling(age string, offsetID, limit int) ([]models.ManifestJob, error) {
	var out []models.ManifestJob
	if err := d.q.GetForCrawling.Select(&out, offsetID, age, limit); err != nil {
		d.log.Printf("error fetching URLs for crawling: %v", err)
		return nil, err
	}

	for n, u := range out {
		url, err := common.IsURL("url", u.URL, maxURLLen)
		if err != nil {
			d.log.Printf("error parsing url: %s: %v: ", u.URL, err)
			continue
		}

		u.URLobj = url
		out[n] = u
	}

	return out, nil
}

// UpdateManifestStatus updates a manifest's status.
func (d *Core) UpdateManifestStatus(id int, status string) error {
	if _, err := d.q.UpdateManifestStatus.Exec(id, status); err != nil {
		d.log.Printf("error updating manifest status: %d: %v", id, err)
		return err
	}

	return nil
}

// UpdateManifestDate updates a manifest's "updated_at" date.
func (d *Core) UpdateManifestDate(id int) error {
	if _, err := d.q.UpdateManifestDate.Exec(id); err != nil {
		d.log.Printf("error updating manifest date: %d: %v", id, err)
		return err
	}

	return nil
}

// UpdateManifestCrawlError updates a manifest's crawl error count and sets
// it to 'disabled' if it exceeds the given limit.
func (d *Core) UpdateManifestCrawlError(id int, message string, maxErrors int) (string, error) {
	var status string
	if err := d.q.UpdateCrawlError.Get(&status, id, message, maxErrors); err != nil {
		d.log.Printf("error updating manifest crawl error status: %d: %v", id, err)
		return "", err
	}

	return status, nil
}

// DeleteManifest deletes a manifest and all associated data;
func (d *Core) DeleteManifest(id int, guid string) error {
	if _, err := d.q.DeleteManifest.Exec(id, guid); err != nil {
		d.log.Printf("error deleting manifest: %d: %v", id, err)
		return err
	}

	return nil
}

// GetTopTags returns top N tags referenced across projects.
func (d *Core) GetTopTags(limit int) ([]string, error) {
	res := []struct {
		Tag string `db:"tag"`
	}{}
	if err := d.q.GetTopTags.Select(&res, limit); err != nil {
		d.log.Printf("error fetching top tags: %v", err)
		return nil, err
	}

	tags := make([]string, 0, len(res))
	for _, t := range res {
		tags = append(tags, t.Tag)
	}

	return tags, nil
}

// GetRecentProjects retrieves N recently updated projects.
func (d *Core) GetRecentProjects(limit int) ([]models.Project, error) {
	var projects []models.Project
	if err := d.q.GetRecentProjects.Select(&projects, limit); err != nil {
		d.log.Printf("error fetching recent projects: %v", err)
		return nil, err
	}

	return projects, nil
}

// InsertManifestReport inserts a flagged report with reason for the manifest
func (d *Core) InsertManifestReport(id int, reason string) error {
	if _, err := d.q.InsertReport.Exec(id, reason); err != nil {
		d.log.Printf("error inserting report for manifest: %d: %v", id, err)
		return err
	}

	return nil
}

// GetProjectsAlphabetically retrieves projects by the first letter of their name
// sorted alphabetically.
func (d *Core) GetProjectsAlphabetically(q string, offset, limit int) ([]models.Project, error) {
	var out []models.Project

	if err := d.q.GetProjectsAlphabetically.Select(&out, q, offset, limit); err != nil {
		d.log.Printf("error fetching projects by start letter: %v", err)
		return nil, err
	}

	return out, nil
}

// GetProjectCountAlphabetically retrieves projects by the first letter of their name
// sorted alphabetically.
func (d *Core) GetProjectCountAlphabetically(q string) (int, error) {
	var num int
	if err := d.q.GetProjectCountAlphabetically.Get(&num, q); err != nil {
		d.log.Printf("error fetching project count by start letter: %v", err)
		return num, err
	}

	return num, nil
}

// GetEntitiesAlphabetically retrieves entities by the first letter of their name
// sorted alphabetically.
func (d *Core) GetEntitiesAlphabetically(q string, offset, limit int) ([]models.Entity, error) {
	var out []models.Entity

	if err := d.q.GetEntitiesAlphabetically.Select(&out, q, offset, limit); err != nil {
		d.log.Printf("error fetching entities by start letter: %v", err)
		return nil, err
	}

	return out, nil
}

// GetEntityCountAlphabetically retrieves projects by the first letter of their name
// sorted alphabetically.
func (d *Core) GetEntityCountAlphabetically(q string) (int, error) {
	var num int
	if err := d.q.GetEntityCountAlphabetically.Get(&num, q); err != nil {
		d.log.Printf("error fetching entity count by start letter: %v", err)
		return num, err
	}

	return num, nil
}

// getManifests retrieves one or more manifests.
func (d *Core) getManifests(id int, guid string, lastID, limit int, status string) ([]models.ManifestData, error) {
	var (
		out []models.ManifestData
	)

	// Get the manifest. entity{} and projects[{}] are retrieved
	// as JSON fields that need to be manually unmarshalled.
	if err := d.q.GetManifests.Select(&out, id, guid, lastID, limit, status); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}

		d.log.Printf("error fetching manifest: %d: %v", id, err)
		return nil, err
	}

	for n, o := range out {
		o := o

		// Entity.
		if err := o.Entity.UnmarshalJSON(o.EntityRaw); err != nil {
			d.log.Printf("error unmarshalling entity: %d: %v", id, err)
			return nil, err
		}

		// Funding.
		if err := o.Funding.UnmarshalJSON(o.FundingRaw); err != nil {
			d.log.Printf("error unmarshalling funding: %d: %v", id, err)
			return nil, err
		}

		// Create a funding map channel for easy lookups.
		o.Channels = make(map[string]v1.Channel)
		for _, c := range o.Funding.Channels {
			o.Channels[c.GUID] = c
		}

		// Unmarshal the entity URL. DB names and local names don't match,
		// and it's a nested structure. Sucks.
		{
			var ug models.EntityURL
			if err := ug.UnmarshalJSON(o.EntityRaw); err != nil {
				d.log.Printf("error unmarshalling entity URL: %d: %v", id, err)
				return nil, err
			}

			if u, err := common.IsURL("url", ug.WebpageURL, maxURLLen); err != nil {
				d.log.Printf("error parsing entity URL: %d: %s: %v", id, ug.WebpageURL, err)
				return nil, err
			} else {
				o.Entity.WebpageURL = v1.URL{URL: ug.WebpageURL, URLobj: u}
			}
		}

		if err := o.Projects.UnmarshalJSON(o.ProjectsRaw); err != nil {
			d.log.Printf("error unmarshalling projects: %d: %v", id, err)
			return nil, err
		}

		// Unmarshal project URLs. DB names and local names don't match,
		// and it's a nested structure. This sucks.
		{
			var ug models.ProjectURLs
			if err := ug.UnmarshalJSON(o.ProjectsRaw); err != nil {
				d.log.Printf("error unmarshalling project URLs: %d: %v", id, err)
				return nil, err
			}

			for n, p := range ug {
				if u, err := common.IsURL("url", p.WebpageURL, maxURLLen); err != nil {
					d.log.Printf("error parsing entity URL: %d: %s: %v", id, p.WebpageURL, err)
					return nil, err
				} else {
					o.Projects[n].WebpageURL = v1.URL{URL: p.WebpageURL, URLobj: u}
				}

				if u, err := common.IsURL("url", p.RepositoryURL, maxURLLen); err != nil {
					d.log.Printf("error parsing entity URL: %d: %s: %v", id, p.RepositoryURL, err)
					return nil, err
				} else {
					o.Projects[n].RepositoryURL = v1.URL{URL: p.RepositoryURL, URLobj: u}
				}
			}
		}

		out[n] = o
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
