# 開発生産性可視化OSS 設計書

## 概要

本設計書は、GitHub Organization の開発活動データを収集・集計する API および CLI ツールを Go 言語で実装するための設計を定義する。

## アーキテクチャ概要

### 全体構成

```
┌─────────────────────────────────────────────────────────┐
│                    CLI (cobra)                          │
│  ┌──────────────────────────────────────────────────┐  │
│  │         API Client (HTTP Client)                  │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
                          │
                          │ HTTP
                          ▼
┌─────────────────────────────────────────────────────────┐
│              API Server (gin/chi)                       │
│  ┌──────────────────────────────────────────────────┐  │
│  │         Metrics Service                          │  │
│  │  ┌──────────────┐  ┌──────────────┐            │  │
│  │  │ Aggregator   │  │ Collector    │            │  │
│  │  └──────────────┘  └──────────────┘            │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
                          │
                          │ Storage Interface
                          ▼
┌─────────────────────────────────────────────────────────┐
│              Storage Layer                               │
│  ┌──────────────────┐        ┌──────────────────┐      │
│  │ SQLite Adapter   │        │ PostgreSQL Adapter│      │
│  └──────────────────┘        └──────────────────┘      │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
              ┌──────────────────┐
              │   Database        │
              └──────────────────┘
```

### レイヤ分離

1. **Presentation Layer**: CLI / API Server
2. **Application Layer**: Metrics Service, Aggregator, Collector
3. **Domain Layer**: エンティティ、ビジネスロジック
4. **Infrastructure Layer**: Storage, GitHub API Client

## ディレクトリ構成

```
github-activity-metrics/
├── cmd/
│   ├── api/              # API サーバーエントリーポイント
│   │   └── main.go
│   └── cli/              # CLI エントリーポイント
│       └── main.go
├── internal/
│   ├── api/              # API ハンドラー
│   │   ├── handler.go
│   │   ├── middleware.go
│   │   └── routes.go
│   ├── collector/        # GitHub API データ収集
│   │   ├── github.go
│   │   ├── collector.go
│   │   └── rate_limiter.go
│   ├── aggregator/       # データ集計ロジック
│   │   ├── aggregator.go
│   │   └── metrics.go
│   ├── domain/           # ドメインモデル
│   │   ├── event.go
│   │   ├── metric.go
│   │   └── repository.go
│   ├── storage/          # ストレージ抽象化
│   │   ├── interface.go
│   │   ├── sqlite/
│   │   │   ├── adapter.go
│   │   │   ├── migration.go
│   │   │   └── schema.sql
│   │   └── postgres/
│   │       ├── adapter.go
│   │       ├── migration.go
│   │       └── schema.sql
│   └── config/           # 設定管理
│       └── config.go
├── pkg/                  # 公開パッケージ（将来の拡張用）
│   └── client/           # API クライアントライブラリ
│       └── client.go
├── migrations/           # DB マイグレーション
│   ├── sqlite/
│   └── postgres/
├── docs/
│   ├── requirements/
│   └── api/              # API 仕様書
├── scripts/              # ユーティリティスクリプト
├── test/                 # テストデータ
├── .env.example
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 主要コンポーネント設計

### 1. Domain Layer

#### Event (生イベント)

```go
// internal/domain/event.go
package domain

import "time"

type EventType string

const (
	EventTypeCommit      EventType = "commit"
	EventTypePullRequest EventType = "pull_request"
	EventTypeDeploy      EventType = "deploy"
)

type Event struct {
	ID          string
	Type        EventType
	Org         string
	Repo        string
	Member      string
	Timestamp   time.Time
	Data        map[string]interface{} // 型安全な構造体に展開可能
	CreatedAt   time.Time
}

type CommitEvent struct {
	Event
	Sha         string
	Message     string
	Additions   int
	Deletions   int
	FilesChanged int
}

type PullRequestEvent struct {
	Event
	Number      int
	State       string // open, closed, merged
	Title       string
	MergedAt    *time.Time
}

type DeployEvent struct {
	Event
	Environment string
	Status      string
	WorkflowRunID string
}
```

#### Metric (集計メトリクス)

```go
// internal/domain/metric.go
package domain

import "time"

type MetricType string

const (
	MetricTypeCommit      MetricType = "commit"
	MetricTypePullRequest MetricType = "pull_request"
	MetricTypeCodeChange  MetricType = "code_change"
	MetricTypeDeploy      MetricType = "deploy"
)

type TimeRange struct {
	Start time.Time
	End   time.Time
	Granularity string // "day", "week", "month"
}

type Metric struct {
	ID          string
	Type        MetricType
	Org         string
	Repo        *string  // nil の場合は Organization 全体
	Member      *string  // nil の場合は Repository 全体
	Value       int64
	TimeRange   TimeRange
	Metadata    map[string]interface{}
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type MemberMetrics struct {
	Member      string
	Commits     int64
	PRs         int64
	Additions   int64
	Deletions   int64
	Deploys     int64
	TimeRange   TimeRange
}

type RepoMetrics struct {
	Repo        string
	Commits     int64
	PRs         int64
	Additions   int64
	Deletions   int64
	Deploys     int64
	TimeRange   TimeRange
}

type OrgMetrics struct {
	Org         string
	TotalRepos  int
	TotalMembers int
	Commits     int64
	PRs         int64
	Additions   int64
	Deletions   int64
	Deploys     int64
	TimeRange   TimeRange
}
```

### 2. Storage Interface

```go
// internal/storage/interface.go
package storage

import (
	"context"
	"time"
	"github-activity-metrics/internal/domain"
)

// Storage は永続化レイヤの抽象インターフェース
type Storage interface {
	// 生イベントの保存
	SaveRawEvent(ctx context.Context, event *domain.Event) error
	SaveRawEvents(ctx context.Context, events []*domain.Event) error
	
	// 集計メトリクスの保存
	SaveAggregatedMetric(ctx context.Context, metric *domain.Metric) error
	SaveAggregatedMetrics(ctx context.Context, metrics []*domain.Metric) error
	
	// メトリクス取得
	GetMetricsByOrg(ctx context.Context, org string, timeRange domain.TimeRange) (*domain.OrgMetrics, error)
	GetMetricsByMember(ctx context.Context, org, member string, timeRange domain.TimeRange) (*domain.MemberMetrics, error)
	GetMetricsByRepo(ctx context.Context, org, repo string, timeRange domain.TimeRange) (*domain.RepoMetrics, error)
	
	// 時系列メトリクス取得
	GetTimeSeriesMetrics(ctx context.Context, org string, metricType domain.MetricType, timeRange domain.TimeRange) ([]*domain.Metric, error)
	
	// イベント取得（再集計用）
	GetEvents(ctx context.Context, org string, eventType domain.EventType, timeRange domain.TimeRange) ([]*domain.Event, error)
	
	// マイグレーション
	Migrate(ctx context.Context) error
	
	// 接続管理
	Close() error
}
```

### 3. Collector (GitHub API データ収集)

```go
// internal/collector/collector.go
package collector

import (
	"context"
	"time"
	"github-activity-metrics/internal/domain"
)

type Collector interface {
	// Organization の全 Repository を取得
	GetRepositories(ctx context.Context, org string) ([]string, error)
	
	// Repository の commit を取得
	GetCommits(ctx context.Context, org, repo string, since, until time.Time) ([]*domain.CommitEvent, error)
	
	// Repository の Pull Request を取得
	GetPullRequests(ctx context.Context, org, repo string, since, until time.Time) ([]*domain.PullRequestEvent, error)
	
	// Repository のデプロイ情報を取得（GitHub Actions）
	GetDeploys(ctx context.Context, org, repo string, since, until time.Time) ([]*domain.DeployEvent, error)
	
	// Organization のメンバー一覧を取得
	GetMembers(ctx context.Context, org string) ([]string, error)
}

// githubCollector は GitHub API を使用した実装
type githubCollector struct {
	client      *github.Client
	rateLimiter *RateLimiter
}

// CollectOrganizationData は Organization 全体のデータを収集
func (c *githubCollector) CollectOrganizationData(
	ctx context.Context,
	org string,
	since, until time.Time,
	onProgress func(repo string, progress float64),
) ([]*domain.Event, error) {
	// 実装: 並列処理で各 Repository のデータを収集
	// goroutine + channel で実装
}
```

#### Rate Limiter

```go
// internal/collector/rate_limiter.go
package collector

import (
	"context"
	"time"
)

type RateLimiter interface {
	Wait(ctx context.Context) error
	CheckLimit() (remaining int, resetTime time.Time, err error)
}

// githubRateLimiter は GitHub API のレート制限を管理
type githubRateLimiter struct {
	// 実装: GitHub API のレート制限ヘッダーを監視
	// X-RateLimit-Remaining, X-RateLimit-Reset をチェック
}
```

### 4. Aggregator (データ集計)

```go
// internal/aggregator/aggregator.go
package aggregator

import (
	"context"
	"github-activity-metrics/internal/domain"
	"github-activity-metrics/internal/storage"
)

type Aggregator interface {
	// イベントからメトリクスを集計
	AggregateEvents(ctx context.Context, events []*domain.Event, timeRange domain.TimeRange) ([]*domain.Metric, error)
	
	// Organization メトリクスを集計
	AggregateOrgMetrics(ctx context.Context, org string, timeRange domain.TimeRange) (*domain.OrgMetrics, error)
	
	// Member メトリクスを集計
	AggregateMemberMetrics(ctx context.Context, org, member string, timeRange domain.TimeRange) (*domain.MemberMetrics, error)
	
	// Repository メトリクスを集計
	AggregateRepoMetrics(ctx context.Context, org, repo string, timeRange domain.TimeRange) (*domain.RepoMetrics, error)
}

type aggregator struct {
	storage storage.Storage
}

func (a *aggregator) AggregateEvents(
	ctx context.Context,
	events []*domain.Event,
	timeRange domain.TimeRange,
) ([]*domain.Metric, error) {
	// 実装: イベントを時系列・粒度別に集計
	// - 日次集計
	// - 週次集計
	// - 月次集計
}
```

### 5. API Server

```go
// internal/api/handler.go
package api

import (
	"net/http"
	"github-activity-metrics/internal/domain"
	"github-activity-metrics/internal/aggregator"
)

type Handler struct {
	aggregator aggregator.Aggregator
}

// GetOrgMetrics は Organization のメトリクスを返す
// GET /api/v1/orgs/{org}/metrics
func (h *Handler) GetOrgMetrics(w http.ResponseWriter, r *http.Request) {
	org := getPathParam(r, "org")
	timeRange := parseTimeRange(r)
	
	metrics, err := h.aggregator.AggregateOrgMetrics(r.Context(), org, timeRange)
	if err != nil {
		respondError(w, err)
		return
	}
	
	respondJSON(w, http.StatusOK, metrics)
}

// GetMemberMetrics は Member のメトリクスを返す
// GET /api/v1/orgs/{org}/members/{member}/metrics
func (h *Handler) GetMemberMetrics(w http.ResponseWriter, r *http.Request) {
	// 実装
}

// GetRepoMetrics は Repository のメトリクスを返す
// GET /api/v1/orgs/{org}/repos/{repo}/metrics
func (h *Handler) GetRepoMetrics(w http.ResponseWriter, r *http.Request) {
	// 実装
}

// GetTimeSeriesMetrics は時系列メトリクスを返す
// GET /api/v1/orgs/{org}/metrics/timeseries?type=commit&granularity=day
func (h *Handler) GetTimeSeriesMetrics(w http.ResponseWriter, r *http.Request) {
	// 実装
}
```

#### API Routes

```go
// internal/api/routes.go
package api

import (
	"github.com/gin-gonic/gin" // または chi
)

func SetupRoutes(handler *Handler) *gin.Engine {
	router := gin.Default()
	
	v1 := router.Group("/api/v1")
	{
		orgs := v1.Group("/orgs/:org")
		{
			orgs.GET("/metrics", handler.GetOrgMetrics)
			orgs.GET("/metrics/timeseries", handler.GetTimeSeriesMetrics)
			
			members := orgs.Group("/members")
			{
				members.GET("/:member/metrics", handler.GetMemberMetrics)
			}
			
			repos := orgs.Group("/repos")
			{
				repos.GET("/:repo/metrics", handler.GetRepoMetrics)
			}
		}
	}
	
	return router
}
```

### 6. CLI

```go
// cmd/cli/main.go
package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "github-metrics",
	Short: "GitHub activity metrics tool",
}

var collectCmd = &cobra.Command{
	Use:   "collect [org]",
	Short: "Collect data from GitHub",
	RunE: func(cmd *cobra.Command, args []string) error {
		org := args[0]
		// Collector を実行してデータを収集・保存
	},
}

var showCmd = &cobra.Command{
	Use:   "show [org]",
	Short: "Show metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		// API を呼び出してメトリクスを表示
	},
}

var showMemberCmd = &cobra.Command{
	Use:   "member [org] [member]",
	Short: "Show member metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		// メンバー別メトリクスを表示
	},
}

func init() {
	rootCmd.AddCommand(collectCmd)
	rootCmd.AddCommand(showCmd)
	showCmd.AddCommand(showMemberCmd)
}
```

## データモデル設計

### SQLite / PostgreSQL 共通スキーマ

#### events テーブル（生イベント）

```sql
CREATE TABLE events (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,  -- 'commit', 'pull_request', 'deploy'
    org TEXT NOT NULL,
    repo TEXT NOT NULL,
    member TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    data JSONB NOT NULL,  -- PostgreSQL は JSONB、SQLite は TEXT
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_events_org_repo ON events(org, repo);
CREATE INDEX idx_events_member ON events(member);
CREATE INDEX idx_events_timestamp ON events(timestamp);
CREATE INDEX idx_events_type ON events(type);
```

#### metrics テーブル（集計メトリクス）

```sql
CREATE TABLE metrics (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,  -- 'commit', 'pull_request', 'code_change', 'deploy'
    org TEXT NOT NULL,
    repo TEXT,           -- NULL の場合は Organization 全体
    member TEXT,           -- NULL の場合は Repository 全体
    value BIGINT NOT NULL,
    time_range_start TIMESTAMP NOT NULL,
    time_range_end TIMESTAMP NOT NULL,
    granularity TEXT NOT NULL,  -- 'day', 'week', 'month'
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org, repo, member, type, time_range_start, time_range_end, granularity)
);

CREATE INDEX idx_metrics_org ON metrics(org);
CREATE INDEX idx_metrics_repo ON metrics(org, repo);
CREATE INDEX idx_metrics_member ON metrics(org, member);
CREATE INDEX idx_metrics_time_range ON metrics(time_range_start, time_range_end);
```

#### repositories テーブル（Repository メタデータ）

```sql
CREATE TABLE repositories (
    org TEXT NOT NULL,
    name TEXT NOT NULL,
    full_name TEXT NOT NULL,
    is_private BOOLEAN NOT NULL,
    last_synced_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (org, name)
);

CREATE INDEX idx_repositories_org ON repositories(org);
```

#### members テーブル（Member メタデータ）

```sql
CREATE TABLE members (
    org TEXT NOT NULL,
    username TEXT NOT NULL,
    display_name TEXT,
    last_synced_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (org, username)
);

CREATE INDEX idx_members_org ON members(org);
```

## Storage 実装詳細

### SQLite Adapter

```go
// internal/storage/sqlite/adapter.go
package sqlite

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github-activity-metrics/internal/storage"
)

type sqliteStorage struct {
	db *sql.DB
}

func NewSQLiteStorage(dbPath string) (storage.Storage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	
	s := &sqliteStorage{db: db}
	if err := s.Migrate(context.Background()); err != nil {
		return nil, err
	}
	
	return s, nil
}

func (s *sqliteStorage) SaveRawEvent(ctx context.Context, event *domain.Event) error {
	// 実装: INSERT INTO events ...
}

func (s *sqliteStorage) GetMetricsByOrg(ctx context.Context, org string, timeRange domain.TimeRange) (*domain.OrgMetrics, error) {
	// 実装: SELECT で集計
}
```

### PostgreSQL Adapter

```go
// internal/storage/postgres/adapter.go
package postgres

import (
	"database/sql"
	_ "github.com/lib/pq"
	"github-activity-metrics/internal/storage"
)

type postgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(connStr string) (storage.Storage, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	
	s := &postgresStorage{db: db}
	if err := s.Migrate(context.Background()); err != nil {
		return nil, err
	}
	
	return s, nil
}

// SQLite と同じインターフェースを実装
// PostgreSQL 固有の最適化（パーティショニング等）は内部で実装
```

## 設定管理

```go
// internal/config/config.go
package config

import (
	"os"
	"github.com/joho/godotenv"
)

type Config struct {
	// GitHub
	GitHubToken string
	
	// Storage
	StorageType string // "sqlite" or "postgres"
	SQLitePath  string
	PostgresURL string
	
	// API Server
	APIPort string
	APIHost string
	
	// CLI
	APIEndpoint string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // .env ファイルを読み込み（エラーは無視）
	
	return &Config{
		GitHubToken: getEnv("GITHUB_TOKEN", ""),
		StorageType: getEnv("STORAGE_TYPE", "sqlite"),
		SQLitePath:  getEnv("SQLITE_PATH", "./metrics.db"),
		PostgresURL: getEnv("POSTGRES_URL", ""),
		APIPort:     getEnv("API_PORT", "8080"),
		APIHost:     getEnv("API_HOST", "localhost"),
		APIEndpoint: getEnv("API_ENDPOINT", "http://localhost:8080"),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
```

## エラーハンドリング

```go
// internal/errors/errors.go
package errors

import "fmt"

type ErrCode string

const (
	ErrCodeNotFound      ErrCode = "NOT_FOUND"
	ErrCodeUnauthorized  ErrCode = "UNAUTHORIZED"
	ErrCodeRateLimited   ErrCode = "RATE_LIMITED"
	ErrCodeInternal      ErrCode = "INTERNAL_ERROR"
)

type AppError struct {
	Code    ErrCode
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewNotFoundError(resource string) *AppError {
	return &AppError{
		Code:    ErrCodeNotFound,
		Message: fmt.Sprintf("%s not found", resource),
	}
}
```

## 並列処理設計

### Collector の並列化

```go
// internal/collector/collector.go (続き)

func (c *githubCollector) CollectOrganizationData(
	ctx context.Context,
	org string,
	since, until time.Time,
	onProgress func(repo string, progress float64),
) ([]*domain.Event, error) {
	repos, err := c.GetRepositories(ctx, org)
	if err != nil {
		return nil, err
	}
	
	// 並列処理用の channel
	eventCh := make(chan *domain.Event, 100)
	errCh := make(chan error, len(repos))
	
	// 各 Repository を並列で処理
	var wg sync.WaitGroup
	for i, repo := range repos {
		wg.Add(1)
		go func(r string, index int) {
			defer wg.Done()
			
			// レート制限チェック
			if err := c.rateLimiter.Wait(ctx); err != nil {
				errCh <- err
				return
			}
			
			// Commit 取得
			commits, err := c.GetCommits(ctx, org, r, since, until)
			if err != nil {
				errCh <- err
				return
			}
			
			for _, commit := range commits {
				eventCh <- commit.ToEvent()
			}
			
			// PR 取得
			prs, err := c.GetPullRequests(ctx, org, r, since, until)
			if err != nil {
				errCh <- err
				return
			}
			
			for _, pr := range prs {
				eventCh <- pr.ToEvent()
			}
			
			// 進捗通知
			if onProgress != nil {
				onProgress(r, float64(index+1)/float64(len(repos)))
			}
		}(repo, i)
	}
	
	// 完了を待機
	go func() {
		wg.Wait()
		close(eventCh)
		close(errCh)
	}()
	
	// イベントを収集
	var events []*domain.Event
	for event := range eventCh {
		events = append(events, event)
	}
	
	// エラーチェック
	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	default:
	}
	
	return events, nil
}
```

## テスト戦略

### 単体テスト

- `internal/collector`: GitHub API のモックを使用
- `internal/aggregator`: テストデータで集計ロジックを検証
- `internal/storage`: インメモリ SQLite またはテスト用 PostgreSQL を使用

### 統合テスト

- API Server + Storage の統合テスト
- CLI + API の統合テスト

### テストツール

- `testify` をテストフレームワークとして使用
- `httptest` で API テスト
- `github.com/DATA-DOG/go-sqlmock` で DB モック

## 依存関係

### 主要ライブラリ

```go
// go.mod (主要部分)
module github-activity-metrics

require (
	github.com/gin-gonic/gin v1.9.1          // API Server
	github.com/spf13/cobra v1.8.0            // CLI
	github.com/google/go-github/v55 v55.0.0  // GitHub API Client
	github.com/mattn/go-sqlite3 v1.14.17     // SQLite Driver
	github.com/lib/pq v1.10.9                // PostgreSQL Driver
	github.com/joho/godotenv v1.5.1          // 環境変数管理
	github.com/stretchr/testify v1.8.4       // テスト
)
```

## デプロイ・実行方法

### API Server 起動

```bash
# 環境変数設定
export GITHUB_TOKEN=your_token
export STORAGE_TYPE=sqlite
export SQLITE_PATH=./metrics.db

# サーバー起動
go run cmd/api/main.go
```

### CLI 実行

```bash
# データ収集
github-metrics collect myorg

# メトリクス表示
github-metrics show myorg

# メンバー別表示
github-metrics show member myorg username
```

## パフォーマンス考慮事項

1. **並列処理**: Repository 単位で goroutine を使用して並列収集
2. **バッチ処理**: イベント保存はバッチで一括 INSERT
3. **インデックス**: クエリパターンに応じた適切なインデックス設計
4. **キャッシュ**: 集計結果を metrics テーブルにキャッシュ
5. **レート制限**: GitHub API のレート制限を適切に管理

## セキュリティ考慮事項

1. **認証情報**: 環境変数または設定ファイルで管理（コードに含めない）
2. **入力検証**: API エンドポイントでの入力値検証
3. **SQL インジェクション**: プリペアドステートメントの使用
4. **CORS**: 必要に応じて CORS 設定

## 拡張性考慮事項

1. **新しいイベント種別**: `EventType` を追加するだけで拡張可能
2. **新しいメトリクス**: `MetricType` と集計ロジックを追加
3. **新しいストレージ**: `Storage` インターフェースを実装
4. **Webhook 対応**: 将来的に Webhook でリアルタイム更新

## 今後の拡張候補

- Webhook によるリアルタイム更新
- グラフ生成（CLI での簡易グラフ表示）
- エクスポート機能（CSV / JSON）
- 複数 Organization 対応
- 認証機能（API Server へのアクセス制御）

