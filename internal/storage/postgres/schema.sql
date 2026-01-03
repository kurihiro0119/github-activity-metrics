-- Events table (raw events)
CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    org TEXT NOT NULL,
    repo TEXT NOT NULL,
    member TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    data JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_events_org_repo ON events(org, repo);
CREATE INDEX IF NOT EXISTS idx_events_member ON events(member);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_org_type_timestamp ON events(org, type, timestamp);

-- Repositories table (repository metadata)
CREATE TABLE IF NOT EXISTS repositories (
    org TEXT NOT NULL,
    name TEXT NOT NULL,
    full_name TEXT NOT NULL,
    is_private BOOLEAN NOT NULL,
    last_synced_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (org, name)
);

CREATE INDEX IF NOT EXISTS idx_repositories_org ON repositories(org);

-- Members table (member metadata)
CREATE TABLE IF NOT EXISTS members (
    org TEXT NOT NULL,
    username TEXT NOT NULL,
    display_name TEXT,
    last_synced_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (org, username)
);

CREATE INDEX IF NOT EXISTS idx_members_org ON members(org);
