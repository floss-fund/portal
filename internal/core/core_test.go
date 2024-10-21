package core

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMakeGUID(t *testing.T) {

	f := func(u1, u2 string) {
		u, err := url.Parse(u1)
		if err != nil {
			t.Fatalf("failed to parse URL: %v", err)
		}

		assert.Equal(t, u2, MakeGUID(u))
	}

	f("https://github.com/user/repo/blob/main/file.txt", "@github.com/user/repo")
	f("https://github.com/user/project/raw/main/funding.json", "@github.com/user/project")
	f("https://example.com/path/to/resource", "@example.com/path/to/resource")
	f("https://example.com/very/long/path/to/resource/that/exceeds/limit/", "@example.com/very/long/path/to")
	f("https://example.com/very/long/here/long-path-to-resource-that-exceeds-limit-b-a-lot-long-path-to-resource-that-exceeds-limit-b-a-lot/", "@example.com/very/long/here/long-path-to-resource-that-exceeds-/**")
	f("https://example.com/", "@example.com")
	f("https://example.com/single", "@example.com/single")
	f("https://sub.domain.example.com/project", "@sub.domain.example.com/project")
}
