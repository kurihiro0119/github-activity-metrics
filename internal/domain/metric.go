package domain

import "time"

// MetricType represents the type of metric
type MetricType string

const (
	MetricTypeCommit      MetricType = "commit"
	MetricTypePullRequest MetricType = "pull_request"
	MetricTypeCodeChange  MetricType = "code_change"
	MetricTypeDeploy      MetricType = "deploy"
)

// TimeRange represents a time range for metrics
type TimeRange struct {
	Start       time.Time
	End         time.Time
	Granularity string // "day", "week", "month"
}

// Metric represents an aggregated metric
type Metric struct {
	ID        string
	Type      MetricType
	Org       string
	Repo      *string // nil means organization-wide
	Member    *string // nil means repository-wide
	Value     int64
	TimeRange TimeRange
	Metadata  map[string]interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
}

// MemberMetrics represents aggregated metrics for a member
type MemberMetrics struct {
	Member    string
	Commits   int64
	PRs       int64
	Additions int64
	Deletions int64
	Deploys   int64
	TimeRange TimeRange
}

// RepoMetrics represents aggregated metrics for a repository
type RepoMetrics struct {
	Repo      string
	Commits   int64
	PRs       int64
	Additions int64
	Deletions int64
	Deploys   int64
	TimeRange TimeRange
}

// OrgMetrics represents aggregated metrics for an organization
type OrgMetrics struct {
	Org          string
	TotalRepos   int
	TotalMembers int
	Commits      int64
	PRs          int64
	Additions    int64
	Deletions    int64
	Deploys      int64
	TimeRange    TimeRange
}

// TimeSeriesMetric represents a single data point in a time series
type TimeSeriesMetric struct {
	Timestamp time.Time
	Value     int64
}

// TimeSeriesData represents time series data for a metric type
type TimeSeriesData struct {
	Type        MetricType
	Granularity string
	DataPoints  []TimeSeriesMetric
}
