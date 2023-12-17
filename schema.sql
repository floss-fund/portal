CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- entities
DROP TYPE IF EXISTS entity_type CASCADE; CREATE TYPE entity_type AS ENUM ('individual', 'group', 'organisation', 'other');
DROP TYPE IF EXISTS entity_role CASCADE; CREATE TYPE entity_role AS ENUM ('owner', 'steward', 'maintainer', 'contributor', 'other');
DROP TABLE IF EXISTS entities CASCADE;
CREATE TABLE IF NOT EXISTS entities (
    id                   SERIAL PRIMARY KEY,
    uuid                 UUID NOT NULL UNIQUE DEFAULT GEN_RANDOM_UUID(),

    type                 entity_type NOT NULL,
    role                 entity_role NOT NULL,
    name                 TEXT NOT NULL,
    email                TEXT NOT NULL,
    telephone            TEXT NOT NULL DEFAULT '',
    webpage_url          TEXT NOT NULL,
    webpage_wellknown    TEXT NULL
);

-- projects
DROP TABLE IF EXISTS projects CASCADE;
CREATE TABLE IF NOT EXISTS projects (
    id                   SERIAL PRIMARY KEY,
    uuid                 UUID NOT NULL UNIQUE DEFAULT GEN_RANDOM_UUID(),

    name                 TEXT NOT NULL,
    description          TEXT NOT NULL,
    webpage_url          TEXT NOT NULL,
    webpage_wellknown    TEXT NULL,
    repository_url       TEXT NOT NULL,
    repository_wellknown TEXT NULL,
    license              TEXT NOT NULL,
    languages            TEXT[] NOT NULL,
    tags                 TEXT[] NOT NULL
);

-- entries
DROP TYPE IF EXISTS entry_status CASCADE; CREATE TYPE entry_status AS ENUM ('pending', 'enabled', 'expiring', 'disabled');
DROP TABLE IF EXISTS entries CASCADE;
CREATE TABLE entries (
    id                  SERIAL PRIMARY KEY,
    uuid                UUID NOT NULL UNIQUE DEFAULT GEN_RANDOM_UUID(),

    version             TEXT NOT NULL,
    schema_url          TEXT NOT NULL,
    funding_channels    JSONB NOT NULL DEFAULT '{}',
    funding_plans       JSONB NOT NULL DEFAULT '{}',
    funding_HISTORY     JSONB NOT NULL DEFAULT '{}',
    entity_id           INTEGER REFERENCES entities(id) ON DELETE CASCADE ON UPDATE CASCADE, 
    status              entry_status NOT NULL DEFAULT 'pending',
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
DROP INDEX IF EXISTS idx_uuid; CREATE UNIQUE INDEX idx_uuid ON entries(uuid);
DROP INDEX IF EXISTS idx_version; CREATE UNIQUE INDEX idx_version ON entries(version);
DROP INDEX IF EXISTS idx_status; CREATE UNIQUE INDEX idx_status ON entries(status);

-- settings
DROP TABLE IF EXISTS settings CASCADE;
CREATE TABLE settings (
    key             TEXT NOT NULL UNIQUE,
    value           JSONB NOT NULL DEFAULT '{}',
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
DROP INDEX IF EXISTS idx_settings_key; CREATE INDEX idx_settings_key ON settings(key);
