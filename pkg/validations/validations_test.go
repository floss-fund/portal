package validations

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsURL(t *testing.T) {
	bad := []string{
		"blah",
		"http",
		"https",
		"https:/blah",
		"http://",
		"https://",
		"https://..........",
		"https://looooooooong.net",
	}

	for _, b := range bad {
		_, err := IsURL("tag", b, 15)
		assert.Error(t, err)
	}
}

func TestWellKnownURL(t *testing.T) {
	type URL struct {
		Manifest  *url.URL
		URL       string
		WellKnown string
	}

	var (
		wellKnowPath = "/.well-known/here"
		m1, _        = url.Parse("https://site.com/funding.json")
		m2, _        = url.Parse("https://site.com/user/funding.json")
		m2, _        = url.Parse("https://site.com/user/project/tree/main/funding.json")
	)

	bad := []URL{}

	for _, b := range bad {
		_, err := IsURL("tag", b, 15)
		assert.Error(t, err)
	}
}
