package search

//easyjson:json
type Entity struct {
	ManifestID int `json:"manifest_id,omitempty"`

	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Role string `json:"role"`
}

//easyjson:json
type Entities []Entity

//easyjson:json
type EntityQuery struct {
	Query string `json:"q"`
	Entity
}

//easyjson:json
type Project struct {
	ManifestID int `json:"manifest_id,omitempty"`

	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Licenses    []string `json:"licenses"`
	Tags        []string `json:"tags"`
}

//easyjson:json
type Projects []Project

//easyjson:json
type ProjectQuery struct {
	Query string `json:"q"`
	Project
}

//easyjson:json
type EntitiesResp struct {
	GroupedHits []struct {
		Hits []struct {
			Entity Entity `json:"document"`
		} `json:"hits"`
	} `json:"grouped_hits"`
}

//easyjson:json
type ProjectsResp struct {
	GroupedHits []struct {
		Hits []struct {
			Project Project `json:"document"`
		} `json:"hits"`
	} `json:"grouped_hits"`
}
