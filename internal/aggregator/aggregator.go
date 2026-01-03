package aggregator

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/kurihiro0119/github-activity-metrics/internal/domain"
	"github.com/kurihiro0119/github-activity-metrics/internal/storage"
)

// Aggregator defines the interface for aggregating metrics
type Aggregator interface {
	// AggregateEvents aggregates events into metrics
	AggregateEvents(ctx context.Context, events []*domain.Event, timeRange domain.TimeRange) ([]*domain.Metric, error)

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

// AggregateEvents aggregates events into metrics
func (a *aggregator) AggregateEvents(ctx context.Context, events []*domain.Event, timeRange domain.TimeRange) ([]*domain.Metric, error) {
	// Group events by time period based on granularity
	periodEvents := make(map[time.Time][]*domain.Event)

	for _, event := range events {
		period := truncateTime(event.Timestamp, timeRange.Granularity)
		periodEvents[period] = append(periodEvents[period], event)
	}

	var metrics []*domain.Metric
	now := time.Now()

	for period, evts := range periodEvents {
		// Count events by type
		commitCount := int64(0)
		prCount := int64(0)
		deployCount := int64(0)

		for _, evt := range evts {
			switch evt.Type {
			case domain.EventTypeCommit:
				commitCount++
			case domain.EventTypePullRequest:
				prCount++
			case domain.EventTypeDeploy:
				deployCount++
			}
		}

		periodEnd := getNextPeriod(period, timeRange.Granularity)

		if commitCount > 0 {
			metrics = append(metrics, &domain.Metric{
				ID:   uuid.New().String(),
				Type: domain.MetricTypeCommit,
				Org:  evts[0].Org,
				TimeRange: domain.TimeRange{
					Start:       period,
					End:         periodEnd,
					Granularity: timeRange.Granularity,
				},
				Value:     commitCount,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}

		if prCount > 0 {
			metrics = append(metrics, &domain.Metric{
				ID:   uuid.New().String(),
				Type: domain.MetricTypePullRequest,
				Org:  evts[0].Org,
				TimeRange: domain.TimeRange{
					Start:       period,
					End:         periodEnd,
					Granularity: timeRange.Granularity,
				},
				Value:     prCount,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}

		if deployCount > 0 {
			metrics = append(metrics, &domain.Metric{
				ID:   uuid.New().String(),
				Type: domain.MetricTypeDeploy,
				Org:  evts[0].Org,
				TimeRange: domain.TimeRange{
					Start:       period,
					End:         periodEnd,
					Granularity: timeRange.Granularity,
				},
				Value:     deployCount,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
	}

	return metrics, nil
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
