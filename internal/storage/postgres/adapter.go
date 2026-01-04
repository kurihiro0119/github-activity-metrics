package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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

	CREATE TABLE IF NOT EXISTS batch_repositories (
		batch_id TEXT NOT NULL,
		owner TEXT NOT NULL,
		repo_name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		events_count INTEGER NOT NULL DEFAULT 0,
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		error TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (batch_id, owner, repo_name),
		FOREIGN KEY (batch_id) REFERENCES collection_batches(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_batch_repositories_batch_id ON batch_repositories(batch_id);
	CREATE INDEX IF NOT EXISTS idx_batch_repositories_status ON batch_repositories(status);
	CREATE INDEX IF NOT EXISTS idx_batch_repositories_owner_repo ON batch_repositories(owner, repo_name);
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

// GetRepoMembersWithMetrics retrieves all members with their metrics for a specific repository
func (s *postgresStorage) GetRepoMembersWithMetrics(ctx context.Context, org, repo string, timeRange domain.TimeRange) ([]*domain.MemberMetrics, error) {
	query := `
		SELECT DISTINCT member FROM events
		WHERE owner = $1 AND repo = $2 AND timestamp >= $3 AND timestamp <= $4
		ORDER BY member
	`
	rows, err := s.db.QueryContext(ctx, query, org, repo, timeRange.Start, timeRange.End)
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
		// Get metrics for this member in this specific repo
		memberMetrics := &domain.MemberMetrics{
			Member:    member,
			TimeRange: timeRange,
		}

		// Get commits count for this repo
		err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM events 
			WHERE owner = $1 AND repo = $2 AND member = $3 AND type = 'commit' AND timestamp >= $4 AND timestamp <= $5
		`, org, repo, member, timeRange.Start, timeRange.End).Scan(&memberMetrics.Commits)
		if err != nil {
			return nil, err
		}

		// Get PRs count for this repo
		err = s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM events 
			WHERE owner = $1 AND repo = $2 AND member = $3 AND type = 'pull_request' AND timestamp >= $4 AND timestamp <= $5
		`, org, repo, member, timeRange.Start, timeRange.End).Scan(&memberMetrics.PRs)
		if err != nil {
			return nil, err
		}

		// Get deploys count for this repo
		err = s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM events 
			WHERE owner = $1 AND repo = $2 AND member = $3 AND type = 'deploy' AND timestamp >= $4 AND timestamp <= $5
		`, org, repo, member, timeRange.Start, timeRange.End).Scan(&memberMetrics.Deploys)
		if err != nil {
			return nil, err
		}

		// Get additions and deletions from commit events using JSONB for this repo
		err = s.db.QueryRowContext(ctx, `
			SELECT 
				COALESCE(SUM((data->>'additions')::int), 0),
				COALESCE(SUM((data->>'deletions')::int), 0)
			FROM events 
			WHERE owner = $1 AND repo = $2 AND member = $3 AND type = 'commit' AND timestamp >= $4 AND timestamp <= $5
		`, org, repo, member, timeRange.Start, timeRange.End).Scan(&memberMetrics.Additions, &memberMetrics.Deletions)
		if err != nil {
			return nil, err
		}

		metrics = append(metrics, memberMetrics)
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

// GetMemberRanking retrieves member rankings
func (s *postgresStorage) GetMemberRanking(ctx context.Context, org string, rankingType domain.RankingType, timeRange domain.TimeRange, limit int) ([]*domain.MemberRanking, error) {
	if limit <= 0 {
		limit = 10
	}

	var query string
	switch rankingType {
	case domain.RankingTypeCommits:
		query = `
			SELECT member,
				COUNT(*) as commits,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'commit' THEN COALESCE((data->>'additions')::int, 0) ELSE 0 END) as additions,
				SUM(CASE WHEN type = 'commit' THEN COALESCE((data->>'deletions')::int, 0) ELSE 0 END) as deletions,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = $1 AND timestamp >= $2 AND timestamp <= $3
			GROUP BY member
			ORDER BY commits DESC
			LIMIT $4
		`
	case domain.RankingTypePRs:
		query = `
			SELECT member,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as prs,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'commit' THEN COALESCE((data->>'additions')::int, 0) ELSE 0 END) as additions,
				SUM(CASE WHEN type = 'commit' THEN COALESCE((data->>'deletions')::int, 0) ELSE 0 END) as deletions,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = $1 AND timestamp >= $2 AND timestamp <= $3
			GROUP BY member
			ORDER BY prs DESC
			LIMIT $4
		`
	case domain.RankingTypeCodeChanges:
		query = `
			SELECT member,
				SUM(CASE WHEN type = 'commit' THEN COALESCE((data->>'additions')::int, 0) + COALESCE((data->>'deletions')::int, 0) ELSE 0 END) as code_changes,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'commit' THEN COALESCE((data->>'additions')::int, 0) ELSE 0 END) as additions,
				SUM(CASE WHEN type = 'commit' THEN COALESCE((data->>'deletions')::int, 0) ELSE 0 END) as deletions,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = $1 AND timestamp >= $2 AND timestamp <= $3
			GROUP BY member
			ORDER BY code_changes DESC
			LIMIT $4
		`
	case domain.RankingTypeDeploys:
		query = `
			SELECT member,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploys,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'commit' THEN COALESCE((data->>'additions')::int, 0) ELSE 0 END) as additions,
				SUM(CASE WHEN type = 'commit' THEN COALESCE((data->>'deletions')::int, 0) ELSE 0 END) as deletions,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = $1 AND timestamp >= $2 AND timestamp <= $3
			GROUP BY member
			ORDER BY deploys DESC
			LIMIT $4
		`
	default:
		return nil, fmt.Errorf("unknown ranking type: %s", rankingType)
	}

	rows, err := s.db.QueryContext(ctx, query, org, timeRange.Start, timeRange.End, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rankings []*domain.MemberRanking
	rank := 1
	for rows.Next() {
		var r domain.MemberRanking
		var commitCount, prCount, deployCount sql.NullInt64
		var additions, deletions sql.NullInt64

		err := rows.Scan(&r.Member, &r.Value, &commitCount, &prCount, &additions, &deletions, &deployCount)
		if err != nil {
			return nil, err
		}

		r.Rank = rank
		if commitCount.Valid {
			r.Commits = commitCount.Int64
		}
		if prCount.Valid {
			r.PRs = prCount.Int64
		}
		if additions.Valid {
			r.Additions = additions.Int64
		}
		if deletions.Valid {
			r.Deletions = deletions.Int64
		}
		if deployCount.Valid {
			r.Deploys = deployCount.Int64
		}

		rankings = append(rankings, &r)
		rank++
	}

	return rankings, nil
}

// GetRepoRanking retrieves repository rankings
func (s *postgresStorage) GetRepoRanking(ctx context.Context, org string, rankingType domain.RankingType, timeRange domain.TimeRange, limit int) ([]*domain.RepoRanking, error) {
	if limit <= 0 {
		limit = 10
	}

	var query string
	switch rankingType {
	case domain.RankingTypeCommits:
		query = `
			SELECT repo,
				COUNT(*) as commits,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = $1 AND timestamp >= $2 AND timestamp <= $3
			GROUP BY repo
			ORDER BY commits DESC
			LIMIT $4
		`
	case domain.RankingTypePRs:
		query = `
			SELECT repo,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as prs,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = $1 AND timestamp >= $2 AND timestamp <= $3
			GROUP BY repo
			ORDER BY prs DESC
			LIMIT $4
		`
	case domain.RankingTypeDeploys:
		query = `
			SELECT repo,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploys,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = $1 AND timestamp >= $2 AND timestamp <= $3
			GROUP BY repo
			ORDER BY deploys DESC
			LIMIT $4
		`
	case domain.RankingTypeCodeChanges:
		query = `
			SELECT repo,
				SUM(CASE WHEN type = 'commit' THEN COALESCE((data->>'additions')::int, 0) + COALESCE((data->>'deletions')::int, 0) ELSE 0 END) as code_changes,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = $1 AND timestamp >= $2 AND timestamp <= $3
			GROUP BY repo
			ORDER BY code_changes DESC
			LIMIT $4
		`
	default:
		return nil, fmt.Errorf("unknown ranking type: %s", rankingType)
	}

	rows, err := s.db.QueryContext(ctx, query, org, timeRange.Start, timeRange.End, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rankings []*domain.RepoRanking
	rank := 1
	for rows.Next() {
		var r domain.RepoRanking
		var commitCount, prCount, deployCount sql.NullInt64

		err := rows.Scan(&r.Repo, &r.Value, &commitCount, &prCount, &deployCount)
		if err != nil {
			return nil, err
		}

		r.Rank = rank
		if commitCount.Valid {
			r.Commits = commitCount.Int64
		}
		if prCount.Valid {
			r.PRs = prCount.Int64
		}
		if deployCount.Valid {
			r.Deploys = deployCount.Int64
		}

		rankings = append(rankings, &r)
		rank++
	}

	return rankings, nil
}

// CreateOrGetBatch creates a new batch or returns existing one with same parameters
func (s *postgresStorage) CreateOrGetBatch(ctx context.Context, batch *domain.CollectionBatch) (*domain.CollectionBatch, error) {
	// Check if batch with same parameters exists
	var existingID, existingStatus string
	var existingCreatedAt, existingUpdatedAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT id, status, created_at, updated_at
		FROM collection_batches
		WHERE mode = $1 AND owner = $2 AND start_date = $3 AND end_date = $4
		ORDER BY created_at DESC
		LIMIT 1
	`, batch.Mode, batch.Owner, batch.StartDate, batch.EndDate).Scan(&existingID, &existingStatus, &existingCreatedAt, &existingUpdatedAt)

	if err == nil {
		// Existing batch found
		batch.ID = existingID
		batch.Status = existingStatus
		batch.CreatedAt = existingCreatedAt
		batch.UpdatedAt = existingUpdatedAt
		return batch, nil
	}

	// Create new batch
	if batch.ID == "" {
		batch.ID = fmt.Sprintf("%s-%s-%d-%d", batch.Mode, batch.Owner, batch.StartDate.Unix(), batch.EndDate.Unix())
	}
	now := time.Now()
	if batch.CreatedAt.IsZero() {
		batch.CreatedAt = now
	}
	batch.UpdatedAt = now

	query := `
		INSERT INTO collection_batches (id, mode, owner, start_date, end_date, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			updated_at = EXCLUDED.updated_at
		RETURNING id, status, created_at, updated_at
	`
	err = s.db.QueryRowContext(ctx, query,
		batch.ID, batch.Mode, batch.Owner, batch.StartDate, batch.EndDate, batch.Status, batch.CreatedAt, batch.UpdatedAt).Scan(
		&batch.ID, &batch.Status, &batch.CreatedAt, &batch.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return batch, nil
}

// GetBatch retrieves a batch by ID
func (s *postgresStorage) GetBatch(ctx context.Context, batchID string) (*domain.CollectionBatch, error) {
	var batch domain.CollectionBatch
	err := s.db.QueryRowContext(ctx, `
		SELECT id, mode, owner, start_date, end_date, status, created_at, updated_at
		FROM collection_batches
		WHERE id = $1
	`, batchID).Scan(
		&batch.ID, &batch.Mode, &batch.Owner, &batch.StartDate, &batch.EndDate,
		&batch.Status, &batch.CreatedAt, &batch.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &batch, nil
}

// UpdateBatchStatus updates the status of a batch
func (s *postgresStorage) UpdateBatchStatus(ctx context.Context, batchID string, status string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE collection_batches
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`, status, batchID)
	return err
}

// GetCompletedReposForBatch returns a map of completed repository names for a batch
func (s *postgresStorage) GetCompletedReposForBatch(ctx context.Context, batchID string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT repo_name
		FROM batch_repositories
		WHERE batch_id = $1 AND status = 'completed'
	`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	completed := make(map[string]bool)
	for rows.Next() {
		var repoName string
		if err := rows.Scan(&repoName); err != nil {
			return nil, err
		}
		completed[repoName] = true
	}
	return completed, nil
}

// SaveBatchRepository saves or updates a batch repository status
func (s *postgresStorage) SaveBatchRepository(ctx context.Context, batchRepo *domain.BatchRepository) error {
	query := `
		INSERT INTO batch_repositories 
		(batch_id, owner, repo_name, status, events_count, started_at, completed_at, error, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (batch_id, owner, repo_name) DO UPDATE SET
			status = EXCLUDED.status,
			events_count = EXCLUDED.events_count,
			started_at = COALESCE(EXCLUDED.started_at, batch_repositories.started_at),
			completed_at = COALESCE(EXCLUDED.completed_at, batch_repositories.completed_at),
			error = EXCLUDED.error,
			updated_at = EXCLUDED.updated_at
	`
	_, err := s.db.ExecContext(ctx, query,
		batchRepo.BatchID, batchRepo.Owner, batchRepo.RepoName, batchRepo.Status,
		batchRepo.EventsCount, batchRepo.StartedAt, batchRepo.CompletedAt, batchRepo.Error,
		batchRepo.CreatedAt, batchRepo.UpdatedAt)
	return err
}

// UpdateBatchRepositoryStatus updates the status of a repository in a batch
func (s *postgresStorage) UpdateBatchRepositoryStatus(ctx context.Context, batchID, owner, repoName, status string, eventsCount int, err error) error {
	now := time.Now()
	var startedAt, completedAt *time.Time
	var errorMsg string

	if status == "processing" {
		startedAt = &now
	} else if status == "completed" {
		completedAt = &now
	} else if status == "failed" && err != nil {
		errorMsg = err.Error()
	}

	query := `
		UPDATE batch_repositories
		SET status = $1, events_count = $2, 
		    started_at = COALESCE($3, started_at), 
		    completed_at = COALESCE($4, completed_at), 
		    error = $5, updated_at = CURRENT_TIMESTAMP
		WHERE batch_id = $6 AND owner = $7 AND repo_name = $8
	`
	result, updateErr := s.db.ExecContext(ctx, query, status, eventsCount, startedAt, completedAt, errorMsg, batchID, owner, repoName)
	if updateErr != nil {
		return updateErr
	}
	
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// If update fails, try insert
		batchRepo := &domain.BatchRepository{
			BatchID:     batchID,
			Owner:       owner,
			RepoName:    repoName,
			Status:      status,
			EventsCount: eventsCount,
			StartedAt:   startedAt,
			CompletedAt: completedAt,
			Error:       errorMsg,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		return s.SaveBatchRepository(ctx, batchRepo)
	}
	return nil
}

// GetOrgTimeSeries retrieves time series data for an organization
func (s *postgresStorage) GetOrgTimeSeries(ctx context.Context, org string, timeRange domain.TimeRange) (*domain.DetailedTimeSeriesData, error) {
	return s.getTimeSeries(ctx, org, "", "", timeRange)
}

// GetRepoTimeSeries retrieves time series data for a repository
func (s *postgresStorage) GetRepoTimeSeries(ctx context.Context, org, repo string, timeRange domain.TimeRange) (*domain.DetailedTimeSeriesData, error) {
	return s.getTimeSeries(ctx, org, repo, "", timeRange)
}

// GetMemberTimeSeries retrieves time series data for a member
func (s *postgresStorage) GetMemberTimeSeries(ctx context.Context, org, member string, timeRange domain.TimeRange) (*domain.DetailedTimeSeriesData, error) {
	return s.getTimeSeries(ctx, org, "", member, timeRange)
}

// getTimeSeries is a helper function to get time series data
func (s *postgresStorage) getTimeSeries(ctx context.Context, org, repo, member string, timeRange domain.TimeRange) (*domain.DetailedTimeSeriesData, error) {
	// Build query based on filters
	query := `
		SELECT 
			DATE_TRUNC($1, timestamp) as period,
			SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END)::BIGINT as commits,
			SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END)::BIGINT as prs,
			SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END)::BIGINT as deploys,
			SUM(CASE WHEN type = 'commit' THEN CAST(data->>'additions' AS INTEGER) ELSE 0 END)::BIGINT as additions,
			SUM(CASE WHEN type = 'commit' THEN CAST(data->>'deletions' AS INTEGER) ELSE 0 END)::BIGINT as deletions
		FROM events
		WHERE owner = $2 AND timestamp >= $3 AND timestamp <= $4
	`
	args := []interface{}{getPostgresDateTrunc(timeRange.Granularity), org, timeRange.Start, timeRange.End}
	argIndex := 5

	if repo != "" {
		query += fmt.Sprintf(" AND repo = $%d", argIndex)
		args = append(args, repo)
		argIndex++
	}

	if member != "" {
		query += fmt.Sprintf(" AND member = $%d", argIndex)
		args = append(args, member)
		argIndex++
	}

	query += fmt.Sprintf(" GROUP BY period ORDER BY period")

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dataPoints []domain.DetailedTimeSeriesMetric
	for rows.Next() {
		var timestamp time.Time
		var commits, prs, additions, deletions, deploys int64

		if err := rows.Scan(&timestamp, &commits, &prs, &deploys, &additions, &deletions); err != nil {
			return nil, err
		}

		dataPoints = append(dataPoints, domain.DetailedTimeSeriesMetric{
			Timestamp: timestamp,
			Commits:   commits,
			PRs:       prs,
			Additions: additions,
			Deletions: deletions,
			Deploys:   deploys,
		})
	}

	// Fill in missing periods
	filledDataPoints := s.fillTimeSeriesGaps(dataPoints, timeRange)

	return &domain.DetailedTimeSeriesData{
		Granularity: timeRange.Granularity,
		DataPoints:  filledDataPoints,
	}, nil
}

// getPostgresDateTrunc returns the PostgreSQL date_trunc unit
func getPostgresDateTrunc(granularity string) string {
	switch granularity {
	case "day":
		return "day"
	case "month":
		return "month"
	default:
		return "day"
	}
}

// fillTimeSeriesGaps fills in missing periods with zero values
func (s *postgresStorage) fillTimeSeriesGaps(dataPoints []domain.DetailedTimeSeriesMetric, timeRange domain.TimeRange) []domain.DetailedTimeSeriesMetric {
	if len(dataPoints) == 0 {
		return dataPoints
	}

	// Create a map of existing timestamps
	existingMap := make(map[time.Time]domain.DetailedTimeSeriesMetric)
	for _, dp := range dataPoints {
		existingMap[truncateTimeForGranularity(dp.Timestamp, timeRange.Granularity)] = dp
	}

	// Generate all periods in the range
	var filled []domain.DetailedTimeSeriesMetric
	current := truncateTimeForGranularity(timeRange.Start, timeRange.Granularity)
	end := truncateTimeForGranularity(timeRange.End, timeRange.Granularity)

	for !current.After(end) {
		if dp, exists := existingMap[current]; exists {
			filled = append(filled, dp)
		} else {
			filled = append(filled, domain.DetailedTimeSeriesMetric{
				Timestamp: current,
				Commits:   0,
				PRs:       0,
				Additions: 0,
				Deletions: 0,
				Deploys:   0,
			})
		}
		current = getNextPeriodForGranularity(current, timeRange.Granularity)
	}

	return filled
}

// truncateTimeForGranularity truncates a time to the start of the period
func truncateTimeForGranularity(t time.Time, granularity string) time.Time {
	switch granularity {
	case "day":
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
}

// getNextPeriodForGranularity returns the start of the next period
func getNextPeriodForGranularity(t time.Time, granularity string) time.Time {
	switch granularity {
	case "day":
		return t.AddDate(0, 0, 1)
	case "month":
		return t.AddDate(0, 1, 0)
	default:
		return t.AddDate(0, 0, 1)
	}
}

// Close closes the database connection
func (s *postgresStorage) Close() error {
	return s.db.Close()
}
