package search

//easyjson:json
type Entity struct {
	ID           string `json:"id"`
	ManifestID   int    `json:"manifest_id"`
	ManifestGUID string `json:"manifest_guid"`
	Type         string `json:"type"`
	Role         string `json:"role"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	WebpageURL   string `json:"webpage_url"`
	NumProjects  int    `json:"num_projects"`
	UpdatedAt    int64  `json:"updated_at"`
}

//easyjson:json
type Entities []Entity

//easyjson:json
type EntityQuery struct {
	Query string `json:"q"`
	Field string `json:"field"`
	Entity
}

//easyjson:json
type Project struct {
	ID                string `json:"id"`
	ManifestID        int    `json:"manifest_id"`
	ManifestGUID      string `json:"manifest_guid"`
	EntityName        string `json:"entity_name"`
	EntityType        string `json:"entity_type"`
	EntityNumProjects int    `json:"entity_num_projects"`

	Name          string   `json:"name"`
	Description   string   `json:"description"`
	WebpageURL    string   `json:"webpage_url"`
	RepositoryURL string   `json:"repository_url"`
	Licenses      []string `json:"licenses"`
	Tags          []string `json:"tags"`
	UpdatedAt     int64    `json:"updated_at"`
}

//easyjson:json
type Projects []Project

//easyjson:json
type ProjectQuery struct {
	Query string `json:"q"`
	Field string `json:"field"`
	Project
}

//easyjson:json
type EntitiesResp struct {
	Found int `json:"found"`
	Hits  []struct {
		Entity Entity `json:"document"`
	} `json:"hits"`
}

//easyjson:json
type ProjectsResp struct {
	Found int `json:"found"`
	Hits  []struct {
		Project Project `json:"document"`
	} `json:"hits"`
}
