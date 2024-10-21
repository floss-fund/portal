package main

import (
	"log"

	"github.com/floss-fund/portal/internal/core"
	"github.com/floss-fund/portal/internal/models"
	"github.com/floss-fund/portal/internal/search"
)

func syncSearch(c *core.Core, s *search.Search, lo *log.Logger) {
	var (
		lastID = 0
		total  = 0
	)
	for {
		items, err := c.GetManifests(lastID, 1000)
		if err != nil {
			lo.Fatalf("error fetching manifests: %v", err)
		}
		if len(items) == 0 {
			break
		}

		// Update each record to the search backend.
		for _, item := range items {
			item := item
			updateSearchRecord(item, item.Status, s)
		}

		lastID = items[len(items)-1].ID
		total += len(items)
	}

	lo.Printf("synced %d items", total)
}

func updateSearchRecord(m models.ManifestData, status string, s *search.Search) {
	// Delete all search data (entity, projects) on the manifest.
	_ = s.Delete(m.ID)

	// If it's active, re-insert it into the search index.
	if status == core.ManifestStatusActive {
		_ = s.InsertEntity(search.Entity{
			ID:           m.GUID,
			ManifestID:   m.ID,
			ManifestGUID: m.GUID,
			Type:         m.Manifest.Entity.Type,
			Role:         m.Manifest.Entity.Role,
			Name:         m.Manifest.Entity.Name,
			Description:  m.Manifest.Entity.Description,
			WebpageURL:   m.Manifest.Entity.WebpageURL.URL,
			NumProjects:  len(m.Manifest.Projects),
			UpdatedAt:    m.CreatedAt.Unix(),
		})

		for _, p := range m.Manifest.Projects {
			_ = s.InsertProject(search.Project{
				ID:                m.GUID + "/" + p.GUID,
				ManifestID:        m.ID,
				ManifestGUID:      m.GUID,
				EntityType:        m.Manifest.Entity.Type,
				EntityName:        m.Manifest.Entity.Name,
				EntityNumProjects: len(m.Manifest.Projects),
				Name:              p.Name,
				WebpageURL:        p.WebpageURL.URL,
				RepositoryURL:     p.RepositoryURL.URL,
				Description:       p.Description,
				Licenses:          p.Licenses,
				Tags:              p.Tags,
				UpdatedAt:         m.CreatedAt.Unix(),
			})
		}
	}
}
