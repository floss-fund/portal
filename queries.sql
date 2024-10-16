-- name: upsert-manifest
WITH man AS (
    INSERT INTO manifests (version, url, guid, funding, meta, status, status_message)
    VALUES (
        $1::JSONB->>'version',
        $2,
        $3,
        $1::JSONB->'funding',
        $4,
        $5,
        $6
    )
    ON CONFLICT (url) DO UPDATE
    SET version = $1->>'version',
        funding = $1::JSONB->'funding',
        meta = $4,
        status = $5,
        status_message = $6,
        updated_at = NOW()
    RETURNING id
),
entity AS (
    INSERT INTO entities (type, role, name, email, phone, description, webpage_url, webpage_wellknown, manifest_id)
    SELECT
        ($1->'entity'->>'type')::entity_type,
        ($1->'entity'->>'role')::entity_role,
        $1->'entity'->>'name',
        $1->'entity'->>'email',
        $1->'entity'->>'phone',
        $1->'entity'->>'description',
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
    DELETE FROM projects WHERE manifest_id=(SELECT id FROM man) AND guid NOT IN (
        SELECT p->>'guid' FROM JSONB_ARRAY_ELEMENTS($1->'projects') AS p
    )
),
prj AS (
    INSERT INTO projects (
        guid, name, description, webpage_url, webpage_wellknown, repository_url, repository_wellknown, licenses, tags, manifest_id
    )
    SELECT
        project->>'guid',
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
    ON CONFLICT (manifest_id, guid) DO UPDATE
    SET name = EXCLUDED.name,
        description = EXCLUDED.description,
        webpage_url = EXCLUDED.webpage_url,
        webpage_wellknown = EXCLUDED.webpage_wellknown,
        repository_url = EXCLUDED.repository_url,
        repository_wellknown = EXCLUDED.repository_wellknown,
        licenses = EXCLUDED.licenses,
        tags = EXCLUDED.tags
)
SELECT (SELECT id FROM man) AS manifest_id;

-- name: get-manifest
WITH man AS (
    SELECT * FROM manifests WHERE
    CASE
        WHEN $1 > 0 THEN id = $1
        WHEN $2 != '' THEN guid = $2
    END
    AND status = 'active'
),
entity AS (
    SELECT TO_JSON(e) AS entity_raw FROM entities e
    WHERE e.manifest_id = (SELECT id FROM man)
),
prj AS (
    SELECT COALESCE(JSON_AGG(TO_JSON(p)), '[]'::json) AS projects_raw
    FROM projects p WHERE p.manifest_id = (SELECT id FROM man)
)
SELECT m.id, m.guid, m.version, m.url, m.funding AS funding_raw, m.status,
        m.status_message, m.crawl_errors, m.crawl_message, m.created_at, m.updated_at,
        e.entity_raw, p.projects_raw FROM man m
    LEFT JOIN entity e ON true
    LEFT JOIN prj p ON true;

-- name: get-manifest-status
SELECT status FROM manifests WHERE url = $1;

-- name: get-for-crawling
SELECT id, url, updated_at FROM manifests
    WHERE id > $1
    AND updated_at > NOW() - $2::INTERVAL
    AND status != 'disabled'
    AND status != 'blocked'
    ORDER BY id LIMIT $3;

-- name: update-manifest-status
UPDATE manifests SET status=$2 WHERE id=$1;

-- name: get-top-tags
SELECT tag FROM top_tags LIMIT $1;

-- name: update-crawl-error
UPDATE manifests SET
    crawl_errors = crawl_errors + 1,
    crawl_message = $2,
    status = (CASE WHEN crawl_errors + 1 >= $3 THEN 'disabled' ELSE status END)
    WHERE id = $1
    RETURNING status;

-- name: get-pending-manifests
WITH man AS (
    SELECT * FROM manifests m
    WHERE m.status = 'pending'
    ORDER BY m.id
    LIMIT $1 OFFSET $2
),
entity AS (
    SELECT e.manifest_id, TO_JSON(e) AS entity_raw
    FROM entities e
    WHERE e.manifest_id IN (SELECT id FROM man)
),
prj AS (
    SELECT p.manifest_id, COALESCE(JSON_AGG(TO_JSON(p)), '[]'::json) AS projects_raw
    FROM projects p
    WHERE p.manifest_id IN (SELECT id FROM man)
    GROUP BY p.manifest_id
)
SELECT m.id, m.guid, m.version, m.url, m.funding AS funding_raw, m.status,
       m.status_message, m.crawl_errors, m.crawl_message, m.created_at, m.updated_at,
       e.entity_raw, p.projects_raw
FROM man m
LEFT JOIN entity e ON e.manifest_id = m.id
LEFT JOIN prj p ON p.manifest_id = m.id;
