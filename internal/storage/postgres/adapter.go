package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	_ "github.com/lib/pq"

	"github.com/kurihiro0119/github-activity-metrics/internal/domain"
	"github.com/kurihiro0119/github-activity-metrics/internal/storage"
)

// postgresStorage implements the Storage interface for PostgreSQL
type postgresStorage struct {
	db *sql.DB
}

// NewPostgresStorage creates a new PostgreSQL storage instance
func NewPostgresStorage(connStr string) (storage.Storage, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	s := &postgresStorage{db: db}
	if err := s.Migrate(context.Background()); err != nil {
		return nil, err
	}

	return s, nil
}

// Migrate runs database migrations
func (s *postgresStorage) Migrate(ctx context.Context) error {
	schema := `
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
	`

	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// SaveRawEvent saves a single raw event
func (s *postgresStorage) SaveRawEvent(ctx context.Context, event *domain.Event) error {
	dataJSON, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO events (id, type, org, repo, member, timestamp, data, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			type = EXCLUDED.type,
			org = EXCLUDED.org,
			repo = EXCLUDED.repo,
			member = EXCLUDED.member,
			timestamp = EXCLUDED.timestamp,
			data = EXCLUDED.data
	`
	_, err = s.db.ExecContext(ctx, query,
		event.ID,
		string(event.Type),
		event.Org,
		event.Repo,
		event.Member,
		event.Timestamp,
		string(dataJSON),
		event.CreatedAt,
	)
	return err
}

// SaveRawEvents saves multiple raw events
func (s *postgresStorage) SaveRawEvents(ctx context.Context, events []*domain.Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO events (id, type, org, repo, member, timestamp, data, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			type = EXCLUDED.type,
			org = EXCLUDED.org,
			repo = EXCLUDED.repo,
			member = EXCLUDED.member,
			timestamp = EXCLUDED.timestamp,
			data = EXCLUDED.data
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, event := range events {
		dataJSON, err := json.Marshal(event.Data)
		if err != nil {
			return err
		}

		_, err = stmt.ExecContext(ctx,
			event.ID,
			string(event.Type),
			event.Org,
			event.Repo,
			event.Member,
			event.Timestamp,
			string(dataJSON),
			event.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// SaveAggregatedMetric saves a single aggregated metric
func (s *postgresStorage) SaveAggregatedMetric(ctx context.Context, metric *domain.Metric) error {
	var metadataJSON []byte
	var err error
	if metric.Metadata != nil {
		metadataJSON, err = json.Marshal(metric.Metadata)
		if err != nil {
			return err
		}
	}

	query := `
		INSERT INTO metrics (id, type, org, repo, member, value, time_range_start, time_range_end, granularity, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (org, repo, member, type, time_range_start, time_range_end, granularity) DO UPDATE SET
			value = EXCLUDED.value,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`
	_, err = s.db.ExecContext(ctx, query,
		metric.ID,
		string(metric.Type),
		metric.Org,
		metric.Repo,
		metric.Member,
		metric.Value,
		metric.TimeRange.Start,
		metric.TimeRange.End,
		metric.TimeRange.Granularity,
		string(metadataJSON),
		metric.CreatedAt,
		metric.UpdatedAt,
	)
	return err
}

// SaveAggregatedMetrics saves multiple aggregated metrics
func (s *postgresStorage) SaveAggregatedMetrics(ctx context.Context, metrics []*domain.Metric) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO metrics (id, type, org, repo, member, value, time_range_start, time_range_end, granularity, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (org, repo, member, type, time_range_start, time_range_end, granularity) DO UPDATE SET
			value = EXCLUDED.value,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, metric := range metrics {
		var metadataJSON []byte
		if metric.Metadata != nil {
			metadataJSON, err = json.Marshal(metric.Metadata)
			if err != nil {
				return err
			}
		}

		_, err = stmt.ExecContext(ctx,
			metric.ID,
			string(metric.Type),
			metric.Org,
			metric.Repo,
			metric.Member,
			metric.Value,
			metric.TimeRange.Start,
			metric.TimeRange.End,
			metric.TimeRange.Granularity,
			string(metadataJSON),
			metric.CreatedAt,
			metric.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetMetricsByOrg retrieves organization-level metrics
func (s *postgresStorage) GetMetricsByOrg(ctx context.Context, org string, timeRange domain.TimeRange) (*domain.OrgMetrics, error) {
	metrics := &domain.OrgMetrics{
		Org:       org,
		TimeRange: timeRange,
	}

	// Get total repos
	var totalRepos int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM repositories WHERE org = $1`, org).Scan(&totalRepos)
	if err != nil {
		return nil, err
	}
	metrics.TotalRepos = totalRepos

	// Get total members
	var totalMembers int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM members WHERE org = $1`, org).Scan(&totalMembers)
	if err != nil {
		return nil, err
	}
	metrics.TotalMembers = totalMembers

	// Get commits count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE org = $1 AND type = 'commit' AND timestamp >= $2 AND timestamp <= $3
	`, org, timeRange.Start, timeRange.End).Scan(&metrics.Commits)
	if err != nil {
		return nil, err
	}

	// Get PRs count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE org = $1 AND type = 'pull_request' AND timestamp >= $2 AND timestamp <= $3
	`, org, timeRange.Start, timeRange.End).Scan(&metrics.PRs)
	if err != nil {
		return nil, err
	}

	// Get deploys count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE org = $1 AND type = 'deploy' AND timestamp >= $2 AND timestamp <= $3
	`, org, timeRange.Start, timeRange.End).Scan(&metrics.Deploys)
	if err != nil {
		return nil, err
	}

	// Get additions and deletions from commit events using JSONB
	err = s.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM((data->>'additions')::int), 0),
			COALESCE(SUM((data->>'deletions')::int), 0)
		FROM events 
		WHERE org = $1 AND type = 'commit' AND timestamp >= $2 AND timestamp <= $3
	`, org, timeRange.Start, timeRange.End).Scan(&metrics.Additions, &metrics.Deletions)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// GetMetricsByMember retrieves member-level metrics
func (s *postgresStorage) GetMetricsByMember(ctx context.Context, org, member string, timeRange domain.TimeRange) (*domain.MemberMetrics, error) {
	metrics := &domain.MemberMetrics{
		Member:    member,
		TimeRange: timeRange,
	}

	// Get commits count
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE org = $1 AND member = $2 AND type = 'commit' AND timestamp >= $3 AND timestamp <= $4
	`, org, member, timeRange.Start, timeRange.End).Scan(&metrics.Commits)
	if err != nil {
		return nil, err
	}

	// Get PRs count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE org = $1 AND member = $2 AND type = 'pull_request' AND timestamp >= $3 AND timestamp <= $4
	`, org, member, timeRange.Start, timeRange.End).Scan(&metrics.PRs)
	if err != nil {
		return nil, err
	}

	// Get deploys count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE org = $1 AND member = $2 AND type = 'deploy' AND timestamp >= $3 AND timestamp <= $4
	`, org, member, timeRange.Start, timeRange.End).Scan(&metrics.Deploys)
	if err != nil {
		return nil, err
	}

	// Get additions and deletions from commit events using JSONB
	err = s.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM((data->>'additions')::int), 0),
			COALESCE(SUM((data->>'deletions')::int), 0)
		FROM events 
		WHERE org = $1 AND member = $2 AND type = 'commit' AND timestamp >= $3 AND timestamp <= $4
	`, org, member, timeRange.Start, timeRange.End).Scan(&metrics.Additions, &metrics.Deletions)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// GetMetricsByRepo retrieves repository-level metrics
func (s *postgresStorage) GetMetricsByRepo(ctx context.Context, org, repo string, timeRange domain.TimeRange) (*domain.RepoMetrics, error) {
	metrics := &domain.RepoMetrics{
		Repo:      repo,
		TimeRange: timeRange,
	}

	// Get commits count
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE org = $1 AND repo = $2 AND type = 'commit' AND timestamp >= $3 AND timestamp <= $4
	`, org, repo, timeRange.Start, timeRange.End).Scan(&metrics.Commits)
	if err != nil {
		return nil, err
	}

	// Get PRs count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE org = $1 AND repo = $2 AND type = 'pull_request' AND timestamp >= $3 AND timestamp <= $4
	`, org, repo, timeRange.Start, timeRange.End).Scan(&metrics.PRs)
	if err != nil {
		return nil, err
	}

	// Get deploys count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE org = $1 AND repo = $2 AND type = 'deploy' AND timestamp >= $3 AND timestamp <= $4
	`, org, repo, timeRange.Start, timeRange.End).Scan(&metrics.Deploys)
	if err != nil {
		return nil, err
	}

	// Get additions and deletions from commit events using JSONB
	err = s.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM((data->>'additions')::int), 0),
			COALESCE(SUM((data->>'deletions')::int), 0)
		FROM events 
		WHERE org = $1 AND repo = $2 AND type = 'commit' AND timestamp >= $3 AND timestamp <= $4
	`, org, repo, timeRange.Start, timeRange.End).Scan(&metrics.Additions, &metrics.Deletions)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// GetTimeSeriesMetrics retrieves time series metrics
func (s *postgresStorage) GetTimeSeriesMetrics(ctx context.Context, org string, metricType domain.MetricType, timeRange domain.TimeRange) ([]*domain.Metric, error) {
	query := `
		SELECT id, type, org, repo, member, value, time_range_start, time_range_end, granularity, metadata, created_at, updated_at
		FROM metrics
		WHERE org = $1 AND type = $2 AND time_range_start >= $3 AND time_range_end <= $4 AND granularity = $5
		ORDER BY time_range_start
	`
	rows, err := s.db.QueryContext(ctx, query, org, string(metricType), timeRange.Start, timeRange.End, timeRange.Granularity)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []*domain.Metric
	for rows.Next() {
		var m domain.Metric
		var repo, member sql.NullString
		var metadataStr sql.NullString
		var start, end time.Time

		err := rows.Scan(&m.ID, &m.Type, &m.Org, &repo, &member, &m.Value, &start, &end, &m.TimeRange.Granularity, &metadataStr, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if repo.Valid {
			m.Repo = &repo.String
		}
		if member.Valid {
			m.Member = &member.String
		}
		m.TimeRange.Start = start
		m.TimeRange.End = end

		if metadataStr.Valid && metadataStr.String != "" {
			var metadata map[string]interface{}
			if err := json.Unmarshal([]byte(metadataStr.String), &metadata); err == nil {
				m.Metadata = metadata
			}
		}

		metrics = append(metrics, &m)
	}

	return metrics, nil
}

// GetEvents retrieves events for re-aggregation
func (s *postgresStorage) GetEvents(ctx context.Context, org string, eventType domain.EventType, timeRange domain.TimeRange) ([]*domain.Event, error) {
	query := `
		SELECT id, type, org, repo, member, timestamp, data, created_at
		FROM events
		WHERE org = $1 AND type = $2 AND timestamp >= $3 AND timestamp <= $4
		ORDER BY timestamp
	`
	rows, err := s.db.QueryContext(ctx, query, org, string(eventType), timeRange.Start, timeRange.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*domain.Event
	for rows.Next() {
		var e domain.Event
		var dataStr string

		err := rows.Scan(&e.ID, &e.Type, &e.Org, &e.Repo, &e.Member, &e.Timestamp, &dataStr, &e.CreatedAt)
		if err != nil {
			return nil, err
		}

		if dataStr != "" {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(dataStr), &data); err == nil {
				e.Data = data
			}
		}

		events = append(events, &e)
	}

	return events, nil
}

// SaveRepository saves a repository
func (s *postgresStorage) SaveRepository(ctx context.Context, repo *domain.Repository) error {
	query := `
		INSERT INTO repositories (org, name, full_name, is_private, last_synced_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (org, name) DO UPDATE SET
			full_name = EXCLUDED.full_name,
			is_private = EXCLUDED.is_private,
			last_synced_at = EXCLUDED.last_synced_at,
			updated_at = EXCLUDED.updated_at
	`
	_, err := s.db.ExecContext(ctx, query,
		repo.Org,
		repo.Name,
		repo.FullName,
		repo.IsPrivate,
		repo.LastSyncedAt,
		repo.CreatedAt,
		repo.UpdatedAt,
	)
	return err
}

// GetRepositories retrieves all repositories for an organization
func (s *postgresStorage) GetRepositories(ctx context.Context, org string) ([]*domain.Repository, error) {
	query := `
		SELECT org, name, full_name, is_private, last_synced_at, created_at, updated_at
		FROM repositories
		WHERE org = $1
		ORDER BY name
	`
	rows, err := s.db.QueryContext(ctx, query, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []*domain.Repository
	for rows.Next() {
		var r domain.Repository
		var lastSyncedAt sql.NullTime

		err := rows.Scan(&r.Org, &r.Name, &r.FullName, &r.IsPrivate, &lastSyncedAt, &r.CreatedAt, &r.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if lastSyncedAt.Valid {
			r.LastSyncedAt = &lastSyncedAt.Time
		}

		repos = append(repos, &r)
	}

	return repos, nil
}

// SaveMember saves a member
func (s *postgresStorage) SaveMember(ctx context.Context, member *domain.Member) error {
	query := `
		INSERT INTO members (org, username, display_name, last_synced_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (org, username) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			last_synced_at = EXCLUDED.last_synced_at,
			updated_at = EXCLUDED.updated_at
	`
	_, err := s.db.ExecContext(ctx, query,
		member.Org,
		member.Username,
		member.DisplayName,
		member.LastSyncedAt,
		member.CreatedAt,
		member.UpdatedAt,
	)
	return err
}

// GetMembers retrieves all members for an organization
func (s *postgresStorage) GetMembers(ctx context.Context, org string) ([]*domain.Member, error) {
	query := `
		SELECT org, username, display_name, last_synced_at, created_at, updated_at
		FROM members
		WHERE org = $1
		ORDER BY username
	`
	rows, err := s.db.QueryContext(ctx, query, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*domain.Member
	for rows.Next() {
		var m domain.Member
		var displayName sql.NullString
		var lastSyncedAt sql.NullTime

		err := rows.Scan(&m.Org, &m.Username, &displayName, &lastSyncedAt, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if displayName.Valid {
			m.DisplayName = displayName.String
		}
		if lastSyncedAt.Valid {
			m.LastSyncedAt = &lastSyncedAt.Time
		}

		members = append(members, &m)
	}

	return members, nil
}

// GetMembersWithMetrics retrieves all members with their metrics
func (s *postgresStorage) GetMembersWithMetrics(ctx context.Context, org string, timeRange domain.TimeRange) ([]*domain.MemberMetrics, error) {
	query := `
		SELECT DISTINCT member FROM events
		WHERE org = $1 AND timestamp >= $2 AND timestamp <= $3
		ORDER BY member
	`
	rows, err := s.db.QueryContext(ctx, query, org, timeRange.Start, timeRange.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memberNames []string
	for rows.Next() {
		var member string
		if err := rows.Scan(&member); err != nil {
			return nil, err
		}
		memberNames = append(memberNames, member)
	}

	var metrics []*domain.MemberMetrics
	for _, member := range memberNames {
		m, err := s.GetMetricsByMember(ctx, org, member, timeRange)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}

	return metrics, nil
}

// GetReposWithMetrics retrieves all repos with their metrics
func (s *postgresStorage) GetReposWithMetrics(ctx context.Context, org string, timeRange domain.TimeRange) ([]*domain.RepoMetrics, error) {
	query := `
		SELECT DISTINCT repo FROM events
		WHERE org = $1 AND timestamp >= $2 AND timestamp <= $3
		ORDER BY repo
	`
	rows, err := s.db.QueryContext(ctx, query, org, timeRange.Start, timeRange.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repoNames []string
	for rows.Next() {
		var repo string
		if err := rows.Scan(&repo); err != nil {
			return nil, err
		}
		repoNames = append(repoNames, repo)
	}

	var metrics []*domain.RepoMetrics
	for _, repo := range repoNames {
		m, err := s.GetMetricsByRepo(ctx, org, repo, timeRange)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}

	return metrics, nil
}

// Close closes the database connection
func (s *postgresStorage) Close() error {
	return s.db.Close()
}
