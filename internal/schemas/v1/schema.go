package v1

import (
	"fmt"
	"net/url"
	"strings"

	"floss.fund/portal/internal/validations"
)

// Major version of this schema.
const version = "v1.0.0"

// Schema represents the schema+parser+validator for a particular version.
type Schema struct {
	exactVersion string
	opt          *Opt
}

type Opt struct {
	// Map of SPDX ID: License name.
	Licenses map[string]string

	// Map of programming language names.
	ProgrammingLanguages map[string]string

	// Map of curency code and names.
	Currencies map[string]string

	WellKnownURI string
}

// New returns a new instance of Schema.
func New(exactVersion string, opt *Opt) *Schema {
	return &Schema{
		exactVersion: exactVersion,
		opt:          opt,
	}
}

// Validate validates a given manifest against its schema.
func (s *Schema) Validate(m Manifest) (Manifest, error) {
	if m.Version != s.exactVersion {
		return m, fmt.Errorf("version should be %s", s.exactVersion)
	}

	mURL, err := validations.IsURL("manifest URL", m.URL, 1024)
	if err != nil {
		return m, err
	}

	// Entity.
	if m.Entity, err = s.ValidateEntity(m.Entity, mURL); err != nil {
		return m, err
	}

	// Projects.
	for n, o := range m.Projects {
		if o, err = s.ValidateProject(o, n, mURL); err != nil {
			return m, err
		}
		m.Projects[n] = o
	}

	// Funding channels.
	chIDs := make(map[string]struct{})
	for n, o := range m.Funding.Channels {
		if o, err = s.ValidateChannel(o, n); err != nil {
			return m, err
		}

		m.Funding.Channels[n] = o
		chIDs[o.ID] = struct{}{}
	}

	// Funding plans.
	if err := validations.InRange[int]("plans", len(m.Funding.Plans), 1, 30); err != nil {
		return m, err
	}
	for n, o := range m.Funding.Plans {
		if o, err = s.ValidatePlan(o, n, chIDs); err != nil {
			return m, err
		}
		m.Funding.Plans[n] = o
	}

	// History.
	if err := validations.InRange[int]("history", len(m.Funding.Plans), 0, 50); err != nil {
		return m, err
	}
	for n, o := range m.Funding.History {
		if o, err = s.ValidateHistory(o, n); err != nil {
			return m, err
		}
		m.Funding.History[n] = o
	}

	return m, nil
}

func (s *Schema) ValidateEntity(o Entity, manifest *url.URL) (Entity, error) {
	if err := validations.InList("entity.type", o.Type, EntityTypes); err != nil {
		return o, err
	}

	if err := validations.InList("entity.role", o.Role, EntityRoles); err != nil {
		return o, err
	}

	if err := validations.InRange[int]("entity.name", len(o.Name), 2, 128); err != nil {
		return o, err
	}

	if err := validations.IsEmail("entity.email", o.Email, 128); err != nil {
		return o, err
	}

	if err := validations.InRange[int]("entity.telephone", len(o.Telephone), 0, 24); err != nil {
		return o, err
	}

	if tgURL, wkURL, err := validations.WellKnownURL("entity.webpageUrl", manifest, o.WebpageURL.URL, o.WebpageURL.WellKnown, s.opt.WellKnownURI, 1024); err != nil {
		return o, err
	} else {
		o.WebpageURL.URLobj = tgURL
		o.WebpageURL.WellKnownObj = wkURL
	}

	return o, nil
}

func (s *Schema) ValidateProject(o Project, n int, manifest *url.URL) (Project, error) {
	if err := validations.InRange[int](fmt.Sprintf("projects[%d].name", n), len(o.Name), 1, 256); err != nil {
		return o, err
	}

	if err := validations.InRange[int](fmt.Sprintf("projects[%d].description", n), len(o.Description), 5, 1024); err != nil {
		return o, err
	}

	if tgURL, wkURL, err := validations.WellKnownURL(fmt.Sprintf("projects[%d].webpageUrl", n), manifest, o.WebpageURL.URL, o.WebpageURL.WellKnown, s.opt.WellKnownURI, 1024); err != nil {
		return o, err
	} else {
		o.WebpageURL.URLobj = tgURL
		o.WebpageURL.WellKnownObj = wkURL
	}

	if tgURL, wkURL, err := validations.WellKnownURL(fmt.Sprintf("projects[%d].repositoryUrl", n), manifest, o.RepositoryUrl.URL, o.RepositoryUrl.WellKnown, s.opt.WellKnownURI, 1024); err != nil {
		return o, err
	} else {
		o.RepositoryUrl.URLobj = tgURL
		o.RepositoryUrl.WellKnownObj = wkURL
	}

	// License.
	licenseTag := fmt.Sprintf("projects[%d].license", n)
	if err := validations.InRange[int](licenseTag, len(o.License), 2, 64); err != nil {
		return o, err
	}
	if strings.HasPrefix(o.License, "spdx:") {
		if err := validations.InMap(licenseTag, "spdx license list", strings.TrimPrefix(o.License, "spdx:"), s.opt.Licenses); err != nil {
			return o, err
		}
	}

	// Frameworks.
	if err := validations.InRange[int](fmt.Sprintf("projects[%d].frameworks", n), len(o.Frameworks), 0, 5); err != nil {
		return o, err
	}
	for i, f := range o.Frameworks {
		fTag := fmt.Sprintf("projects[%d].frameworks[%d]", n, i)
		if err := validations.InRange[int](fTag, len(f), 2, 64); err != nil {
			return o, err
		}

		if strings.HasPrefix(f, "lang:") {
			if err := validations.InMap(fTag, "default programming language list", strings.TrimPrefix(f, "lang:"), s.opt.ProgrammingLanguages); err != nil {
				return o, err
			}
		}
	}

	// Tags.
	if err := validations.InRange[int](fmt.Sprintf("projects[%d].tags", n), len(o.Tags), 1, 10); err != nil {
		return o, err
	}
	for i, t := range o.Tags {
		if err := validations.IsTag(fmt.Sprintf("projects[%d].tags[%d]", n, i), t, 2, 32); err != nil {
			return o, err
		}
	}

	return o, nil
}

func (s *Schema) ValidateChannel(o Channel, n int) (Channel, error) {
	if err := validations.IsID(fmt.Sprintf("channels[%d].id", n), o.ID, 3, 32); err != nil {
		return o, err
	}

	if err := validations.InList(fmt.Sprintf("channels[%d].type", n), o.Type, ChannelTypes); err != nil {
		return o, err
	}

	if err := validations.InRange[int](fmt.Sprintf("channels[%d].address", n), len(o.Address), 0, 128); err != nil {
		return o, err
	}

	if err := validations.InRange[int](fmt.Sprintf("channels[%d].description", n), len(o.Description), 0, 1024); err != nil {
		return o, err
	}

	return o, nil
}

func (s *Schema) ValidatePlan(o Plan, n int, channelIDs map[string]struct{}) (Plan, error) {
	if err := validations.IsID(fmt.Sprintf("plans[%d].id", n), o.ID, 3, 32); err != nil {
		return o, err
	}

	if err := validations.InList(fmt.Sprintf("plans[%d].status", n), o.Status, PlanStatuses); err != nil {
		return o, err
	}

	if err := validations.InRange[int](fmt.Sprintf("plans[%d].name", n), len(o.Name), 3, 128); err != nil {
		return o, err
	}

	if err := validations.InRange[int](fmt.Sprintf("plans[%d].description", n), len(o.Description), 0, 1024); err != nil {
		return o, err
	}

	if err := validations.InRange[float64](fmt.Sprintf("plans[%d].amount", n), o.Amount, 0, 1000000000); err != nil {
		return o, err
	}

	if err := validations.InMap(fmt.Sprintf("plans[%d].currency", n), "currencies list", o.Currency, s.opt.Currencies); err != nil {
		return o, err
	}

	if err := validations.InList(fmt.Sprintf("plans[%d].frequency", n), o.Frequency, PlanFrequencies); err != nil {
		return o, err
	}

	for _, ch := range o.Channels {
		if _, ok := channelIDs[ch]; !ok {
			return o, fmt.Errorf("unknown channel id in plans[%d].frequency", n)
		}
	}

	return o, nil
}

func (s *Schema) ValidateHistory(o HistoryItem, n int) (HistoryItem, error) {
	if err := validations.InRange[int](fmt.Sprintf("history[%d].year", n), o.Year, 1970, 2075); err != nil {
		return o, err
	}

	if err := validations.InRange[float64](fmt.Sprintf("plans[%d].income", n), o.Income, 0, 1000000000); err != nil {
		return o, err
	}

	if err := validations.InRange[float64](fmt.Sprintf("plans[%d].expenses", n), o.Expenses, 0, 1000000000); err != nil {
		return o, err
	}

	if err := validations.InRange[int](fmt.Sprintf("projects[%d].description", n), len(o.Description), 0, 1024); err != nil {
		return o, err
	}

	return o, nil
}
