-- name: upsert-manifest
INSERT INTO manifests (version, url, body, entity, projects, funding_channels, funding_plans, funding_history, meta, status)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	ON CONFLICT (url) DO UPDATE SET
		version=EXCLUDED.version,
		body=EXCLUDED.body,
		entity=EXCLUDED.entity,
		projects=EXCLUDED.projects,
		funding_channels=EXCLUDED.funding_channels,
		funding_plans=EXCLUDED.funding_plans,
		funding_history=EXCLUDED.funding_history,
		meta=EXCLUDED.meta,
		status=(
			CASE WHEN $10 = 'pending' AND manifests.status = 'active' THEN 'active' ELSE EXCLUDED.status END
		),
		updated_at=NOW()
	WHERE manifests.status != 'disabled'
	RETURNING *;

-- name: get-for-crawling
SELECT id, url FROM manifests WHERE id > $1 AND updated_at < NOW() - $2::INTERVAL AND status != 'disabled' ORDER BY id LIMIT $3;

-- name: update-manifest-status
UPDATE manifests SET status=$2 WHERE id=$1;

-- name: get-top-tags
SELECT tag FROM top_tags LIMIT $1;
