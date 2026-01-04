package collector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"

	"github.com/kurihiro0119/github-activity-metrics/internal/domain"
)

// githubCollector implements Collector using GitHub API
type githubCollector struct {
	client      *github.Client
	rateLimiter RateLimiter
}

// NewGitHubCollector creates a new GitHub collector
func NewGitHubCollector(token string) Collector {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	
	// Create HTTP client with timeout
	tc := oauth2.NewClient(ctx, ts)
	tc.Timeout = 30 * time.Second // Set 30 second timeout
	
	client := github.NewClient(tc)

	return &githubCollector{
		client:      client,
		rateLimiter: NewRateLimiter(),
	}
}

// GetRepositories retrieves all repositories for an organization
func (c *githubCollector) GetRepositories(ctx context.Context, org string) ([]*domain.Repository, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	var allRepos []*domain.Repository
	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	pageCount := 0
	for {
		pageCount++
		
		// Wait for rate limiter before making API call
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter error: %w", err)
		}
		
		fmt.Printf("  Fetching page %d...\n", pageCount)
		repos, resp, err := c.client.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			// Handle rate limit error (403)
			if resp != nil && resp.StatusCode == 403 {
				// Update rate limiter from response if available
				if resp.Rate.Remaining == 0 && !resp.Rate.Reset.Time.IsZero() {
					c.rateLimiter.UpdateLimit(0, resp.Rate.Reset.Time)
					waitDuration := time.Until(resp.Rate.Reset.Time)
					if waitDuration > 0 {
						fmt.Printf("  Rate limit exceeded. Waiting %v until reset...\n", waitDuration.Round(time.Second))
						select {
						case <-ctx.Done():
							return nil, ctx.Err()
						case <-time.After(waitDuration):
							// Retry after waiting
							continue
						}
					}
				}
				return nil, fmt.Errorf("rate limit exceeded: %w", err)
			}
			// Provide more detailed error information
			if resp != nil {
				return nil, fmt.Errorf("failed to list repositories (status: %d): %w", resp.StatusCode, err)
			}
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		c.updateRateLimitFromResponse(resp)
		fmt.Printf("  Processed page %d, found %d repositories (rate limit: %d remaining)\n", pageCount, len(repos), resp.Rate.Remaining)

		for _, repo := range repos {
			now := time.Now()
			allRepos = append(allRepos, &domain.Repository{
				Org:       org,
				Name:      repo.GetName(),
				FullName:  repo.GetFullName(),
				IsPrivate: repo.GetPrivate(),
				OwnerType: "organization",
				CreatedAt: now,
				UpdatedAt: now,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

// GetCommits retrieves commits for a repository
func (c *githubCollector) GetCommits(ctx context.Context, org, repo string, since, until time.Time) ([]*domain.CommitEvent, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	var allCommits []*domain.CommitEvent
	opts := &github.CommitsListOptions{
		Since:       since,
		Until:       until,
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		commits, resp, err := c.client.Repositories.ListCommits(ctx, org, repo, opts)
		if err != nil {
			// Skip if repository is empty or has no commits
			if resp != nil && resp.StatusCode == 409 {
				return allCommits, nil
			}
			return nil, fmt.Errorf("failed to list commits for %s/%s: %w", org, repo, err)
		}

		c.updateRateLimitFromResponse(resp)

		for _, commit := range commits {
			author := ""
			if commit.Author != nil {
				author = commit.Author.GetLogin()
			} else if commit.Commit != nil && commit.Commit.Author != nil {
				author = commit.Commit.Author.GetName()
			}

			// Get commit details for additions/deletions
			additions := 0
			deletions := 0
			filesChanged := 0

			if err := c.rateLimiter.Wait(ctx); err != nil {
				return nil, err
			}

			commitDetail, detailResp, err := c.client.Repositories.GetCommit(ctx, org, repo, commit.GetSHA(), nil)
			if err == nil {
				c.updateRateLimitFromResponse(detailResp)
				if commitDetail.Stats != nil {
					additions = commitDetail.Stats.GetAdditions()
					deletions = commitDetail.Stats.GetDeletions()
				}
				filesChanged = len(commitDetail.Files)
			}

			// Generate unique ID based on org, repo, type, and SHA to prevent duplicates
			commitID := fmt.Sprintf("%s-%s-commit-%s", org, repo, commit.GetSHA())

			commitEvent := &domain.CommitEvent{
				ID:           commitID,
				Org:          org,
				Repo:         repo,
				Member:       author,
				OwnerType:    "organization",
				Timestamp:    commit.Commit.Author.GetDate().Time,
				Sha:          commit.GetSHA(),
				Message:      commit.Commit.GetMessage(),
				Additions:    additions,
				Deletions:    deletions,
				FilesChanged: filesChanged,
				CreatedAt:    time.Now(),
			}
			allCommits = append(allCommits, commitEvent)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage

		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	return allCommits, nil
}

// GetPullRequests retrieves pull requests for a repository
func (c *githubCollector) GetPullRequests(ctx context.Context, org, repo string, since, until time.Time) ([]*domain.PullRequestEvent, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	var allPRs []*domain.PullRequestEvent
	opts := &github.PullRequestListOptions{
		State:       "all",
		Sort:        "created",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		prs, resp, err := c.client.PullRequests.List(ctx, org, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list pull requests for %s/%s: %w", org, repo, err)
		}

		c.updateRateLimitFromResponse(resp)

		for _, pr := range prs {
			createdAt := pr.GetCreatedAt().Time
			if createdAt.Before(since) {
				// PRs are sorted by created date desc, so we can stop here
				return allPRs, nil
			}
			if createdAt.After(until) {
				continue
			}

			state := pr.GetState()
			if pr.GetMerged() {
				state = "merged"
			}

			var mergedAt *time.Time
			if pr.MergedAt != nil {
				t := pr.MergedAt.Time
				mergedAt = &t
			}

			// Generate unique ID based on org, repo, type, and PR number to prevent duplicates
			prID := fmt.Sprintf("%s-%s-pr-%d", org, repo, pr.GetNumber())

			prEvent := &domain.PullRequestEvent{
				ID:        prID,
				Org:       org,
				Repo:      repo,
				Member:    pr.User.GetLogin(),
				OwnerType: "organization",
				Timestamp: createdAt,
				Number:    pr.GetNumber(),
				State:     state,
				Title:     pr.GetTitle(),
				MergedAt:  mergedAt,
				CreatedAt: time.Now(),
			}
			allPRs = append(allPRs, prEvent)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage

		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	return allPRs, nil
}

// GetDeploys retrieves deployment events for a repository (from GitHub Actions)
func (c *githubCollector) GetDeploys(ctx context.Context, org, repo string, since, until time.Time) ([]*domain.DeployEvent, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	var allDeploys []*domain.DeployEvent
	opts := &github.DeploymentsListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		deployments, resp, err := c.client.Repositories.ListDeployments(ctx, org, repo, opts)
		if err != nil {
			// Skip if deployments are not available
			if resp != nil && resp.StatusCode == 404 {
				return allDeploys, nil
			}
			return nil, fmt.Errorf("failed to list deployments for %s/%s: %w", org, repo, err)
		}

		c.updateRateLimitFromResponse(resp)

		for _, deployment := range deployments {
			createdAt := deployment.GetCreatedAt().Time
			if createdAt.Before(since) || createdAt.After(until) {
				continue
			}

			// Get deployment status
			if err := c.rateLimiter.Wait(ctx); err != nil {
				return nil, err
			}

			statuses, statusResp, err := c.client.Repositories.ListDeploymentStatuses(ctx, org, repo, deployment.GetID(), &github.ListOptions{PerPage: 1})
			if err != nil {
				continue
			}
			c.updateRateLimitFromResponse(statusResp)

			status := "unknown"
			if len(statuses) > 0 {
				status = statuses[0].GetState()
			}

			creator := ""
			if deployment.Creator != nil {
				creator = deployment.Creator.GetLogin()
			}

			// Generate unique ID based on org, repo, type, and deployment ID to prevent duplicates
			deployID := fmt.Sprintf("%s-%s-deploy-%d", org, repo, deployment.GetID())

			deployEvent := &domain.DeployEvent{
				ID:            deployID,
				Org:           org,
				Repo:          repo,
				Member:        creator,
				OwnerType:     "organization",
				Timestamp:     createdAt,
				Environment:   deployment.GetEnvironment(),
				Status:        status,
				WorkflowRunID: fmt.Sprintf("%d", deployment.GetID()),
				CreatedAt:     time.Now(),
			}
			allDeploys = append(allDeploys, deployEvent)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage

		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	return allDeploys, nil
}

// GetMembers retrieves all members of an organization
func (c *githubCollector) GetMembers(ctx context.Context, org string) ([]*domain.Member, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	var allMembers []*domain.Member
	opts := &github.ListMembersOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		members, resp, err := c.client.Organizations.ListMembers(ctx, org, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list members for %s: %w", org, err)
		}

		c.updateRateLimitFromResponse(resp)

		for _, member := range members {
			now := time.Now()
			allMembers = append(allMembers, &domain.Member{
				Org:         org,
				Username:    member.GetLogin(),
				DisplayName: member.GetName(),
				OwnerType:   "organization",
				CreatedAt:   now,
				UpdatedAt:   now,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage

		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	return allMembers, nil
}

// CollectOrganizationData collects all data for an organization
func (c *githubCollector) CollectOrganizationData(ctx context.Context, org string, since, until time.Time, onProgress func(repo string, progress float64)) ([]*domain.Event, error) {
	// Get all repositories
	repos, err := c.GetRepositories(ctx, org)
	if err != nil {
		return nil, err
	}

	var allEvents []*domain.Event
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(repos))

	// Limit concurrent goroutines
	semaphore := make(chan struct{}, 5)

	for i, repo := range repos {
		wg.Add(1)
		go func(r *domain.Repository, index int) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Collect commits
			commits, err := c.GetCommits(ctx, org, r.Name, since, until)
			if err != nil {
				errCh <- fmt.Errorf("failed to get commits for %s: %w", r.Name, err)
				return
			}

			mu.Lock()
			for _, commit := range commits {
				allEvents = append(allEvents, commit.ToEvent())
			}
			mu.Unlock()

			// Collect pull requests
			prs, err := c.GetPullRequests(ctx, org, r.Name, since, until)
			if err != nil {
				errCh <- fmt.Errorf("failed to get pull requests for %s: %w", r.Name, err)
				return
			}

			mu.Lock()
			for _, pr := range prs {
				allEvents = append(allEvents, pr.ToEvent())
			}
			mu.Unlock()

			// Collect deployments
			deploys, err := c.GetDeploys(ctx, org, r.Name, since, until)
			if err != nil {
				errCh <- fmt.Errorf("failed to get deployments for %s: %w", r.Name, err)
				return
			}

			mu.Lock()
			for _, deploy := range deploys {
				allEvents = append(allEvents, deploy.ToEvent())
			}
			mu.Unlock()

			// Report progress
			if onProgress != nil {
				onProgress(r.Name, float64(index+1)/float64(len(repos)))
			}
		}(repo, i)
	}

	wg.Wait()
	close(errCh)

	// Check for errors
	for err := range errCh {
		if err != nil {
			// Log error but continue with other repos (EDGE-001)
			fmt.Printf("Warning: %v\n", err)
		}
	}

	return allEvents, nil
}

// CollectOrganizationDataWithCallback collects data and calls callback for each repository's events
func (c *githubCollector) CollectOrganizationDataWithCallback(ctx context.Context, org string, since, until time.Time, onProgress func(repo string, progress float64), onRepoComplete func(repo string, events []*domain.Event) error) error {
	// Get all repositories
	repos, err := c.GetRepositories(ctx, org)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(repos))

	// Limit concurrent goroutines
	semaphore := make(chan struct{}, 5)

	for i, repo := range repos {
		wg.Add(1)
		go func(r *domain.Repository, index int) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			var repoEvents []*domain.Event

			// Collect commits
			commits, err := c.GetCommits(ctx, org, r.Name, since, until)
			if err != nil {
				errCh <- fmt.Errorf("failed to get commits for %s: %w", r.Name, err)
				return
			}
			for _, commit := range commits {
				repoEvents = append(repoEvents, commit.ToEvent())
			}

			// Collect pull requests
			prs, err := c.GetPullRequests(ctx, org, r.Name, since, until)
			if err != nil {
				errCh <- fmt.Errorf("failed to get pull requests for %s: %w", r.Name, err)
				return
			}
			for _, pr := range prs {
				repoEvents = append(repoEvents, pr.ToEvent())
			}

			// Collect deployments
			deploys, err := c.GetDeploys(ctx, org, r.Name, since, until)
			if err != nil {
				errCh <- fmt.Errorf("failed to get deployments for %s: %w", r.Name, err)
				return
			}
			for _, deploy := range deploys {
				repoEvents = append(repoEvents, deploy.ToEvent())
			}

			// Call callback to save events for this repository
			if onRepoComplete != nil {
				if err := onRepoComplete(r.Name, repoEvents); err != nil {
					errCh <- fmt.Errorf("failed to save events for %s: %w", r.Name, err)
					return
				}
			}

			// Report progress
			if onProgress != nil {
				onProgress(r.Name, float64(index+1)/float64(len(repos)))
			}
		}(repo, i)
	}

	wg.Wait()
	close(errCh)

	// Check for errors
	for err := range errCh {
		if err != nil {
			// Log error but continue with other repos
			fmt.Printf("Warning: %v\n", err)
		}
	}

	return nil
}

// GetUserRepositories retrieves all repositories for a user
func (c *githubCollector) GetUserRepositories(ctx context.Context, user string) ([]*domain.Repository, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	var allRepos []*domain.Repository
	opts := &github.RepositoryListOptions{
		Type:        "all", // all, owner, member
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		repos, resp, err := c.client.Repositories.List(ctx, user, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories for user %s: %w", user, err)
		}

		c.updateRateLimitFromResponse(resp)

		for _, repo := range repos {
			now := time.Now()
			allRepos = append(allRepos, &domain.Repository{
				Org:       user, // Use user as org for consistency
				Name:      repo.GetName(),
				FullName:  repo.GetFullName(),
				IsPrivate: repo.GetPrivate(),
				OwnerType: "user",
				CreatedAt: now,
				UpdatedAt: now,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage

		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	return allRepos, nil
}

// CollectUserData collects all data for a user account
func (c *githubCollector) CollectUserData(ctx context.Context, user string, since, until time.Time, onProgress func(repo string, progress float64)) ([]*domain.Event, error) {
	// Get all repositories
	repos, err := c.GetUserRepositories(ctx, user)
	if err != nil {
		return nil, err
	}

	var allEvents []*domain.Event
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(repos))

	// Limit concurrent goroutines
	semaphore := make(chan struct{}, 5)

	for i, repo := range repos {
		wg.Add(1)
		go func(r *domain.Repository, index int) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Collect commits
			commits, err := c.GetCommits(ctx, user, r.Name, since, until)
			if err != nil {
				errCh <- fmt.Errorf("failed to get commits for %s: %w", r.Name, err)
				return
			}

			mu.Lock()
			for _, commit := range commits {
				event := commit.ToEvent()
				event.OwnerType = "user"
				allEvents = append(allEvents, event)
			}
			mu.Unlock()

			// Collect pull requests
			prs, err := c.GetPullRequests(ctx, user, r.Name, since, until)
			if err != nil {
				errCh <- fmt.Errorf("failed to get pull requests for %s: %w", r.Name, err)
				return
			}

			mu.Lock()
			for _, pr := range prs {
				event := pr.ToEvent()
				event.OwnerType = "user"
				allEvents = append(allEvents, event)
			}
			mu.Unlock()

			// Collect deployments
			deploys, err := c.GetDeploys(ctx, user, r.Name, since, until)
			if err != nil {
				errCh <- fmt.Errorf("failed to get deployments for %s: %w", r.Name, err)
				return
			}

			mu.Lock()
			for _, deploy := range deploys {
				event := deploy.ToEvent()
				event.OwnerType = "user"
				allEvents = append(allEvents, event)
			}
			mu.Unlock()

			// Report progress
			if onProgress != nil {
				onProgress(r.Name, float64(index+1)/float64(len(repos)))
			}
		}(repo, i)
	}

	wg.Wait()
	close(errCh)

	// Check for errors
	for err := range errCh {
		if err != nil {
			// Log error but continue with other repos
			fmt.Printf("Warning: %v\n", err)
		}
	}

	return allEvents, nil
}

// CollectUserDataWithCallback collects data and calls callback for each repository's events
func (c *githubCollector) CollectUserDataWithCallback(ctx context.Context, user string, since, until time.Time, onProgress func(repo string, progress float64), onRepoComplete func(repo string, events []*domain.Event) error) error {
	// Get all repositories
	repos, err := c.GetUserRepositories(ctx, user)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(repos))

	// Limit concurrent goroutines
	semaphore := make(chan struct{}, 5)

	for i, repo := range repos {
		wg.Add(1)
		go func(r *domain.Repository, index int) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			var repoEvents []*domain.Event

			// Collect commits
			commits, err := c.GetCommits(ctx, user, r.Name, since, until)
			if err != nil {
				errCh <- fmt.Errorf("failed to get commits for %s: %w", r.Name, err)
				return
			}
			for _, commit := range commits {
				event := commit.ToEvent()
				event.OwnerType = "user"
				repoEvents = append(repoEvents, event)
			}

			// Collect pull requests
			prs, err := c.GetPullRequests(ctx, user, r.Name, since, until)
			if err != nil {
				errCh <- fmt.Errorf("failed to get pull requests for %s: %w", r.Name, err)
				return
			}
			for _, pr := range prs {
				event := pr.ToEvent()
				event.OwnerType = "user"
				repoEvents = append(repoEvents, event)
			}

			// Collect deployments
			deploys, err := c.GetDeploys(ctx, user, r.Name, since, until)
			if err != nil {
				errCh <- fmt.Errorf("failed to get deployments for %s: %w", r.Name, err)
				return
			}
			for _, deploy := range deploys {
				event := deploy.ToEvent()
				event.OwnerType = "user"
				repoEvents = append(repoEvents, event)
			}

			// Call callback to save events for this repository
			if onRepoComplete != nil {
				if err := onRepoComplete(r.Name, repoEvents); err != nil {
					errCh <- fmt.Errorf("failed to save events for %s: %w", r.Name, err)
					return
				}
			}

			// Report progress
			if onProgress != nil {
				onProgress(r.Name, float64(index+1)/float64(len(repos)))
			}
		}(repo, i)
	}

	wg.Wait()
	close(errCh)

	// Check for errors
	for err := range errCh {
		if err != nil {
			// Log error but continue with other repos
			fmt.Printf("Warning: %v\n", err)
		}
	}

	return nil
}

// updateRateLimitFromResponse updates the rate limiter from API response
func (c *githubCollector) updateRateLimitFromResponse(resp *github.Response) {
	if resp != nil && resp.Rate.Remaining >= 0 {
		c.rateLimiter.UpdateLimit(resp.Rate.Remaining, resp.Rate.Reset.Time)
	}
}
