package validations

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

var (
	reTag = regexp.MustCompile(`^\p{L}([\p{L}\d-]+)?\p{L}$`)
	reID  = regexp.MustCompile(`^[a-z]([[a-z]\d-]+)?[a-z]$`)
)

// InRange checks whether the given number > min and < max.
func InRange[T int | int32 | int64 | float32 | float64](tag string, num T, min, max T) error {
	if num < min || num > max {
		return fmt.Errorf("%s should be of length %v - %v", tag, min, max)
	}

	return nil
}

func InList[S ~[]E, E comparable](tag string, item E, items S) error {
	if !slices.Contains(items, item) {
		if s, ok := any(items).([]string); ok {
			return fmt.Errorf("%s should be one of: %s", tag, strings.Join(s, ", "))
		}
		return fmt.Errorf("%s has an unknown value", tag)
	}

	return nil
}

func InMap[M ~map[I]I, I comparable](tag, mapName string, item I, mp M) error {
	if _, ok := mp[item]; !ok {
		return fmt.Errorf("%s was not found in the %s", tag, mapName)
	}

	return nil
}

func MaxItems[T ~[]E, E any](tag string, set T, max int) error {
	if len(set) > max {
		return fmt.Errorf("%s can only have max %d elements", tag, max)
	}

	return nil
}

func IsEmail(tag, s string, maxLen int) error {
	if err := InRange[int](tag, len(s), 3, maxLen); err != nil {
		return err
	}

	em, err := mail.ParseAddress(s)
	if err != nil || em.Address != s {
		return fmt.Errorf("%s is not a valid e-mail", tag)
	}

	return nil
}

func IsURL(tag, u string, maxLen int) (*url.URL, error) {
	if err := InRange[int](tag, len(u), 10, maxLen); err != nil {
		return nil, err
	}

	p, err := url.Parse(u)
	if err != nil || (p.Scheme != "https" && p.Scheme != "http") {
		return nil, fmt.Errorf("%s is not a valid URL", tag)
	}

	return p, nil
}

func CheckURL(tag string, mainURL, wellKnownURL string, maxLen int) error {
	// Validate the main URL.
	m, err := IsURL(tag+".url", mainURL, maxLen)
	if err != nil {
		return err
	}

	if wellKnownURL == "" {
		return nil
	}

	// Validate its corresponding well known URL.
	wk, err := IsURL(tag+".well-known", wellKnownURL, maxLen)
	if err != nil {
		return err
	}

	sfx := "/.well-known/funding-json-urls"
	if !strings.HasSuffix(wk.Path, sfx) {
		return fmt.Errorf("%s.wellKnown should end in %s", tag, sfx)
	}

	// well-known URL should match the main URL.
	if wk.Host != m.Host {
		return fmt.Errorf("%s.url and %s.wellKnown hostnames don't match", tag, tag)
	}

	var (
		basePath = strings.TrimRight(m.Path, "/")
		wkPath   = strings.TrimRight(wk.Path, "/")
	)

	// If the base path is the root of the domain, then .well-known should also be.
	if basePath == "" && strings.TrimRight(wkPath, sfx) != "" {
		return fmt.Errorf("%s.url and %s.wellKnown paths don't match", tag, tag)
	}

	// If it's not at the root, then basePath should be a suffix of the well known path.
	// eg:
	// github.com/user ~= github.com/user/project/blob/main/.well-known/funding-json-urls
	// github.com/user/project ~= github.com/user/project/blob/main/.well-known/funding-json-urls
	// github.com/use !~= github.com/user/project/blob/main/.well-known/funding-json-urls
	if !strings.HasPrefix(wkPath, basePath) || wkPath[len(basePath)] != '/' {
		return fmt.Errorf("%s.url and %s.wellKnown paths don't match", tag, tag)
	}

	return nil
}

func IsRepoURL(tag, u string) error {
	if err := InRange[int](tag, len(u), 8, 1024); err != nil {
		return err
	}

	p, err := url.Parse(u)
	if err != nil || (p.Scheme != "https" && p.Scheme != "http" && p.Scheme != "git" && p.Scheme != "svn") {
		return fmt.Errorf("%s is not a valid URL", p)
	}

	return nil
}

func IsTag(tag string, val string, min, max int) error {
	if err := InRange[int](tag, len(val), min, max); err != nil {
		return err
	}

	err := fmt.Errorf("%s should be lowercase alpha-numeric-dashes and length %d - %d", tag, min, max)

	if !reTag.MatchString(val) {
		return err
	}

	if strings.Contains(val, "--") {
		return err
	}

	return nil
}

func IsID(tag string, val string, min, max int) error {
	if err := InRange[int](tag, len(val), min, max); err != nil {
		return err
	}

	err := fmt.Errorf("%s should be lowercase alpha-numeric-dashes and length %d - %d", tag, min, max)

	if !reID.MatchString(val) {
		return err
	}

	if strings.Contains(val, "--") {
		return err
	}

	return nil
}
