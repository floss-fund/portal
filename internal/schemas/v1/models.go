package v1

import (
	"encoding/json"
)

var (
	EntityTypes     = []string{"individual", "group", "organisation", "other"}
	EntityRoles     = []string{"owner", "steward", "maintainer", "contributor", "other"}
	ChannelTypes    = []string{"bank", "gateway", "cheque", "cash", "other"}
	PlanFrequencies = []string{"one-time", "weekly", "biweekly", "fornightly", "monthly", "bimonthly", "yearly"}
	PlanStatuses    = []string{"active", "inactive"}
)

//easyjson:json
type URL struct {
	URL       string `json:"url"`
	WellKnown string `json:"wellKnown"`
}

// Entity represents an entity in charge of a project: individual, organisation etc.
//
//easyjson:json
type Entity struct {
	Type       string `json:"type"`
	Role       string `json:"role"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Telephone  string `json:"telephone"`
	WebpageURL URL    `json:"webpageUrl"`
}

// Project represents a FOSS project.
//
//easyjson:json
type Project struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	WebpageURL    URL      `json:"webpageUrl"`
	RepositoryUrl URL      `json:"repositoryUrl"`
	License       string   `json:"license"`
	Frameworks    []string `json:"frameworks"`
	Tags          []string `json:"tags"`
}

// Channel is a loose representation of a payment channel. eg: bank, cash, or a processor like PayPal.
//
//easyjson:json
type Channel struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Address     string `json:"address"`
	Description string `json:"description"`
}

// Plan represents a payment plan / ask for the project.
//
//easyjson:json
type Plan struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Amount      float64  `json:"amount"`
	Currency    string   `json:"currency"`
	Frequency   string   `json:"frequency"`
	Channels    []string `json:"channels"`
}

// History represents a very course, high level income/expense statement.
//
//easyjson:json
type History struct {
	Year        int     `json:"year"`
	Income      float64 `json:"income"`
	Expenses    float64 `json:"expenses"`
	Description string  `json:"description"`
}

//easyjson:json
type Manifest struct {
	ID       string          `json:"id"`
	UUID     string          `json:"uuid"`
	Version  string          `json:"version"`
	URL      string          `json:"url"`
	Body     json.RawMessage `json:"body"`
	Entity   Entity          `json:"entity"`
	Projects []Project       `json:"projects"`

	Funding struct {
		Channels []Channel `json:"channels"`
		Plans    []Plan    `json:"plans"`
		History  []History `json:"history"`
	} `json:"funding"`
}
