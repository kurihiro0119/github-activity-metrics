package collector

import (
	"context"
	"time"

	"github.com/kurihiro0119/github-activity-metrics/internal/domain"
)

// Collector defines the interface for collecting GitHub data
type Collector interface {
	// GetRepositories retrieves all repositories for an organization
	GetRepositories(ctx context.Context, org string) ([]*domain.Repository, error)

	// GetCommits retrieves commits for a repository
	GetCommits(ctx context.Context, org, repo string, since, until time.Time) ([]*domain.CommitEvent, error)

	// GetPullRequests retrieves pull requests for a repository
	GetPullRequests(ctx context.Context, org, repo string, since, until time.Time) ([]*domain.PullRequestEvent, error)

	// GetDeploys retrieves deployment events for a repository (from GitHub Actions)
	GetDeploys(ctx context.Context, org, repo string, since, until time.Time) ([]*domain.DeployEvent, error)

	// GetMembers retrieves all members of an organization
	GetMembers(ctx context.Context, org string) ([]*domain.Member, error)

	// CollectOrganizationData collects all data for an organization
	CollectOrganizationData(ctx context.Context, org string, since, until time.Time, onProgress func(repo string, progress float64)) ([]*domain.Event, error)
}

// ProgressCallback is a callback function for reporting progress
type ProgressCallback func(repo string, progress float64)
