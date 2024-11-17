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
        updated_at = NOW(),
        crawl_errors = 0,
        crawl_message = ''
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

-- name: get-manifests
WITH man AS (
    SELECT * FROM manifests 
    WHERE 
    (CASE
        WHEN $1 > 0 THEN id = $1
        WHEN $2 != '' THEN guid = $2
        ELSE TRUE
    END)
    AND (CASE 
        WHEN $5 != '' THEN status = $5::manifest_status
        ELSE TRUE
    END)
),
entity AS (
    SELECT m.id, TO_JSON(e) AS entity_raw 
    FROM entities e
    LEFT JOIN man m ON e.manifest_id = m.id
),
prj AS (
    SELECT m.id, COALESCE(JSON_AGG(TO_JSON(p)), '[]'::json) AS projects_raw
    FROM projects p 
    JOIN man m ON p.manifest_id = m.id
    GROUP BY m.id
)
SELECT m.id, m.guid, m.version, m.url, m.funding AS funding_raw, 
       m.status, m.status_message, m.crawl_errors, 
       m.crawl_message, m.created_at, m.updated_at, 
       COALESCE(e.entity_raw, '[]'::json) AS entity_raw, 
       COALESCE(p.projects_raw, '[]'::json) AS projects_raw
FROM man m
    LEFT JOIN entity e ON e.id = m.id
    LEFT JOIN prj p ON p.id = m.id
    WHERE m.id > $3 ORDER BY m.id LIMIT $4;


-- name: get-manifest-status
SELECT status FROM manifests WHERE url = $1;

-- name: get-for-crawling
SELECT id, url, updated_at, status FROM manifests
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

-- name: delete-manifest
DELETE FROM manifests WHERE
    CASE
        WHEN $1 > 0 THEN id = $1
        WHEN $2 != '' THEN guid = $2
    END;

-- name: insert-report
INSERT INTO reports (manifest_id, reason) 
VALUES (
    $1,
    $2
);

-- name: get-recent-projects
WITH ranked_projects AS (
    SELECT 
        p.id,
        p.guid AS project_guid,
        p.manifest_id,
        m.guid AS manifest_guid,
        e.name AS entity_name,
        e.type AS entity_type,
        (SELECT COUNT(*) FROM projects WHERE manifest_id = p.manifest_id) AS entity_num_projects,
        p.name,
        p.description,
        p.webpage_url,
        p.repository_url,
        p.licenses,
        p.tags,
        p.created_at,
        ROW_NUMBER() OVER (PARTITION BY p.manifest_id ORDER BY p.created_at DESC) AS rn
    FROM projects p
    JOIN manifests m ON p.manifest_id = m.id AND m.status = 'active'
    JOIN entities e ON e.manifest_id = m.id
)
SELECT 
    id,
    project_guid,
    manifest_id,
    manifest_guid,
    entity_name,
    entity_type,
    entity_num_projects,
    name,
    description,
    webpage_url,
    repository_url,
    licenses,
    tags,
    created_at
FROM ranked_projects
WHERE rn <= 2
ORDER BY created_at DESC
LIMIT $1;