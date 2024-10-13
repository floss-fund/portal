package search

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/floss-fund/go-funding-json/common"
	"github.com/knadh/koanf/maps"
)

const (
	searchURI           = "/collections/%s/documents/search"
	docsURI             = "/collections/%s/documents"
	collectionsURI      = "/collections"
	deleteCollectionURI = "/collections/%s"
	deleteDocURI        = "/collections/%s/documents/%s"
	collEntities        = "entities"
	collProjects        = "projects"
)

type Opt struct {
	// Typesense params.
	RootURL    string
	APIKey     string
	Collection string
	Groups     []string

	PerPage int

	HTTP common.HTTPOpt
}

type Search struct {
	opt Opt

	perPage string
	groups  map[string]bool

	hc  *common.HTTPClient
	log *log.Logger
}

var (
	//go:embed schema.json
	efs embed.FS
)

// New returns a new instance of Omnisearch.
func New(o Opt, l *log.Logger) *Search {
	if o.PerPage == 0 {
		o.PerPage = 50
	}

	return &Search{
		opt:     o,
		perPage: strconv.Itoa(o.PerPage),
		hc:      common.NewHTTPClient(o.HTTP, l),
		groups:  maps.StringSliceToLookupMap(o.Groups),
		log:     l,
	}
}

// SearchEntities searches the entities collection.
func (o *Search) SearchEntities(q EntityQuery) (Entities, int, error) {
	p := url.Values{}
	p.Set("q", q.Query)
	p.Set("query_by", "name")

	if q.Type != "" {
		p.Set("filter_by", "type:="+q.Type)
	}
	if q.Role != "" {
		p.Set("filter_by", "role:="+q.Type)
	}

	p.Set("per_page", o.perPage)

	// Search.
	b, _, err := o.do(http.MethodGet, fmt.Sprintf(searchURI, collEntities), []byte(p.Encode()))
	if err != nil {
		return nil, 0, err
	}

	var res EntitiesResp
	if err := res.UnmarshalJSON(b); err != nil {
		return nil, 0, err
	}

	// Iterate through the raw results and replace the Title and Description
	// fields with their <mark> highlighted equivalents, if any.
	out := make(Entities, 0, len(res.Hits))
	for _, h := range res.Hits {
		d := h.Entity

		out = append(out, d)
	}

	return out, res.Found, nil
}

// InsertEntity adds a Entity to the search index.
func (s *Search) InsertEntity(e Entity) error {
	// Marshal to JSON.
	b, err := e.MarshalJSON()
	if err != nil {
		return err
	}

	if _, _, err := s.do(http.MethodPost, fmt.Sprintf(docsURI, collEntities)+"?action=upsert", b); err != nil {
		return err
	}

	return nil
}

// DeleteEntity delete an Entity from the search index.
func (s *Search) DeleteEntity(id string) error {
	if _, _, err := s.do(http.MethodDelete, fmt.Sprintf(deleteDocURI, collEntities, id), nil); err != nil {
		return err
	}

	return nil
}

// SearchProjects searches the entities collection.
func (o *Search) SearchProjects(q ProjectQuery) (Projects, int, error) {
	p := url.Values{}
	p.Set("q", q.Query)

	if q.Field == "tags" {
		p.Set("query_by", "tags")
	} else {
		p.Set("query_by", "name,tags,description")
	}

	if len(q.Licenses) > 0 {
		p.Set("filter_by", "licenses="+strings.Join(q.Licenses, ","))
	}

	p.Set("per_page", o.perPage)

	// Search.
	b, _, err := o.do(http.MethodGet, fmt.Sprintf(searchURI, collProjects), []byte(p.Encode()))
	if err != nil {
		return nil, 0, err
	}

	var res ProjectsResp
	if err := res.UnmarshalJSON(b); err != nil {
		return nil, 0, err
	}

	// Iterate through the raw results and replace the Title and Description
	// fields with their <mark> highlighted equivalents, if any.
	out := make(Projects, 0, len(res.Hits))
	for _, h := range res.Hits {
		d := h.Project

		out = append(out, d)
	}

	return out, res.Found, nil
}

// GetRecentEntities retrieves N recently updated entities.
func (o *Search) GetRecentEntities(limit int) (Entities, error) {
	p := url.Values{}
	p.Set("q", "*")
	p.Set("sort_by", "updated_at:desc")
	p.Set("limit", fmt.Sprintf("%d", limit))

	// Search.
	b, _, err := o.do(http.MethodGet, fmt.Sprintf(searchURI, collEntities), []byte(p.Encode()))
	if err != nil {
		return nil, err
	}

	var res EntitiesResp
	if err := res.UnmarshalJSON(b); err != nil {
		return nil, err
	}

	// Iterate through the raw results and replace the Title and Description
	// fields with their <mark> highlighted equivalents, if any.
	out := make(Entities, 0, len(res.Hits))
	for _, h := range res.Hits {
		d := h.Entity

		out = append(out, d)
	}

	return out, nil
}

// GetRecentProjects retrieves N recently updated entities.
func (o *Search) GetRecentProjects(limit int) (Projects, error) {
	p := url.Values{}
	p.Set("q", "*")
	p.Set("sort_by", "updated_at:desc")
	p.Set("limit", fmt.Sprintf("%d", limit))

	// Search.
	b, _, err := o.do(http.MethodGet, fmt.Sprintf(searchURI, collProjects), []byte(p.Encode()))
	if err != nil {
		return nil, err
	}

	var res ProjectsResp
	if err := res.UnmarshalJSON(b); err != nil {
		return nil, err
	}

	// Iterate through the raw results and replace the Title and Description
	// fields with their <mark> highlighted equivalents, if any.
	out := make(Projects, 0, len(res.Hits))
	for _, h := range res.Hits {
		d := h.Project

		out = append(out, d)
	}

	return out, nil
}

// InsertProject adds a project to the search index.
func (s *Search) InsertProject(p Project) error {
	// Marshal to JSON.
	b, err := p.MarshalJSON()
	if err != nil {
		return err
	}

	if _, _, err := s.do(http.MethodPost, fmt.Sprintf(docsURI, collProjects)+"?action=upsert", b); err != nil {
		s.log.Printf("error inserting project: %s: %v", p.ID, err)
		return err
	}

	return nil
}

// DeleteProject deletes a Project from the search index.
func (s *Search) DeleteProject(id string) error {
	if _, _, err := s.do(http.MethodDelete, fmt.Sprintf(deleteDocURI, collProjects, id), nil); err != nil {
		s.log.Printf("error deleting project ID: %s: %v", id, err)
		return err
	}

	return nil
}

// Delete deletes the entity and projects associted with the given manifest ID.
func (s *Search) Delete(manifestID int) error {
	p := url.Values{}
	p.Set("filter_by", "manifest_id:="+fmt.Sprintf("%d", manifestID))

	if _, _, err := s.do(http.MethodDelete, fmt.Sprintf(docsURI, collProjects), []byte(p.Encode())); err != nil {
		s.log.Printf("error deleting projects by manifest ID: %v", err)
		return err
	}
	if _, _, err := s.do(http.MethodDelete, fmt.Sprintf(docsURI, collEntities), []byte(p.Encode())); err != nil {
		s.log.Printf("error deleting entities entries by manifest ID: %v", err)
		return err
	}

	return nil
}

// InitSchema deletes and recreates the empty collection afresh.
// If `typ` is given, only entries with type=$typ are deleted.
func (o *Search) InitSchema() error {
	colls, err := o.readSchema()
	if err != nil {
		return err
	}

	for name, b := range colls {
		// Delete the collection if it already exists.
		o.do(http.MethodDelete, fmt.Sprintf(deleteCollectionURI, name), nil)

		// Create the collection.
		if body, _, err := o.do(http.MethodPost, collectionsURI, b); err != nil {
			if len(body) > 0 {
				o.log.Println(string(body))
			}
			return err
		}
	}

	return nil
}

// ImportRawData imports raw JSON document data into the Typesense collection.
func (o *Search) ImportRawData(b []byte) error {
	// Create the collection.
	_, _, err := o.do(http.MethodPost, fmt.Sprintf("/collections/%s/documents/import?action=upsert", o.opt.Collection), b)
	if err != nil {
		return err
	}

	return nil
}

func (o *Search) do(method, uri string, body []byte) ([]byte, int, error) {
	headers := http.Header{}
	headers.Add("X-TYPESENSE-API-KEY", o.opt.APIKey)

	body, _, _, statusCode, err := o.hc.DoReq(method, o.opt.RootURL+uri, body, headers)
	if err != nil {
		return body, statusCode, err
	}

	// 200 OK.
	if statusCode < 300 {
		return body, statusCode, nil
	}

	// Non-200 error. Extract the message.
	out := struct {
		Message string `json:"message"`
	}{}
	if err := json.Unmarshal(body, &out); err != nil {
		return body, statusCode, err
	}

	return body, statusCode, errors.New(out.Message)
}

// readSchema reads the JSON schema used for initializing the collection.
func (o *Search) readSchema() (map[string][]byte, error) {
	// Read the raw JSON schema.
	schema, err := efs.ReadFile("schema.json")
	if err != nil {
		return nil, err
	}

	var data []map[string]interface{}
	if err := json.Unmarshal(schema, &data); err != nil {
		return nil, err
	}

	out := make(map[string][]byte)
	for _, d := range data {
		b, err := json.Marshal(d)
		if err != nil {
			return nil, err
		}

		name, ok := d["name"]
		if !ok {
			return nil, errors.New("`name` not found in collection schema")
		}

		out[name.(string)] = b
	}

	return out, nil
}
