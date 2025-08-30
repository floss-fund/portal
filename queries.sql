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
        email = $1->'entity'->>'email',
        phone = $1->'entity'->>'phone',
        description = $1->'entity'->>'description',
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
    AND updated_at < NOW() - $2::INTERVAL
    AND crawl_errors < $3
    AND status != 'disabled'
    AND status != 'blocked'
    ORDER BY id LIMIT $4;

-- name: update-manifest-status
UPDATE manifests SET status=$2 WHERE id=$1;

-- name: update-manifest-date
UPDATE manifests SET updated_at=NOW() WHERE id=$1;

-- name: get-top-tags
SELECT tag FROM top_tags LIMIT $1;

-- name: update-crawl-error
UPDATE manifests SET
    crawl_errors = crawl_errors + 1,
    crawl_message = $2,
    status = (CASE WHEN $4 AND crawl_errors + 1 >= $3 THEN 'disabled' ELSE status END)
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
        p.guid,
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
    guid,
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

-- name: get-projects
WITH project_counts AS (
    SELECT manifest_id, COUNT(*) AS project_count FROM projects GROUP BY manifest_id
)
SELECT
	COUNT(*) OVER () AS total,
    CONCAT(m.guid, '/', p.guid) as id,
    JSONB_BUILD_OBJECT(
        'manifest_id', m.id,
        'manifest_guid', m.guid,
        'entity_id', e.id,
        'type', e.type,
        'role', e.role,
        'name', e.name,
        'num_projects', pc.project_count
    ) AS entity,
    p.name,
    p.description,
    p.webpage_url,
    p.repository_url,
    p.licenses,
    p.tags,
    p.updated_at
FROM projects p
    JOIN manifests m ON p.manifest_id = m.id
    JOIN entities e ON e.manifest_id = m.id
    JOIN project_counts pc ON pc.manifest_id = p.manifest_id
ORDER BY p.%s OFFSET $1 LIMIT $2;

-- name: query-projects-template
-- raw: true
WITH res AS (%query%),
ordIds AS (SELECT id, total, ROW_NUMBER() OVER () AS ord FROM res),
pc AS (
    SELECT p.manifest_id, COUNT(*) AS num FROM projects p JOIN res r ON r.id = p.id GROUP BY p.manifest_id
)
SELECT
    p.*,
    o.ord,
    o.total,

    CONCAT(m.guid, '/', p.guid) as guid,
    JSONB_BUILD_OBJECT(
        'manifest_id', m.id,
        'manifest_guid', m.guid,
        'entity_id', e.id,
        'type', e.type,
        'role', e.role,
        'name', e.name,
        'num_projects', pc.num,
        'webpageUrl', e.webpage_url,
        'webpageWellknown', COALESCE(e.webpage_wellknown, '')
    ) AS entity

    FROM projects AS p
    JOIN manifests m ON m.id = p.manifest_id
    JOIN entities e ON e.manifest_id = p.manifest_id
    JOIN ordIds o ON o.id = p.id
    JOIN pc ON pc.manifest_id = p.manifest_id
ORDER BY o.ord;

-- name: search-projects
-- raw: true
-- $1 plaintext text search term
-- $2 tags[]
-- $3 licenses[]
-- $4 offset
-- $5 limit
SELECT
    COUNT(*) OVER () AS total,
    id,
    CASE
        WHEN $1::TEXT != '' THEN TS_RANK_CD(p.search_tokens, PLAINTO_TSQUERY('simple', $1))
        ELSE 0
    END AS rank
FROM projects p
WHERE
    ($1::TEXT != '' OR p.search_tokens @@ PLAINTO_TSQUERY('simple', $1)) AND
    (CARDINALITY($2::TEXT[]) = 0 OR p.tags <@ $2) AND
    (CARDINALITY($3::TEXT[]) = 0 OR p.licenses <@ $3)
ORDER BY
    CASE
        WHEN $1::TEXT != '' THEN TS_RANK_CD(p.search_tokens, PLAINTO_TSQUERY('simple', $1)) ELSE 0
    END DESC
    OFFSET $4 LIMIT $5


-- name: get-entities
-- raw: true
SELECT
    COUNT(*) OVER () AS total,
    e.*,
    (
        SELECT COUNT(*)
        FROM projects
        WHERE manifest_id = e.manifest_id
    ) AS num_projects,
    m.guid AS manifest_guid
FROM entities e JOIN manifests m ON m.id = e.manifest_id
ORDER BY %s OFFSET $1 LIMIT $2;

-- name: get-manifests-dump
WITH project_json AS (
    SELECT 
        manifest_id,
        JSONB_AGG(JSONB_BUILD_OBJECT(
            'guid', guid,
            'name', name,
            'description', description,
            'webpageUrl', JSONB_BUILD_OBJECT(
                'url', webpage_url,
                'wellKnown', webpage_wellknown
            ),
            'repositoryUrl', JSONB_BUILD_OBJECT(
                'url', repository_url,
                'wellKnown', repository_wellknown
            ),
            'licenses', TO_JSONB(licenses),
            'tags', TO_JSONB(tags)
        )) AS projects_json
    FROM projects
    GROUP BY manifest_id
)
SELECT m.id, m.url, m.created_at, m.updated_at, m.status, JSONB_BUILD_OBJECT(
    'version', m.version,
    'entity', JSONB_BUILD_OBJECT(
        'type', e.type,
        'role', e.role,
        'name', e.name,
        'email', e.email,
        'phone', e.phone,
        'description', e.description,
        'webpageUrl', JSONB_BUILD_OBJECT(
            'url', e.webpage_url,
            'wellKnown', e.webpage_wellknown
        )
    ),
    'projects', COALESCE(p.projects_json, '[]'::JSONB),
    'funding', m.funding
) AS manifest_json
FROM manifests m
    LEFT JOIN entities e ON e.manifest_id = m.id
    LEFT JOIN project_json p ON p.manifest_id = m.id
WHERE m.id > $1 ORDER BY m.id LIMIT $2;

-- name: search-entities
SELECT
    COUNT(*) OVER () AS total,
    t.*,
    t.webpage_url AS "webpageUrl",
    t.webpage_wellknown AS "webpageWellknown",
    TS_RANK_CD(t.search_tokens, PLAINTO_TSQUERY('simple', $1)) AS rank,
    COALESCE(project_counts.num_projects, 0) AS num_projects,
    m.guid AS manifest_guid
FROM entities t
LEFT JOIN (
    SELECT manifest_id, COUNT(*) AS num_projects
    FROM projects GROUP BY manifest_id
) AS project_counts ON project_counts.manifest_id = t.manifest_id
JOIN manifests m ON m.id = t.manifest_id
WHERE t.search_tokens @@ PLAINTO_TSQUERY('simple', $1)
ORDER BY rank DESC, t.id OFFSET $2 LIMIT $3;
