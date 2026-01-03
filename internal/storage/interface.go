package storage

import (
	"context"

	"github.com/kurihiro0119/github-activity-metrics/internal/domain"
)

// Storage is the abstract interface for the persistence layer
type Storage interface {
	// Raw event operations
	SaveRawEvent(ctx context.Context, event *domain.Event) error
	SaveRawEvents(ctx context.Context, events []*domain.Event) error

	// Aggregated metric operations
	SaveAggregatedMetric(ctx context.Context, metric *domain.Metric) error
	SaveAggregatedMetrics(ctx context.Context, metrics []*domain.Metric) error

	// Metric retrieval
	GetMetricsByOrg(ctx context.Context, org string, timeRange domain.TimeRange) (*domain.OrgMetrics, error)
	GetMetricsByMember(ctx context.Context, org, member string, timeRange domain.TimeRange) (*domain.MemberMetrics, error)
	GetMetricsByRepo(ctx context.Context, org, repo string, timeRange domain.TimeRange) (*domain.RepoMetrics, error)

	// Time series metrics retrieval
	GetTimeSeriesMetrics(ctx context.Context, org string, metricType domain.MetricType, timeRange domain.TimeRange) ([]*domain.Metric, error)

	// Event retrieval (for re-aggregation)
	GetEvents(ctx context.Context, org string, eventType domain.EventType, timeRange domain.TimeRange) ([]*domain.Event, error)

	// Repository operations
	SaveRepository(ctx context.Context, repo *domain.Repository) error
	GetRepositories(ctx context.Context, org string) ([]*domain.Repository, error)

	// Member operations
	SaveMember(ctx context.Context, member *domain.Member) error
	GetMembers(ctx context.Context, org string) ([]*domain.Member, error)

	// List all members with metrics
	GetMembersWithMetrics(ctx context.Context, org string, timeRange domain.TimeRange) ([]*domain.MemberMetrics, error)

	// List all repos with metrics
	GetReposWithMetrics(ctx context.Context, org string, timeRange domain.TimeRange) ([]*domain.RepoMetrics, error)

	// Migration
	Migrate(ctx context.Context) error

	// Connection management
	Close() error
}
