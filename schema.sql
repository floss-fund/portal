CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- manifests
DROP TYPE IF EXISTS manifest_status CASCADE; CREATE TYPE manifest_status AS ENUM ('pending', 'active', 'expiring', 'disabled', 'blocked');
DROP TABLE IF EXISTS manifests CASCADE;
CREATE TABLE manifests (
    id                   SERIAL PRIMARY KEY,
    guid                 TEXT NOT NULL UNIQUE,

    version              TEXT NOT NULL,
    url                  TEXT NOT NULL UNIQUE,
    funding              JSONB NOT NULL DEFAULT '{}',
    meta                 JSONB NOT NULL DEFAULT '{}',
    status               manifest_status NOT NULL DEFAULT 'pending',
    status_message       TEXT NULL,
    crawl_errors         INT NOT NULL DEFAULT 0,
    crawl_message        TEXT NULL,

    created_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
DROP INDEX IF EXISTS idx_funding_channels; CREATE INDEX idx_funding_channels ON manifests USING GIN ((funding->'channels'));
DROP INDEX IF EXISTS idx_funding_plans; CREATE INDEX idx_funding_plans ON manifests USING GIN ((funding->'plans'));
DROP INDEX IF EXISTS idx_funding_history; CREATE INDEX idx_funding_history ON manifests USING GIN ((funding->'history'));

-- -- entities
DROP TYPE IF EXISTS entity_type CASCADE; CREATE TYPE entity_type AS ENUM ('individual', 'group', 'organisation', 'other');
DROP TYPE IF EXISTS entity_role CASCADE; CREATE TYPE entity_role AS ENUM ('owner', 'steward', 'maintainer', 'contributor', 'other');
DROP TABLE IF EXISTS entities CASCADE;
CREATE TABLE IF NOT EXISTS entities (
    id                  SERIAL PRIMARY KEY,

    type                entity_type NOT NULL,
    role                entity_role NOT NULL,
    name                TEXT NOT NULL,
    email               TEXT NOT NULL,
    phone               TEXT NULL,
    webpage_url         TEXT NOT NULL,
    webpage_wellknown   TEXT NULL,
    meta                JSONB NOT NULL DEFAULT '{}',
    manifest_id         INTEGER UNIQUE REFERENCES manifests(id) ON DELETE CASCADE ON UPDATE CASCADE,

    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
DROP INDEX IF EXISTS idx_entity_manifest; CREATE INDEX idx_entity_manifest ON entities(manifest_id);
DROP INDEX IF EXISTS idx_entity_name; CREATE INDEX idx_entity_name ON entities USING GIN (LOWER(name) gin_trgm_ops);
DROP INDEX IF EXISTS idx_entity_email; CREATE INDEX idx_entity_email ON entities(LOWER(email));

-- projects
DROP TABLE IF EXISTS projects CASCADE;
CREATE TABLE IF NOT EXISTS projects (
    id                   SERIAL PRIMARY KEY,

    guid                 TEXT NOT NULL,
    name                 TEXT NOT NULL,
    description          TEXT NOT NULL,
    webpage_url          TEXT NOT NULL,
    webpage_wellknown    TEXT NULL,
    repository_url       TEXT NOT NULL,
    repository_wellknown TEXT NULL,
    licenses             TEXT[] NOT NULL,
    tags                 TEXT[] NOT NULL,
    meta                 JSONB NOT NULL DEFAULT '{}',
    manifest_id          INTEGER REFERENCES manifests(id) ON DELETE CASCADE ON UPDATE CASCADE,

    created_at           TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at           TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
DROP INDEX IF EXISTS idx_project_guid; CREATE UNIQUE INDEX idx_project_guid ON projects(manifest_id, guid);
DROP INDEX IF EXISTS idx_project_manifest; CREATE INDEX idx_project_manifest ON projects(manifest_id);
DROP INDEX IF EXISTS idx_project_name; CREATE INDEX idx_project_name ON projects USING GIN (LOWER(name) gin_trgm_ops);
DROP INDEX IF EXISTS idx_project_licenses; CREATE INDEX idx_project_licenses ON projects USING GIN (licenses);
DROP INDEX IF EXISTS idx_project_tags; CREATE INDEX idx_project_tags ON projects USING GIN (tags);

-- settings
DROP TABLE IF EXISTS settings CASCADE;
CREATE TABLE settings (
    key                 TEXT NOT NULL UNIQUE,
    value               JSONB NOT NULL DEFAULT '{}',
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
DROP INDEX IF EXISTS idx_settings_key; CREATE INDEX idx_settings_key ON settings(key);

-- top tags.
DROP MATERIALIZED VIEW IF EXISTS top_tags;
CREATE MATERIALIZED VIEW top_tags AS
SELECT unnest(tags) AS tag, COUNT(*) AS tag_count FROM projects GROUP BY unnest(tags) ORDER BY tag_count DESC LIMIT 1000;
