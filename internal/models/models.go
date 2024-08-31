package models

import "net/url"

type ManifestURL struct {
	ID     int      `json:"id"`
	URL    string   `json:"url"`
	URLobj *url.URL `json:"-" db:"-"`
}
