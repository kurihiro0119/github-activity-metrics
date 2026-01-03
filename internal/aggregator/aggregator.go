package aggregator

import (
	"context"
	"time"

	"github.com/kurihiro0119/github-activity-metrics/internal/domain"
	"github.com/kurihiro0119/github-activity-metrics/internal/storage"
)

// Aggregator defines the interface for aggregating metrics
type Aggregator interface {
	// AggregateOrgMetrics aggregates organization-level metrics
	AggregateOrgMetrics(ctx context.Context, org string, timeRange domain.TimeRange) (*domain.OrgMetrics, error)

	// AggregateMemberMetrics aggregates member-level metrics
	AggregateMemberMetrics(ctx context.Context, org, member string, timeRange domain.TimeRange) (*domain.MemberMetrics, error)

	// AggregateRepoMetrics aggregates repository-level metrics
	AggregateRepoMetrics(ctx context.Context, org, repo string, timeRange domain.TimeRange) (*domain.RepoMetrics, error)

	// GetMembersMetrics retrieves metrics for all members
	GetMembersMetrics(ctx context.Context, org string, timeRange domain.TimeRange) ([]*domain.MemberMetrics, error)

	// GetReposMetrics retrieves metrics for all repositories
	GetReposMetrics(ctx context.Context, org string, timeRange domain.TimeRange) ([]*domain.RepoMetrics, error)

	// GetTimeSeriesMetrics retrieves time series metrics
	GetTimeSeriesMetrics(ctx context.Context, org string, metricType domain.MetricType, timeRange domain.TimeRange) (*domain.TimeSeriesData, error)
}

// aggregator implements the Aggregator interface
type aggregator struct {
	storage storage.Storage
}

// NewAggregator creates a new aggregator
func NewAggregator(storage storage.Storage) Aggregator {
	return &aggregator{
		storage: storage,
	}
}

// AggregateOrgMetrics aggregates organization-level metrics
func (a *aggregator) AggregateOrgMetrics(ctx context.Context, org string, timeRange domain.TimeRange) (*domain.OrgMetrics, error) {
	return a.storage.GetMetricsByOrg(ctx, org, timeRange)
}

// AggregateMemberMetrics aggregates member-level metrics
func (a *aggregator) AggregateMemberMetrics(ctx context.Context, org, member string, timeRange domain.TimeRange) (*domain.MemberMetrics, error) {
	return a.storage.GetMetricsByMember(ctx, org, member, timeRange)
}

// AggregateRepoMetrics aggregates repository-level metrics
func (a *aggregator) AggregateRepoMetrics(ctx context.Context, org, repo string, timeRange domain.TimeRange) (*domain.RepoMetrics, error) {
	return a.storage.GetMetricsByRepo(ctx, org, repo, timeRange)
}

// GetMembersMetrics retrieves metrics for all members
func (a *aggregator) GetMembersMetrics(ctx context.Context, org string, timeRange domain.TimeRange) ([]*domain.MemberMetrics, error) {
	return a.storage.GetMembersWithMetrics(ctx, org, timeRange)
}

// GetReposMetrics retrieves metrics for all repositories
func (a *aggregator) GetReposMetrics(ctx context.Context, org string, timeRange domain.TimeRange) ([]*domain.RepoMetrics, error) {
	return a.storage.GetReposWithMetrics(ctx, org, timeRange)
}

// GetTimeSeriesMetrics retrieves time series metrics
func (a *aggregator) GetTimeSeriesMetrics(ctx context.Context, org string, metricType domain.MetricType, timeRange domain.TimeRange) (*domain.TimeSeriesData, error) {
	// Get events for the time range
	var eventType domain.EventType
	switch metricType {
	case domain.MetricTypeCommit:
		eventType = domain.EventTypeCommit
	case domain.MetricTypePullRequest:
		eventType = domain.EventTypePullRequest
	case domain.MetricTypeDeploy:
		eventType = domain.EventTypeDeploy
	default:
		eventType = domain.EventTypeCommit
	}

	events, err := a.storage.GetEvents(ctx, org, eventType, timeRange)
	if err != nil {
		return nil, err
	}

	// Group events by time period
	periodCounts := make(map[time.Time]int64)
	for _, event := range events {
		period := truncateTime(event.Timestamp, timeRange.Granularity)
		periodCounts[period]++
	}

	// Generate all periods in the range
	var dataPoints []domain.TimeSeriesMetric
	current := truncateTime(timeRange.Start, timeRange.Granularity)
	for !current.After(timeRange.End) {
		count := periodCounts[current]
		dataPoints = append(dataPoints, domain.TimeSeriesMetric{
			Timestamp: current,
			Value:     count,
		})
		current = getNextPeriod(current, timeRange.Granularity)
	}

	return &domain.TimeSeriesData{
		Type:        metricType,
		Granularity: timeRange.Granularity,
		DataPoints:  dataPoints,
	}, nil
}

// truncateTime truncates a time to the start of the period based on granularity
func truncateTime(t time.Time, granularity string) time.Time {
	switch granularity {
	case "day":
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case "week":
		// Get the start of the week (Monday)
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return time.Date(t.Year(), t.Month(), t.Day()-weekday+1, 0, 0, 0, 0, t.Location())
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
}

// getNextPeriod returns the start of the next period
func getNextPeriod(t time.Time, granularity string) time.Time {
	switch granularity {
	case "day":
		return t.AddDate(0, 0, 1)
	case "week":
		return t.AddDate(0, 0, 7)
	case "month":
		return t.AddDate(0, 1, 0)
	default:
		return t.AddDate(0, 0, 1)
	}
}
