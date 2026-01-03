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

-- Metrics table (aggregated metrics)
CREATE TABLE IF NOT EXISTS metrics (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    org TEXT NOT NULL,
    repo TEXT,
    member TEXT,
    value BIGINT NOT NULL,
    time_range_start TIMESTAMP NOT NULL,
    time_range_end TIMESTAMP NOT NULL,
    granularity TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org, repo, member, type, time_range_start, time_range_end, granularity)
);

CREATE INDEX IF NOT EXISTS idx_metrics_org ON metrics(org);
CREATE INDEX IF NOT EXISTS idx_metrics_repo ON metrics(org, repo);
CREATE INDEX IF NOT EXISTS idx_metrics_member ON metrics(org, member);
CREATE INDEX IF NOT EXISTS idx_metrics_time_range ON metrics(time_range_start, time_range_end);

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
