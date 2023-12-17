package v1

import (
	"encoding/json"
	"fmt"
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
}

// New returns a new instance of Schema.
func New(exactVersion string, opt *Opt) *Schema {
	return &Schema{
		exactVersion: exactVersion,
		opt:          opt,
	}
}

// Parse parses and validates a given JSON body against the schema and returns the parsed object.
func (s *Schema) Parse(b []byte) (Entry, error) {
	var r Entry
	if err := json.Unmarshal(b, &r); err != nil {
		return r, err
	}

	// Entity.
	if err := s.ValidateEntity(r.Entity); err != nil {
		return r, err
	}

	// Projects.
	for n, o := range r.Projects {
		if err := s.ValidateProject(o, n); err != nil {
			return r, err
		}
	}

	// Funding channels.
	chIDs := make(map[string]struct{})
	for n, o := range r.Funding.Channels {
		if err := s.ValidateChannel(o, n); err != nil {
			return r, err
		}

		chIDs[o.ID] = struct{}{}
	}

	// Funding plans.
	for n, o := range r.Funding.Plans {
		if err := s.ValidatePlan(o, n, chIDs); err != nil {
			return r, err
		}
	}

	// History.
	for n, o := range r.Funding.History {
		if err := s.ValidateHistory(o, n); err != nil {
			return r, err
		}
	}

	return r, nil
}

func (s *Schema) ValidateEntity(o Entity) error {
	if err := validations.InList("entity.type", o.Type, EntityTypes); err != nil {
		return err
	}

	if err := validations.InList("entity.role", o.Type, EntityRoles); err != nil {
		return err
	}

	if err := validations.InRange[int]("entity.name", len(o.Name), 2, 128); err != nil {
		return err
	}

	if err := validations.IsEmail("entity.email", o.Email, 128); err != nil {
		return err
	}

	if err := validations.InRange[int]("entity.telephone", len(o.Telephone), 0, 24); err != nil {
		return err
	}

	if err := validations.CheckURL("entity.webpageUrl", o.WebpageURL.URL, o.WebpageURL.WellKnown, 1024); err != nil {
		return err
	}

	return nil
}

func (s *Schema) ValidateProject(o Project, n int) error {
	if err := validations.InRange[int](fmt.Sprintf("projects[%d].name", n), len(o.Name), 1, 256); err != nil {
		return err
	}

	if err := validations.InRange[int](fmt.Sprintf("projects[%d].description", n), len(o.Description), 1, 1024); err != nil {
		return err
	}

	if err := validations.CheckURL(fmt.Sprintf("projects[%d].webpageUrl", n), o.WebpageURL.URL, o.WebpageURL.WellKnown, 1024); err != nil {
		return err
	}

	if err := validations.CheckURL(fmt.Sprintf("projects[%d].repositoryURL", n), o.RepositoryUrl.URL, o.RepositoryUrl.WellKnown, 1024); err != nil {
		return err
	}

	// License.
	licenseTag := fmt.Sprintf("projects[%d].license", n)
	if err := validations.InRange[int](licenseTag, len(o.License), 2, 64); err != nil {
		return err
	}
	if strings.HasPrefix(o.License, "spdx:") {
		if err := validations.InMap(licenseTag, "spdx license list", o.License, s.opt.Licenses); err != nil {
			return err
		}
	}

	// Languages.
	for i, lang := range o.Languages {
		langTag := fmt.Sprintf("projects[%d].languages[%d]", n, i)
		if err := validations.InRange[int](langTag, len(lang), 2, 64); err != nil {
			return err
		}

		if err := validations.InMap(langTag, "default programming language list", lang, s.opt.ProgrammingLanguages); err != nil {
			return err
		}
	}

	// Tags.
	if err := validations.MaxItems[[]string](fmt.Sprintf("projects[%d].tags", n), o.Tags, 10); err != nil {
		return err
	}
	for i, t := range o.Tags {
		if err := validations.IsTag(fmt.Sprintf("projects[%d].tags[%d]", n, i), t, 2, 32); err != nil {
			return err
		}
	}

	return nil
}

func (s *Schema) ValidateChannel(o Channel, n int) error {
	if err := validations.IsID(fmt.Sprintf("channels[%d].id", n), o.ID, 3, 32); err != nil {
		return err
	}

	if err := validations.InList(fmt.Sprintf("channels[%d].type", n), o.Type, EntityTypes); err != nil {
		return err
	}

	if err := validations.InRange[int](fmt.Sprintf("channels[%d].address", n), len(o.Address), 0, 128); err != nil {
		return err
	}

	if err := validations.InRange[int](fmt.Sprintf("channels[%d].description", n), len(o.Description), 0, 1024); err != nil {
		return err
	}

	return nil
}

func (s *Schema) ValidatePlan(o Plan, n int, channelIDs map[string]struct{}) error {
	if err := validations.IsID(fmt.Sprintf("plans[%d].id", n), o.ID, 3, 32); err != nil {
		return err
	}

	if err := validations.InList(fmt.Sprintf("plans[%d].status", n), o.Status, PlanStatuses); err != nil {
		return err
	}

	if err := validations.InRange[int](fmt.Sprintf("plans[%d].name", n), len(o.Name), 3, 128); err != nil {
		return err
	}

	if err := validations.InRange[int](fmt.Sprintf("plans[%d].description", n), len(o.Description), 3, 1024); err != nil {
		return err
	}

	if err := validations.InRange[float64](fmt.Sprintf("plans[%d].amount", n), o.Amount, 0, 1000000000); err != nil {
		return err
	}

	if err := validations.InMap(fmt.Sprintf("plans[%d].currency", n), "currencies list", o.Currency, s.opt.Currencies); err != nil {
		return err
	}

	if err := validations.InList(fmt.Sprintf("plans[%d].frequency", n), o.Frequency, PlanFrequencies); err != nil {
		return err
	}

	for _, ch := range o.Channels {
		if _, ok := channelIDs[ch]; !ok {
			return fmt.Errorf("unknown channel id in plans[%d].frequency", n)
		}
	}

	return nil
}

func (s *Schema) ValidateHistory(o History, n int) error {
	if err := validations.InRange[int](fmt.Sprintf("history[%d].year", n), o.Year, 1970, 2050); err != nil {
		return err
	}

	if err := validations.InRange[float64](fmt.Sprintf("plans[%d].income", n), o.Income, 0, 1000000000); err != nil {
		return err
	}

	if err := validations.InRange[float64](fmt.Sprintf("plans[%d].expenses", n), o.Expenses, 0, 1000000000); err != nil {
		return err
	}

	if err := validations.InRange[int](fmt.Sprintf("projects[%d].description", n), len(o.Description), 0, 1024); err != nil {
		return err
	}

	return nil
}
