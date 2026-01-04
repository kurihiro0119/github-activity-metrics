package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/kurihiro0119/github-activity-metrics/internal/aggregator"
	"github.com/kurihiro0119/github-activity-metrics/internal/collector"
	"github.com/kurihiro0119/github-activity-metrics/internal/config"
	"github.com/kurihiro0119/github-activity-metrics/internal/domain"
	"github.com/kurihiro0119/github-activity-metrics/internal/storage"
	"github.com/kurihiro0119/github-activity-metrics/internal/storage/postgres"
	"github.com/kurihiro0119/github-activity-metrics/internal/storage/sqlite"
)

var (
	cfgFile     string
	outputJSON  bool
	startDate   string
	endDate     string
	granularity string
)

var rootCmd = &cobra.Command{
	Use:   "github-metrics",
	Short: "GitHub activity metrics tool",
	Long: `A CLI tool for collecting and visualizing GitHub organization activity metrics.

This tool collects commit, pull request, and deployment data from GitHub
and provides aggregated metrics for organizations, repositories, and members.`,
}

var collectCmd = &cobra.Command{
	Use:   "collect [org|user]",
	Short: "Collect data from GitHub",
	Long:  `Collect activity data from a GitHub organization or user account and store it locally.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runCollect,
}

var showCmd = &cobra.Command{
	Use:   "show [org]",
	Short: "Show organization metrics",
	Long:  `Display aggregated metrics for a GitHub organization.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runShowOrg,
}

var showMembersCmd = &cobra.Command{
	Use:   "members [org]",
	Short: "Show member metrics",
	Long:  `Display metrics for all members in a GitHub organization.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runShowMembers,
}

var showMemberCmd = &cobra.Command{
	Use:   "member [org] [member]",
	Short: "Show metrics for a specific member",
	Long:  `Display metrics for a specific member in a GitHub organization.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runShowMember,
}

var showReposCmd = &cobra.Command{
	Use:   "repos [org]",
	Short: "Show repository metrics",
	Long:  `Display metrics for all repositories in a GitHub organization.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runShowRepos,
}

var showRepoCmd = &cobra.Command{
	Use:   "repo [org] [repo]",
	Short: "Show metrics for a specific repository",
	Long:  `Display metrics for a specific repository in a GitHub organization.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runShowRepo,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .env)")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().StringVar(&startDate, "start", "", "start date (YYYY-MM-DD)")
	rootCmd.PersistentFlags().StringVar(&endDate, "end", "", "end date (YYYY-MM-DD)")
	rootCmd.PersistentFlags().StringVar(&granularity, "granularity", "day", "time granularity (day, week, month)")

	rootCmd.AddCommand(collectCmd)
	rootCmd.AddCommand(showCmd)
	showCmd.AddCommand(showMembersCmd)
	showCmd.AddCommand(showMemberCmd)
	showCmd.AddCommand(showReposCmd)
	showCmd.AddCommand(showRepoCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func getStorage(cfg *config.Config) (storage.Storage, error) {
	switch cfg.StorageType {
	case "postgres":
		return postgres.NewPostgresStorage(cfg.PostgresURL)
	default:
		return sqlite.NewSQLiteStorage(cfg.SQLitePath)
	}
}

func getTimeRange() domain.TimeRange {
	now := time.Now()
	start := now.AddDate(0, -1, 0)
	end := now

	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			start = t
		}
	}

	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			end = t
		}
	}

	return domain.TimeRange{
		Start:       start,
		End:         end,
		Granularity: granularity,
	}
}

func runCollect(cmd *cobra.Command, args []string) error {
	target := args[0] // org or user

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	store, err := getStorage(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	coll := collector.NewGitHubCollector(cfg.GitHubToken)
	ctx := context.Background()
	timeRange := getTimeRange()

	var repos []*domain.Repository
	var events []*domain.Event

	if cfg.Mode == "user" {
		fmt.Printf("Collecting data for user: %s\n", target)
		fmt.Printf("Time range: %s to %s\n", timeRange.Start.Format("2006-01-02"), timeRange.End.Format("2006-01-02"))

		// Collect repositories
		fmt.Println("Fetching repositories...")
		repos, err = coll.GetUserRepositories(ctx, target)
		if err != nil {
			return fmt.Errorf("failed to get repositories: %w", err)
		}
		fmt.Printf("Found %d repositories\n", len(repos))

		// Save repositories
		for _, repo := range repos {
			if err := store.SaveRepository(ctx, repo); err != nil {
				fmt.Printf("Warning: failed to save repository %s: %v\n", repo.Name, err)
			}
		}

		// Save user as member (for consistency)
		now := time.Now()
		member := &domain.Member{
			Org:         target,
			Username:    target,
			DisplayName: target,
			OwnerType:   "user",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := store.SaveMember(ctx, member); err != nil {
			fmt.Printf("Warning: failed to save member %s: %v\n", member.Username, err)
		}

		// Collect events
		fmt.Println("Collecting activity data...")
		events, err = coll.CollectUserData(ctx, target, timeRange.Start, timeRange.End, func(repo string, progress float64) {
			fmt.Printf("\rProgress: %.1f%% (%s)", progress*100, repo)
		})
		if err != nil {
			return fmt.Errorf("failed to collect data: %w", err)
		}
	} else {
		fmt.Printf("Collecting data for organization: %s\n", target)
		fmt.Printf("Time range: %s to %s\n", timeRange.Start.Format("2006-01-02"), timeRange.End.Format("2006-01-02"))

		// Collect repositories
		fmt.Println("Fetching repositories...")
		repos, err = coll.GetRepositories(ctx, target)
		if err != nil {
			return fmt.Errorf("failed to get repositories: %w", err)
		}
		fmt.Printf("Found %d repositories\n", len(repos))

		// Save repositories
		for _, repo := range repos {
			if err := store.SaveRepository(ctx, repo); err != nil {
				fmt.Printf("Warning: failed to save repository %s: %v\n", repo.Name, err)
			}
		}

		// Collect members
		fmt.Println("Fetching members...")
		members, err := coll.GetMembers(ctx, target)
		if err != nil {
			fmt.Printf("Warning: failed to get members: %v\n", err)
		} else {
			fmt.Printf("Found %d members\n", len(members))
			for _, member := range members {
				if err := store.SaveMember(ctx, member); err != nil {
					fmt.Printf("Warning: failed to save member %s: %v\n", member.Username, err)
				}
			}
		}

		// Collect events
		fmt.Println("Collecting activity data...")
		events, err = coll.CollectOrganizationData(ctx, target, timeRange.Start, timeRange.End, func(repo string, progress float64) {
			fmt.Printf("\rProgress: %.1f%% (%s)", progress*100, repo)
		})
		if err != nil {
			return fmt.Errorf("failed to collect data: %w", err)
		}
	}

	fmt.Printf("\nCollected %d events\n", len(events))

	// Save events
	fmt.Println("Saving events...")
	if err := store.SaveRawEvents(ctx, events); err != nil {
		return fmt.Errorf("failed to save events: %w", err)
	}

	fmt.Println("Data collection complete!")
	return nil
}

func runShowOrg(cmd *cobra.Command, args []string) error {
	org := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	store, err := getStorage(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	agg := aggregator.NewAggregator(store)
	ctx := context.Background()
	timeRange := getTimeRange()

	metrics, err := agg.AggregateOrgMetrics(ctx, org, timeRange)
	if err != nil {
		return fmt.Errorf("failed to get metrics: %w", err)
	}

	if outputJSON {
		fmt.Printf(`{"org":"%s","total_repos":%d,"total_members":%d,"commits":%d,"prs":%d,"additions":%d,"deletions":%d,"deploys":%d}`,
			metrics.Org, metrics.TotalRepos, metrics.TotalMembers, metrics.Commits, metrics.PRs, metrics.Additions, metrics.Deletions, metrics.Deploys)
		fmt.Println()
		return nil
	}

	fmt.Printf("\nOrganization Metrics: %s\n", org)
	fmt.Printf("Time Range: %s to %s\n\n", timeRange.Start.Format("2006-01-02"), timeRange.End.Format("2006-01-02"))

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Metric", "Value"})
	table.Append([]string{"Total Repositories", fmt.Sprintf("%d", metrics.TotalRepos)})
	table.Append([]string{"Total Members", fmt.Sprintf("%d", metrics.TotalMembers)})
	table.Append([]string{"Commits", fmt.Sprintf("%d", metrics.Commits)})
	table.Append([]string{"Pull Requests", fmt.Sprintf("%d", metrics.PRs)})
	table.Append([]string{"Lines Added", fmt.Sprintf("%d", metrics.Additions)})
	table.Append([]string{"Lines Deleted", fmt.Sprintf("%d", metrics.Deletions)})
	table.Append([]string{"Deployments", fmt.Sprintf("%d", metrics.Deploys)})
	table.Render()

	return nil
}

func runShowMembers(cmd *cobra.Command, args []string) error {
	org := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	store, err := getStorage(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	agg := aggregator.NewAggregator(store)
	ctx := context.Background()
	timeRange := getTimeRange()

	metrics, err := agg.GetMembersMetrics(ctx, org, timeRange)
	if err != nil {
		return fmt.Errorf("failed to get metrics: %w", err)
	}

	if outputJSON {
		fmt.Print("[")
		for i, m := range metrics {
			if i > 0 {
				fmt.Print(",")
			}
			fmt.Printf(`{"member":"%s","commits":%d,"prs":%d,"additions":%d,"deletions":%d,"deploys":%d}`,
				m.Member, m.Commits, m.PRs, m.Additions, m.Deletions, m.Deploys)
		}
		fmt.Println("]")
		return nil
	}

	fmt.Printf("\nMember Metrics: %s\n", org)
	fmt.Printf("Time Range: %s to %s\n\n", timeRange.Start.Format("2006-01-02"), timeRange.End.Format("2006-01-02"))

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Member", "Commits", "PRs", "Additions", "Deletions", "Deploys"})
	for _, m := range metrics {
		table.Append([]string{
			m.Member,
			fmt.Sprintf("%d", m.Commits),
			fmt.Sprintf("%d", m.PRs),
			fmt.Sprintf("%d", m.Additions),
			fmt.Sprintf("%d", m.Deletions),
			fmt.Sprintf("%d", m.Deploys),
		})
	}
	table.Render()

	return nil
}

func runShowMember(cmd *cobra.Command, args []string) error {
	org := args[0]
	member := args[1]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	store, err := getStorage(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	agg := aggregator.NewAggregator(store)
	ctx := context.Background()
	timeRange := getTimeRange()

	metrics, err := agg.AggregateMemberMetrics(ctx, org, member, timeRange)
	if err != nil {
		return fmt.Errorf("failed to get metrics: %w", err)
	}

	if outputJSON {
		fmt.Printf(`{"member":"%s","commits":%d,"prs":%d,"additions":%d,"deletions":%d,"deploys":%d}`,
			metrics.Member, metrics.Commits, metrics.PRs, metrics.Additions, metrics.Deletions, metrics.Deploys)
		fmt.Println()
		return nil
	}

	fmt.Printf("\nMember Metrics: %s/%s\n", org, member)
	fmt.Printf("Time Range: %s to %s\n\n", timeRange.Start.Format("2006-01-02"), timeRange.End.Format("2006-01-02"))

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Metric", "Value"})
	table.Append([]string{"Commits", fmt.Sprintf("%d", metrics.Commits)})
	table.Append([]string{"Pull Requests", fmt.Sprintf("%d", metrics.PRs)})
	table.Append([]string{"Lines Added", fmt.Sprintf("%d", metrics.Additions)})
	table.Append([]string{"Lines Deleted", fmt.Sprintf("%d", metrics.Deletions)})
	table.Append([]string{"Deployments", fmt.Sprintf("%d", metrics.Deploys)})
	table.Render()

	return nil
}

func runShowRepos(cmd *cobra.Command, args []string) error {
	org := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	store, err := getStorage(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	agg := aggregator.NewAggregator(store)
	ctx := context.Background()
	timeRange := getTimeRange()

	metrics, err := agg.GetReposMetrics(ctx, org, timeRange)
	if err != nil {
		return fmt.Errorf("failed to get metrics: %w", err)
	}

	if outputJSON {
		fmt.Print("[")
		for i, m := range metrics {
			if i > 0 {
				fmt.Print(",")
			}
			fmt.Printf(`{"repo":"%s","commits":%d,"prs":%d,"additions":%d,"deletions":%d,"deploys":%d}`,
				m.Repo, m.Commits, m.PRs, m.Additions, m.Deletions, m.Deploys)
		}
		fmt.Println("]")
		return nil
	}

	fmt.Printf("\nRepository Metrics: %s\n", org)
	fmt.Printf("Time Range: %s to %s\n\n", timeRange.Start.Format("2006-01-02"), timeRange.End.Format("2006-01-02"))

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Repository", "Commits", "PRs", "Additions", "Deletions", "Deploys"})
	for _, m := range metrics {
		table.Append([]string{
			m.Repo,
			fmt.Sprintf("%d", m.Commits),
			fmt.Sprintf("%d", m.PRs),
			fmt.Sprintf("%d", m.Additions),
			fmt.Sprintf("%d", m.Deletions),
			fmt.Sprintf("%d", m.Deploys),
		})
	}
	table.Render()

	return nil
}

func runShowRepo(cmd *cobra.Command, args []string) error {
	org := args[0]
	repo := args[1]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	store, err := getStorage(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	agg := aggregator.NewAggregator(store)
	ctx := context.Background()
	timeRange := getTimeRange()

	metrics, err := agg.AggregateRepoMetrics(ctx, org, repo, timeRange)
	if err != nil {
		return fmt.Errorf("failed to get metrics: %w", err)
	}

	if outputJSON {
		fmt.Printf(`{"repo":"%s","commits":%d,"prs":%d,"additions":%d,"deletions":%d,"deploys":%d}`,
			metrics.Repo, metrics.Commits, metrics.PRs, metrics.Additions, metrics.Deletions, metrics.Deploys)
		fmt.Println()
		return nil
	}

	fmt.Printf("\nRepository Metrics: %s/%s\n", org, repo)
	fmt.Printf("Time Range: %s to %s\n\n", timeRange.Start.Format("2006-01-02"), timeRange.End.Format("2006-01-02"))

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Metric", "Value"})
	table.Append([]string{"Commits", fmt.Sprintf("%d", metrics.Commits)})
	table.Append([]string{"Pull Requests", fmt.Sprintf("%d", metrics.PRs)})
	table.Append([]string{"Lines Added", fmt.Sprintf("%d", metrics.Additions)})
	table.Append([]string{"Lines Deleted", fmt.Sprintf("%d", metrics.Deletions)})
	table.Append([]string{"Deployments", fmt.Sprintf("%d", metrics.Deploys)})
	table.Render()

	return nil
}
