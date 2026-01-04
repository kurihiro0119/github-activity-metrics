package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/kurihiro0119/github-activity-metrics/internal/domain"
	"github.com/kurihiro0119/github-activity-metrics/internal/storage"
)

// sqliteStorage implements the Storage interface for SQLite
type sqliteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(dbPath string) (storage.Storage, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	s := &sqliteStorage{db: db}
	if err := s.Migrate(context.Background()); err != nil {
		return nil, err
	}

	return s, nil
}

// Migrate runs database migrations
func (s *sqliteStorage) Migrate(ctx context.Context) error {
	// Check if migration is needed (check if old 'org' column exists)
	var tableInfo string
	err := s.db.QueryRowContext(ctx, `
		SELECT sql FROM sqlite_master 
		WHERE type='table' AND name='events' AND sql LIKE '%org TEXT%'
	`).Scan(&tableInfo)
	
	if err == nil {
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
		data TEXT NOT NULL,
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
		is_private INTEGER NOT NULL,
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
func (s *sqliteStorage) migrateFromOrgToOwner(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Migrate events table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE events_new (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			owner TEXT NOT NULL,
			owner_type TEXT NOT NULL DEFAULT 'organization',
			repo TEXT NOT NULL,
			member TEXT NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			data TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO events_new (id, type, owner, owner_type, repo, member, timestamp, data, created_at)
		SELECT id, type, org, 'organization', repo, member, timestamp, data, created_at
		FROM events
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `DROP TABLE events`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `ALTER TABLE events_new RENAME TO events`)
	if err != nil {
		return err
	}

	// Create indexes for events
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX idx_events_owner_repo ON events(owner, repo);
		CREATE INDEX idx_events_member ON events(member);
		CREATE INDEX idx_events_timestamp ON events(timestamp);
		CREATE INDEX idx_events_type ON events(type);
		CREATE INDEX idx_events_owner_type_timestamp ON events(owner, type, timestamp);
		CREATE INDEX idx_events_owner_type ON events(owner_type);
	`)
	if err != nil {
		return err
	}

	// Migrate repositories table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE repositories_new (
			owner TEXT NOT NULL,
			owner_type TEXT NOT NULL DEFAULT 'organization',
			name TEXT NOT NULL,
			full_name TEXT NOT NULL,
			is_private INTEGER NOT NULL,
			last_synced_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (owner, name)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO repositories_new (owner, owner_type, name, full_name, is_private, last_synced_at, created_at, updated_at)
		SELECT org, 'organization', name, full_name, is_private, last_synced_at, created_at, updated_at
		FROM repositories
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `DROP TABLE repositories`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `ALTER TABLE repositories_new RENAME TO repositories`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX idx_repositories_owner ON repositories(owner);
		CREATE INDEX idx_repositories_owner_type ON repositories(owner_type);
	`)
	if err != nil {
		return err
	}

	// Migrate members table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE members_new (
			owner TEXT NOT NULL,
			owner_type TEXT NOT NULL DEFAULT 'organization',
			username TEXT NOT NULL,
			display_name TEXT,
			last_synced_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (owner, username)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO members_new (owner, owner_type, username, display_name, last_synced_at, created_at, updated_at)
		SELECT org, 'organization', username, display_name, last_synced_at, created_at, updated_at
		FROM members
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `DROP TABLE members`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `ALTER TABLE members_new RENAME TO members`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX idx_members_owner ON members(owner);
		CREATE INDEX idx_members_owner_type ON members(owner_type);
	`)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// SaveRawEvent saves a single raw event
func (s *sqliteStorage) SaveRawEvent(ctx context.Context, event *domain.Event) error {
	dataJSON, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}

	ownerType := event.OwnerType
	if ownerType == "" {
		ownerType = "organization" // default
	}

	query := `
		INSERT OR REPLACE INTO events (id, type, owner, owner_type, repo, member, timestamp, data, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
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
func (s *sqliteStorage) SaveRawEvents(ctx context.Context, events []*domain.Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO events (id, type, owner, owner_type, repo, member, timestamp, data, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
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
func (s *sqliteStorage) GetMetricsByOrg(ctx context.Context, org string, timeRange domain.TimeRange) (*domain.OrgMetrics, error) {
	metrics := &domain.OrgMetrics{
		Org:       org,
		TimeRange: timeRange,
	}

	// Get total repos
	var totalRepos int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM repositories WHERE owner = ?`, org).Scan(&totalRepos)
	if err != nil {
		return nil, err
	}
	metrics.TotalRepos = totalRepos

	// Get total members
	var totalMembers int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM members WHERE owner = ?`, org).Scan(&totalMembers)
	if err != nil {
		return nil, err
	}
	metrics.TotalMembers = totalMembers

	// Get commits count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = ? AND type = 'commit' AND timestamp >= ? AND timestamp <= ?
	`, org, timeRange.Start, timeRange.End).Scan(&metrics.Commits)
	if err != nil {
		return nil, err
	}

	// Get PRs count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = ? AND type = 'pull_request' AND timestamp >= ? AND timestamp <= ?
	`, org, timeRange.Start, timeRange.End).Scan(&metrics.PRs)
	if err != nil {
		return nil, err
	}

	// Get deploys count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = ? AND type = 'deploy' AND timestamp >= ? AND timestamp <= ?
	`, org, timeRange.Start, timeRange.End).Scan(&metrics.Deploys)
	if err != nil {
		return nil, err
	}

	// Get additions and deletions from commit events
	rows, err := s.db.QueryContext(ctx, `
		SELECT data FROM events 
		WHERE owner = ? AND type = 'commit' AND timestamp >= ? AND timestamp <= ?
	`, org, timeRange.Start, timeRange.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var dataStr string
		if err := rows.Scan(&dataStr); err != nil {
			return nil, err
		}
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
			continue
		}
		if additions, ok := data["additions"].(float64); ok {
			metrics.Additions += int64(additions)
		}
		if deletions, ok := data["deletions"].(float64); ok {
			metrics.Deletions += int64(deletions)
		}
	}

	return metrics, nil
}

// GetMetricsByMember retrieves member-level metrics
func (s *sqliteStorage) GetMetricsByMember(ctx context.Context, org, member string, timeRange domain.TimeRange) (*domain.MemberMetrics, error) {
	metrics := &domain.MemberMetrics{
		Member:    member,
		TimeRange: timeRange,
	}

	// Get commits count
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = ? AND member = ? AND type = 'commit' AND timestamp >= ? AND timestamp <= ?
	`, org, member, timeRange.Start, timeRange.End).Scan(&metrics.Commits)
	if err != nil {
		return nil, err
	}

	// Get PRs count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = ? AND member = ? AND type = 'pull_request' AND timestamp >= ? AND timestamp <= ?
	`, org, member, timeRange.Start, timeRange.End).Scan(&metrics.PRs)
	if err != nil {
		return nil, err
	}

	// Get deploys count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = ? AND member = ? AND type = 'deploy' AND timestamp >= ? AND timestamp <= ?
	`, org, member, timeRange.Start, timeRange.End).Scan(&metrics.Deploys)
	if err != nil {
		return nil, err
	}

	// Get additions and deletions from commit events
	rows, err := s.db.QueryContext(ctx, `
		SELECT data FROM events 
		WHERE owner = ? AND member = ? AND type = 'commit' AND timestamp >= ? AND timestamp <= ?
	`, org, member, timeRange.Start, timeRange.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var dataStr string
		if err := rows.Scan(&dataStr); err != nil {
			return nil, err
		}
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
			continue
		}
		if additions, ok := data["additions"].(float64); ok {
			metrics.Additions += int64(additions)
		}
		if deletions, ok := data["deletions"].(float64); ok {
			metrics.Deletions += int64(deletions)
		}
	}

	return metrics, nil
}

// GetMetricsByRepo retrieves repository-level metrics
func (s *sqliteStorage) GetMetricsByRepo(ctx context.Context, org, repo string, timeRange domain.TimeRange) (*domain.RepoMetrics, error) {
	metrics := &domain.RepoMetrics{
		Repo:      repo,
		TimeRange: timeRange,
	}

	// Get commits count
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = ? AND repo = ? AND type = 'commit' AND timestamp >= ? AND timestamp <= ?
	`, org, repo, timeRange.Start, timeRange.End).Scan(&metrics.Commits)
	if err != nil {
		return nil, err
	}

	// Get PRs count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = ? AND repo = ? AND type = 'pull_request' AND timestamp >= ? AND timestamp <= ?
	`, org, repo, timeRange.Start, timeRange.End).Scan(&metrics.PRs)
	if err != nil {
		return nil, err
	}

	// Get deploys count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events 
		WHERE owner = ? AND repo = ? AND type = 'deploy' AND timestamp >= ? AND timestamp <= ?
	`, org, repo, timeRange.Start, timeRange.End).Scan(&metrics.Deploys)
	if err != nil {
		return nil, err
	}

	// Get additions and deletions from commit events
	rows, err := s.db.QueryContext(ctx, `
		SELECT data FROM events 
		WHERE owner = ? AND repo = ? AND type = 'commit' AND timestamp >= ? AND timestamp <= ?
	`, org, repo, timeRange.Start, timeRange.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var dataStr string
		if err := rows.Scan(&dataStr); err != nil {
			return nil, err
		}
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
			continue
		}
		if additions, ok := data["additions"].(float64); ok {
			metrics.Additions += int64(additions)
		}
		if deletions, ok := data["deletions"].(float64); ok {
			metrics.Deletions += int64(deletions)
		}
	}

	return metrics, nil
}

// GetEvents retrieves events for re-aggregation
func (s *sqliteStorage) GetEvents(ctx context.Context, org string, eventType domain.EventType, timeRange domain.TimeRange) ([]*domain.Event, error) {
	query := `
		SELECT id, type, owner, owner_type, repo, member, timestamp, data, created_at
		FROM events
		WHERE owner = ? AND type = ? AND timestamp >= ? AND timestamp <= ?
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
func (s *sqliteStorage) SaveRepository(ctx context.Context, repo *domain.Repository) error {
	ownerType := repo.OwnerType
	if ownerType == "" {
		ownerType = "organization" // default
	}
	query := `
		INSERT OR REPLACE INTO repositories (owner, owner_type, name, full_name, is_private, last_synced_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	isPrivate := 0
	if repo.IsPrivate {
		isPrivate = 1
	}
	_, err := s.db.ExecContext(ctx, query,
		repo.Org, // Org field maps to owner column
		ownerType,
		repo.Name,
		repo.FullName,
		isPrivate,
		repo.LastSyncedAt,
		repo.CreatedAt,
		repo.UpdatedAt,
	)
	return err
}

// GetRepositories retrieves all repositories for an organization
func (s *sqliteStorage) GetRepositories(ctx context.Context, org string) ([]*domain.Repository, error) {
	query := `
		SELECT owner, owner_type, name, full_name, is_private, last_synced_at, created_at, updated_at
		FROM repositories
		WHERE owner = ?
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
		var isPrivate int
		var lastSyncedAt sql.NullTime

		err := rows.Scan(&r.Org, &r.OwnerType, &r.Name, &r.FullName, &isPrivate, &lastSyncedAt, &r.CreatedAt, &r.UpdatedAt)
		if err != nil {
			return nil, err
		}

		r.IsPrivate = isPrivate == 1
		if lastSyncedAt.Valid {
			r.LastSyncedAt = &lastSyncedAt.Time
		}

		repos = append(repos, &r)
	}

	return repos, nil
}

// SaveMember saves a member
func (s *sqliteStorage) SaveMember(ctx context.Context, member *domain.Member) error {
	ownerType := member.OwnerType
	if ownerType == "" {
		ownerType = "organization" // default
	}
	query := `
		INSERT OR REPLACE INTO members (owner, owner_type, username, display_name, last_synced_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
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
func (s *sqliteStorage) GetMembers(ctx context.Context, org string) ([]*domain.Member, error) {
	query := `
		SELECT owner, owner_type, username, display_name, last_synced_at, created_at, updated_at
		FROM members
		WHERE owner = ?
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
func (s *sqliteStorage) GetMembersWithMetrics(ctx context.Context, org string, timeRange domain.TimeRange) ([]*domain.MemberMetrics, error) {
	query := `
		SELECT DISTINCT member FROM events
		WHERE owner = ? AND timestamp >= ? AND timestamp <= ?
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
func (s *sqliteStorage) GetReposWithMetrics(ctx context.Context, org string, timeRange domain.TimeRange) ([]*domain.RepoMetrics, error) {
	query := `
		SELECT DISTINCT repo FROM events
		WHERE owner = ? AND timestamp >= ? AND timestamp <= ?
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
func (s *sqliteStorage) GetMemberRanking(ctx context.Context, org string, rankingType domain.RankingType, timeRange domain.TimeRange, limit int) ([]*domain.MemberRanking, error) {
	if limit <= 0 {
		limit = 10
	}

	var query string
	switch rankingType {
	case domain.RankingTypeCommits:
		query = `
			SELECT member,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commits,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'commit' THEN CAST(json_extract(data, '$.additions') AS INTEGER) ELSE 0 END) as additions,
				SUM(CASE WHEN type = 'commit' THEN CAST(json_extract(data, '$.deletions') AS INTEGER) ELSE 0 END) as deletions,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = ? AND timestamp >= ? AND timestamp <= ?
			GROUP BY member
			ORDER BY commits DESC
			LIMIT ?
		`
	case domain.RankingTypePRs:
		query = `
			SELECT member,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as prs,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'commit' THEN CAST(json_extract(data, '$.additions') AS INTEGER) ELSE 0 END) as additions,
				SUM(CASE WHEN type = 'commit' THEN CAST(json_extract(data, '$.deletions') AS INTEGER) ELSE 0 END) as deletions,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = ? AND timestamp >= ? AND timestamp <= ?
			GROUP BY member
			ORDER BY prs DESC
			LIMIT ?
		`
	case domain.RankingTypeCodeChanges:
		query = `
			SELECT member,
				SUM(CASE WHEN type = 'commit' THEN CAST(json_extract(data, '$.additions') AS INTEGER) + CAST(json_extract(data, '$.deletions') AS INTEGER) ELSE 0 END) as code_changes,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'commit' THEN CAST(json_extract(data, '$.additions') AS INTEGER) ELSE 0 END) as additions,
				SUM(CASE WHEN type = 'commit' THEN CAST(json_extract(data, '$.deletions') AS INTEGER) ELSE 0 END) as deletions,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = ? AND timestamp >= ? AND timestamp <= ?
			GROUP BY member
			ORDER BY code_changes DESC
			LIMIT ?
		`
	case domain.RankingTypeDeploys:
		query = `
			SELECT member,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploys,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'commit' THEN CAST(json_extract(data, '$.additions') AS INTEGER) ELSE 0 END) as additions,
				SUM(CASE WHEN type = 'commit' THEN CAST(json_extract(data, '$.deletions') AS INTEGER) ELSE 0 END) as deletions,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = ? AND timestamp >= ? AND timestamp <= ?
			GROUP BY member
			ORDER BY deploys DESC
			LIMIT ?
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
func (s *sqliteStorage) GetRepoRanking(ctx context.Context, org string, rankingType domain.RankingType, timeRange domain.TimeRange, limit int) ([]*domain.RepoRanking, error) {
	if limit <= 0 {
		limit = 10
	}

	var query string
	switch rankingType {
	case domain.RankingTypeCommits:
		query = `
			SELECT repo,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commits,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = ? AND timestamp >= ? AND timestamp <= ?
			GROUP BY repo
			ORDER BY commits DESC
			LIMIT ?
		`
	case domain.RankingTypePRs:
		query = `
			SELECT repo,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as prs,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = ? AND timestamp >= ? AND timestamp <= ?
			GROUP BY repo
			ORDER BY prs DESC
			LIMIT ?
		`
	case domain.RankingTypeDeploys:
		query = `
			SELECT repo,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploys,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = ? AND timestamp >= ? AND timestamp <= ?
			GROUP BY repo
			ORDER BY deploys DESC
			LIMIT ?
		`
	case domain.RankingTypeCodeChanges:
		// Code changes ranking for repos (sum of additions + deletions)
		query = `
			SELECT repo,
				SUM(CASE WHEN type = 'commit' THEN CAST(json_extract(data, '$.additions') AS INTEGER) + CAST(json_extract(data, '$.deletions') AS INTEGER) ELSE 0 END) as code_changes,
				SUM(CASE WHEN type = 'commit' THEN 1 ELSE 0 END) as commit_count,
				SUM(CASE WHEN type = 'pull_request' THEN 1 ELSE 0 END) as pr_count,
				SUM(CASE WHEN type = 'deploy' THEN 1 ELSE 0 END) as deploy_count
			FROM events
			WHERE owner = ? AND timestamp >= ? AND timestamp <= ?
			GROUP BY repo
			ORDER BY code_changes DESC
			LIMIT ?
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

// Close closes the database connection
func (s *sqliteStorage) Close() error {
	return s.db.Close()
}
