package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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
	// Check if migration is needed (check if old 'org' column exists)
	var columnExists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'events' AND column_name = 'org'
		)
	`).Scan(&columnExists)

	if err == nil && columnExists {
		// Old schema exists, need to migrate
		if err := s.migrateFromOrgToOwner(ctx); err != nil {
			return fmt.Errorf("failed to migrate from org to owner: %w", err)
		}
	}

	// Create new schema (or ensure it exists after migration)
	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		owner TEXT NOT NULL,
		owner_type TEXT NOT NULL DEFAULT 'organization',
		repo TEXT NOT NULL,
		member TEXT NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		data JSONB NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_events_owner_repo ON events(owner, repo);
	CREATE INDEX IF NOT EXISTS idx_events_member ON events(member);
	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
	CREATE INDEX IF NOT EXISTS idx_events_owner_type_timestamp ON events(owner, type, timestamp);
	CREATE INDEX IF NOT EXISTS idx_events_owner_type ON events(owner_type);

	CREATE TABLE IF NOT EXISTS repositories (
		owner TEXT NOT NULL,
		owner_type TEXT NOT NULL DEFAULT 'organization',
		name TEXT NOT NULL,
		full_name TEXT NOT NULL,
		is_private BOOLEAN NOT NULL,
		last_synced_at TIMESTAMP,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (owner, name)
	);

	CREATE INDEX IF NOT EXISTS idx_repositories_owner ON repositories(owner);
	CREATE INDEX IF NOT EXISTS idx_repositories_owner_type ON repositories(owner_type);

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
	`

	_, err = s.db.ExecContext(ctx, schema)
	return err
}

// migrateFromOrgToOwner migrates existing tables from 'org' to 'owner' with 'owner_type'
func (s *postgresStorage) migrateFromOrgToOwner(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Migrate events table
	_, err = tx.ExecContext(ctx, `ALTER TABLE events RENAME COLUMN org TO owner`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS owner_type TEXT NOT NULL DEFAULT 'organization'`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `UPDATE events SET owner_type = 'organization' WHERE owner_type IS NULL`)
	if err != nil {
		return err
	}

	// Drop old indexes and create new ones
	_, err = tx.ExecContext(ctx, `DROP INDEX IF EXISTS idx_events_org_repo`)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DROP INDEX IF EXISTS idx_events_org_type_timestamp`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_events_owner_repo ON events(owner, repo)`)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_events_owner_type_timestamp ON events(owner, type, timestamp)`)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_events_owner_type ON events(owner_type)`)
	if err != nil {
		return err
	}

	// Migrate repositories table
	_, err = tx.ExecContext(ctx, `ALTER TABLE repositories RENAME COLUMN org TO owner`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `ALTER TABLE repositories ADD COLUMN IF NOT EXISTS owner_type TEXT NOT NULL DEFAULT 'organization'`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `UPDATE repositories SET owner_type = 'organization' WHERE owner_type IS NULL`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `DROP INDEX IF EXISTS idx_repositories_org`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_repositories_owner ON repositories(owner)`)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_repositories_owner_type ON repositories(owner_type)`)
	if err != nil {
		return err
	}

	// Migrate members table
	_, err = tx.ExecContext(ctx, `ALTER TABLE members RENAME COLUMN org TO owner`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `ALTER TABLE members ADD COLUMN IF NOT EXISTS owner_type TEXT NOT NULL DEFAULT 'organization'`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `UPDATE members SET owner_type = 'organization' WHERE owner_type IS NULL`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `DROP INDEX IF EXISTS idx_members_org`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_members_owner ON members(owner)`)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_members_owner_type ON members(owner_type)`)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// SaveRawEvent saves a single raw event
func (s *postgresStorage) SaveRawEvent(ctx context.Context, event *domain.Event) error {
	dataJSON, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}

	ownerType := event.OwnerType
	if ownerType == "" {
		ownerType = "organization" // default
	}

	query := `
		INSERT INTO events (id, type, owner, owner_type, repo, member, timestamp, data, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			type = EXCLUDED.type,
			owner = EXCLUDED.owner,
			owner_type = EXCLUDED.owner_type,
			repo = EXCLUDED.repo,
			member = EXCLUDED.member,
			timestamp = EXCLUDED.timestamp,
			data = EXCLUDED.data
	`
	_, err = s.db.ExecContext(ctx, query,
		event.ID,
		string(event.Type),
		event.Org, // Org field maps to owner column
		ownerType,
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
		INSERT INTO events (id, type, owner, owner_type, repo, member, timestamp, data, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			type = EXCLUDED.type,
			owner = EXCLUDED.owner,
			owner_type = EXCLUDED.owner_type,
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

		ownerType := event.OwnerType
		if ownerType == "" {
			ownerType = "organization" // default
		}

		_, err = stmt.ExecContext(ctx,
			event.ID,
			string(event.Type),
			event.Org, // Org field maps to owner column
			ownerType,
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

// GetMetricsByOrg retrieves organization-level metrics
func (s *postgresStorage) GetMetricsByOrg(ctx context.Context, org string, timeRange domain.TimeRange) (*domain.OrgMetrics, error) {
	metrics := &domain.OrgMetrics{
		Org:       org,
		TimeRange: timeRange,
	}

	// Get total repos
	var totalRepos int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM repositories WHERE owner = $1`, org).Scan(&totalRepos)
	if err != nil {
		return nil, err
	}
	metrics.TotalRepos = totalRepos

	// Get total members
	var totalMembers int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM members WHERE owner = $1`, org).Scan(&totalMembers)
	if err != nil {
		return nil, err
	}
	metrics.TotalMembers = totalMembers

	// Get commits count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = $1 AND type = 'commit' AND timestamp >= $2 AND timestamp <= $3
	`, org, timeRange.Start, timeRange.End).Scan(&metrics.Commits)
	if err != nil {
		return nil, err
	}

	// Get PRs count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = $1 AND type = 'pull_request' AND timestamp >= $2 AND timestamp <= $3
	`, org, timeRange.Start, timeRange.End).Scan(&metrics.PRs)
	if err != nil {
		return nil, err
	}

	// Get deploys count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = $1 AND type = 'deploy' AND timestamp >= $2 AND timestamp <= $3
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
		WHERE owner = $1 AND type = 'commit' AND timestamp >= $2 AND timestamp <= $3
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
		WHERE owner = $1 AND member = $2 AND type = 'commit' AND timestamp >= $3 AND timestamp <= $4
	`, org, member, timeRange.Start, timeRange.End).Scan(&metrics.Commits)
	if err != nil {
		return nil, err
	}

	// Get PRs count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = $1 AND member = $2 AND type = 'pull_request' AND timestamp >= $3 AND timestamp <= $4
	`, org, member, timeRange.Start, timeRange.End).Scan(&metrics.PRs)
	if err != nil {
		return nil, err
	}

	// Get deploys count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = $1 AND member = $2 AND type = 'deploy' AND timestamp >= $3 AND timestamp <= $4
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
		WHERE owner = $1 AND member = $2 AND type = 'commit' AND timestamp >= $3 AND timestamp <= $4
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
		WHERE owner = $1 AND repo = $2 AND type = 'commit' AND timestamp >= $3 AND timestamp <= $4
	`, org, repo, timeRange.Start, timeRange.End).Scan(&metrics.Commits)
	if err != nil {
		return nil, err
	}

	// Get PRs count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = $1 AND repo = $2 AND type = 'pull_request' AND timestamp >= $3 AND timestamp <= $4
	`, org, repo, timeRange.Start, timeRange.End).Scan(&metrics.PRs)
	if err != nil {
		return nil, err
	}

	// Get deploys count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = $1 AND repo = $2 AND type = 'deploy' AND timestamp >= $3 AND timestamp <= $4
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
		WHERE owner = $1 AND repo = $2 AND type = 'commit' AND timestamp >= $3 AND timestamp <= $4
	`, org, repo, timeRange.Start, timeRange.End).Scan(&metrics.Additions, &metrics.Deletions)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// GetEvents retrieves events for re-aggregation
func (s *postgresStorage) GetEvents(ctx context.Context, org string, eventType domain.EventType, timeRange domain.TimeRange) ([]*domain.Event, error) {
	query := `
		SELECT id, type, owner, owner_type, repo, member, timestamp, data, created_at
		FROM events
		WHERE owner = $1 AND type = $2 AND timestamp >= $3 AND timestamp <= $4
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

		var ownerType string
		err := rows.Scan(&e.ID, &e.Type, &e.Org, &ownerType, &e.Repo, &e.Member, &e.Timestamp, &dataStr, &e.CreatedAt)
		e.OwnerType = ownerType
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
	ownerType := repo.OwnerType
	if ownerType == "" {
		ownerType = "organization" // default
	}
	query := `
		INSERT INTO repositories (owner, owner_type, name, full_name, is_private, last_synced_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (owner, name) DO UPDATE SET
			full_name = EXCLUDED.full_name,
			is_private = EXCLUDED.is_private,
			owner_type = EXCLUDED.owner_type,
			last_synced_at = EXCLUDED.last_synced_at,
			updated_at = EXCLUDED.updated_at
	`
	_, err := s.db.ExecContext(ctx, query,
		repo.Org, // Org field maps to owner column
		ownerType,
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
		SELECT owner, owner_type, name, full_name, is_private, last_synced_at, created_at, updated_at
		FROM repositories
		WHERE owner = $1
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

		err := rows.Scan(&r.Org, &r.OwnerType, &r.Name, &r.FullName, &r.IsPrivate, &lastSyncedAt, &r.CreatedAt, &r.UpdatedAt)
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
	ownerType := member.OwnerType
	if ownerType == "" {
		ownerType = "organization" // default
	}
	query := `
		INSERT INTO members (owner, owner_type, username, display_name, last_synced_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (owner, username) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			owner_type = EXCLUDED.owner_type,
			last_synced_at = EXCLUDED.last_synced_at,
			updated_at = EXCLUDED.updated_at
	`
	_, err := s.db.ExecContext(ctx, query,
		member.Org, // Org field maps to owner column
		ownerType,
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
		SELECT owner, owner_type, username, display_name, last_synced_at, created_at, updated_at
		FROM members
		WHERE owner = $1
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

		err := rows.Scan(&m.Org, &m.OwnerType, &m.Username, &displayName, &lastSyncedAt, &m.CreatedAt, &m.UpdatedAt)
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
		WHERE owner = $1 AND timestamp >= $2 AND timestamp <= $3
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
		WHERE owner = $1 AND timestamp >= $2 AND timestamp <= $3
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
