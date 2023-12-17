-- name: upsert-entry
INSERT INTO entries (version, manifest_url, manifest, entity, projects, funding_channels, funding_plans, funding_history, meta, status)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	ON CONFLICT (manifest_url) DO UPDATE SET
		version=EXCLUDED.version,
		manifest=EXCLUDED.manifest,
		entity=EXCLUDED.entity,
		projects=EXCLUDED.projects,
		funding_channels=EXCLUDED.funding_channels,
		funding_plans=EXCLUDED.funding_plans,
		funding_history=EXCLUDED.funding_history,
		meta=EXCLUDED.meta,
		status=EXCLUDED.status,
		updated_at=NOW()
	WHERE entries.status != 'disabled'
	RETURNING *;
