-- name: upsert-manifest
WITH man AS (
    INSERT INTO manifests (version, url, funding, meta, status, status_message)
    VALUES (
        $1::JSONB->>'version',
        $2,
        $1::JSONB->'funding',
        $3,
        $4,
        $5
    )
    ON CONFLICT (url) DO UPDATE
    SET version = $1->>'version', funding = $1::JSONB->'funding', updated_at = NOW()
    RETURNING id
),
entity AS (
    INSERT INTO entities (type, role, name, email, phone, webpage_url, webpage_wellknown, manifest_id)
    SELECT
        ($1->'entity'->>'type')::entity_type,
        ($1->'entity'->>'role')::entity_role,
        $1->'entity'->>'name',
        $1->'entity'->>'email',
        $1->'entity'->>'phone',
        $1->'entity'->'webpageUrl'->>'url',
        $1->'entity'->'webpageUrl'->>'wellKnown',
        (SELECT id FROM man)
    ON CONFLICT (manifest_id) DO UPDATE SET
        type = ($1->'entity'->>'type')::entity_type,
        role = ($1->'entity'->>'role')::entity_role,
        name = $1->'entity'->>'name',
        phone = $1->'entity'->>'phone',
        webpage_url = $1->'entity'->'webpageUrl'->>'url',
        webpage_wellknown = $1->'entity'->'webpageUrl'->>'wellKnown',
        updated_at = NOW()
    RETURNING id
),
delPrj AS (
	-- Delete project IDs that have disappeared from the manifest.
	DELETE FROM projects WHERE project_id::text NOT IN (
	    SELECT p->>'id' FROM JSONB_ARRAY_ELEMENTS($1->'projects') AS p
	)
),
prj AS (
    INSERT INTO projects (
        project_id, name, description, webpage_url, webpage_wellknown, repository_url, repository_wellknown, licenses, tags, manifest_id
    )
    SELECT
        project->>'id',
        project->>'name',
        project->>'description',
        project->'webpageUrl'->>'url',
        project->'webpageUrl'->>'wellKnown',
        project->'repositoryUrl'->>'url',
        project->'repositoryUrl'->>'wellKnown',
        ARRAY(SELECT JSONB_ARRAY_ELEMENTS_TEXT(project->'licenses')),
        ARRAY(SELECT JSONB_ARRAY_ELEMENTS_TEXT(project->'tags')),
        (SELECT id FROM man) AS manifest_id
    FROM JSONB_ARRAY_ELEMENTS($1->'projects') AS project
)
SELECT (SELECT id FROM man) AS manifest_id;

-- name: check-manifest-exists
SELECT id  FROM manifests WHERE url = $1;

-- name: get-for-crawling
SELECT id, uuid, url FROM manifests WHERE id > $1 AND updated_at < NOW() - $2::INTERVAL AND status != 'disabled' ORDER BY id LIMIT $3;

-- name: update-manifest-status
UPDATE manifests SET status=$2 WHERE id=$1;

-- name: get-top-tags
SELECT tag FROM top_tags LIMIT $1;
