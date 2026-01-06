-- Events table (raw events)
CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    owner TEXT NOT NULL,
    owner_type TEXT NOT NULL DEFAULT 'organization',
    repo TEXT NOT NULL,
    member TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    data TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_events_owner_repo ON events(owner, repo);
CREATE INDEX IF NOT EXISTS idx_events_member ON events(member);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_owner_type_timestamp ON events(owner, type, timestamp);
CREATE INDEX IF NOT EXISTS idx_events_owner_type ON events(owner_type);

-- Repositories table (repository metadata)
CREATE TABLE IF NOT EXISTS repositories (
    owner TEXT NOT NULL,
    owner_type TEXT NOT NULL DEFAULT 'organization',
    name TEXT NOT NULL,
    full_name TEXT NOT NULL,
    is_private INTEGER NOT NULL,
    last_synced_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (owner, name)
);

CREATE INDEX IF NOT EXISTS idx_repositories_owner ON repositories(owner);
CREATE INDEX IF NOT EXISTS idx_repositories_owner_type ON repositories(owner_type);

-- Members table (member metadata)
CREATE TABLE IF NOT EXISTS members (
    owner TEXT NOT NULL,
    owner_type TEXT NOT NULL DEFAULT 'organization',
    username TEXT NOT NULL,
    display_name TEXT,
    last_synced_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (owner, username)
);

CREATE INDEX IF NOT EXISTS idx_members_owner ON members(owner);
CREATE INDEX IF NOT EXISTS idx_members_owner_type ON members(owner_type);

-- Collection batches table (batch collection jobs)
CREATE TABLE IF NOT EXISTS collection_batches (
    id TEXT PRIMARY KEY,
    mode TEXT NOT NULL,
    owner TEXT NOT NULL,
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    status TEXT NOT NULL DEFAULT 'in_progress',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_collection_batches_owner ON collection_batches(owner);
CREATE INDEX IF NOT EXISTS idx_collection_batches_status ON collection_batches(status);
CREATE INDEX IF NOT EXISTS idx_collection_batches_mode_owner_dates ON collection_batches(mode, owner, start_date, end_date);
