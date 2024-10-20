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

var reGithub = regexp.MustCompile(`^(https://github\.com/([^/]+)/([^/]+))/blob/([^/]+)`)

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
	UpsertManifest       *sqlx.Stmt `query:"upsert-manifest"`
	GetManifest          *sqlx.Stmt `query:"get-manifest"`
	GetManifestStatus    *sqlx.Stmt `query:"get-manifest-status"`
	GetForCrawling       *sqlx.Stmt `query:"get-for-crawling"`
	UpdateManifestStatus *sqlx.Stmt `query:"update-manifest-status"`
	UpdateCrawlError     *sqlx.Stmt `query:"update-crawl-error"`
	GetTopTags           *sqlx.Stmt `query:"get-top-tags"`
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

// GetManifest retrieves a manifest.
func (d *Core) GetManifest(id int, guid string) (models.ManifestData, error) {
	var (
		out models.ManifestData
	)

	// Get the manifest. entity{} and projects[{}] are retrieved
	// as JSON fields that need to be manually unmarshalled.
	if err := d.q.GetManifest.Get(&out, id, guid); err != nil {
		if err == sql.ErrNoRows {
			return out, ErrNotFound
		}

		d.log.Printf("error fetching manifest: %d: %v", id, err)
		return out, err
	}

	// Entity.
	if err := out.Entity.UnmarshalJSON(out.EntityRaw); err != nil {
		d.log.Printf("error unmarshalling entity: %d: %v", id, err)
		return out, err
	}

	// Funding.
	if err := out.Funding.UnmarshalJSON(out.FundingRaw); err != nil {
		d.log.Printf("error unmarshalling funding: %d: %v", id, err)
		return out, err
	}

	// Create a funding map channel for easy lookups.
	out.Channels = make(map[string]v1.Channel)
	for _, c := range out.Funding.Channels {
		out.Channels[c.GUID] = c
	}

	// Unmarshal the entity URL. DB names and local names don't match,
	// and it's a nested structure. Sucks.
	{
		var ug models.EntityURL
		if err := ug.UnmarshalJSON(out.EntityRaw); err != nil {
			d.log.Printf("error unmarshalling entity URL: %d: %v", id, err)
			return out, err
		}

		if u, err := common.IsURL("url", ug.WebpageURL, maxURLLen); err != nil {
			d.log.Printf("error parsing entity URL: %d: %s: %v", id, ug.WebpageURL, err)
			return out, err
		} else {
			out.Entity.WebpageURL = v1.URL{URL: ug.WebpageURL, URLobj: u}
		}
	}

	if err := out.Projects.UnmarshalJSON(out.ProjectsRaw); err != nil {
		d.log.Printf("error unmarshalling projects: %d: %v", id, err)
		return out, err
	}

	// Unmarshal project URLs. DB names and local names don't match,
	// and it's a nested structure. This sucks.
	{
		var ug models.ProjectURLs
		if err := ug.UnmarshalJSON(out.ProjectsRaw); err != nil {
			d.log.Printf("error unmarshalling project URLs: %d: %v", id, err)
			return out, err
		}

		for n, p := range ug {
			if u, err := common.IsURL("url", p.WebpageURL, maxURLLen); err != nil {
				d.log.Printf("error parsing entity URL: %d: %s: %v", id, p.WebpageURL, err)
				return out, err
			} else {
				out.Projects[n].WebpageURL = v1.URL{URL: p.WebpageURL, URLobj: u}
			}

			if u, err := common.IsURL("url", p.RepositoryURL, maxURLLen); err != nil {
				d.log.Printf("error parsing entity URL: %d: %s: %v", id, p.RepositoryURL, err)
				return out, err
			} else {
				out.Projects[n].RepositoryURL = v1.URL{URL: p.RepositoryURL, URLobj: u}
			}
		}
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
func (d *Core) UpsertManifest(m models.ManifestData) error {
	body, err := m.Manifest.MarshalJSON()
	if err != nil {
		d.log.Printf("error marshalling manifest: %s: %v", m.URL, err)
		return err
	}

	if _, err := d.q.UpsertManifest.Exec(json.RawMessage(body), m.Manifest.URL.URL, m.GUID, json.RawMessage("{}"), ManifestStatusPending, ""); err != nil {
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
