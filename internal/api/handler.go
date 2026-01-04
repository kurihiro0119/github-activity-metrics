package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kurihiro0119/github-activity-metrics/internal/aggregator"
	"github.com/kurihiro0119/github-activity-metrics/internal/domain"
	apperrors "github.com/kurihiro0119/github-activity-metrics/internal/errors"
)

// Handler handles API requests
type Handler struct {
	aggregator aggregator.Aggregator
}

// NewHandler creates a new API handler
func NewHandler(agg aggregator.Aggregator) *Handler {
	return &Handler{
		aggregator: agg,
	}
}

// GetOrgMetrics returns organization-level metrics
// GET /api/v1/orgs/:org/metrics
func (h *Handler) GetOrgMetrics(c *gin.Context) {
	org := c.Param("org")
	timeRange := parseTimeRange(c)

	metrics, err := h.aggregator.AggregateOrgMetrics(c.Request.Context(), org, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetMemberMetrics returns member-level metrics
// GET /api/v1/orgs/:org/members/:member/metrics
func (h *Handler) GetMemberMetrics(c *gin.Context) {
	org := c.Param("org")
	member := c.Param("member")
	timeRange := parseTimeRange(c)

	metrics, err := h.aggregator.AggregateMemberMetrics(c.Request.Context(), org, member, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetRepoMetrics returns repository-level metrics
// GET /api/v1/orgs/:org/repos/:repo/metrics
func (h *Handler) GetRepoMetrics(c *gin.Context) {
	org := c.Param("org")
	repo := c.Param("repo")
	timeRange := parseTimeRange(c)

	metrics, err := h.aggregator.AggregateRepoMetrics(c.Request.Context(), org, repo, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetRepoMembersMetrics returns metrics for all members in a specific repository
// GET /api/v1/orgs/:org/repos/:repo/members/metrics
func (h *Handler) GetRepoMembersMetrics(c *gin.Context) {
	org := c.Param("org")
	repo := c.Param("repo")
	timeRange := parseTimeRange(c)

	metrics, err := h.aggregator.GetRepoMembersMetrics(c.Request.Context(), org, repo, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetMembersMetrics returns metrics for all members
// GET /api/v1/orgs/:org/members/metrics
func (h *Handler) GetMembersMetrics(c *gin.Context) {
	org := c.Param("org")
	timeRange := parseTimeRange(c)

	metrics, err := h.aggregator.GetMembersMetrics(c.Request.Context(), org, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetReposMetrics returns metrics for all repositories
// GET /api/v1/orgs/:org/repos/metrics
func (h *Handler) GetReposMetrics(c *gin.Context) {
	org := c.Param("org")
	timeRange := parseTimeRange(c)

	metrics, err := h.aggregator.GetReposMetrics(c.Request.Context(), org, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetTimeSeriesMetrics returns time series metrics
// GET /api/v1/orgs/:org/metrics/timeseries
func (h *Handler) GetTimeSeriesMetrics(c *gin.Context) {
	org := c.Param("org")
	metricTypeStr := c.DefaultQuery("type", "commit")
	timeRange := parseTimeRange(c)

	var metricType domain.MetricType
	switch metricTypeStr {
	case "commit":
		metricType = domain.MetricTypeCommit
	case "pull_request":
		metricType = domain.MetricTypePullRequest
	case "deploy":
		metricType = domain.MetricTypeDeploy
	default:
		metricType = domain.MetricTypeCommit
	}

	metrics, err := h.aggregator.GetTimeSeriesMetrics(c.Request.Context(), org, metricType, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetUserMetrics returns user-level metrics (same as org metrics)
// GET /api/v1/users/:user/metrics
func (h *Handler) GetUserMetrics(c *gin.Context) {
	user := c.Param("user")
	timeRange := parseTimeRange(c)

	// Use org metrics aggregator (user is stored as org in the database)
	metrics, err := h.aggregator.AggregateOrgMetrics(c.Request.Context(), user, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetUserTimeSeriesMetrics returns time series metrics for a user
// GET /api/v1/users/:user/metrics/timeseries
func (h *Handler) GetUserTimeSeriesMetrics(c *gin.Context) {
	user := c.Param("user")
	metricTypeStr := c.DefaultQuery("type", "commit")
	timeRange := parseTimeRange(c)

	var metricType domain.MetricType
	switch metricTypeStr {
	case "commit":
		metricType = domain.MetricTypeCommit
	case "pull_request":
		metricType = domain.MetricTypePullRequest
	case "deploy":
		metricType = domain.MetricTypeDeploy
	default:
		metricType = domain.MetricTypeCommit
	}

	// Use org time series aggregator (user is stored as org in the database)
	metrics, err := h.aggregator.GetTimeSeriesMetrics(c.Request.Context(), user, metricType, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetUserReposMetrics returns metrics for all repositories of a user
// GET /api/v1/users/:user/repos/metrics
func (h *Handler) GetUserReposMetrics(c *gin.Context) {
	user := c.Param("user")
	timeRange := parseTimeRange(c)

	// Use org repos metrics aggregator (user is stored as org in the database)
	metrics, err := h.aggregator.GetReposMetrics(c.Request.Context(), user, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetUserRepoMetrics returns repository-level metrics for a user
// GET /api/v1/users/:user/repos/:repo/metrics
func (h *Handler) GetUserRepoMetrics(c *gin.Context) {
	user := c.Param("user")
	repo := c.Param("repo")
	timeRange := parseTimeRange(c)

	// Use org repo metrics aggregator (user is stored as org in the database)
	metrics, err := h.aggregator.AggregateRepoMetrics(c.Request.Context(), user, repo, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetUserRepoMembersMetrics returns metrics for all members in a user's specific repository
// GET /api/v1/users/:user/repos/:repo/members/metrics
func (h *Handler) GetUserRepoMembersMetrics(c *gin.Context) {
	user := c.Param("user")
	repo := c.Param("repo")
	timeRange := parseTimeRange(c)

	// Use org repo members metrics aggregator (user is stored as org in the database)
	metrics, err := h.aggregator.GetRepoMembersMetrics(c.Request.Context(), user, repo, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// GetOrgTimeSeriesDetailed returns detailed time series data for an organization
// GET /api/v1/orgs/:org/metrics/timeseries/detailed
func (h *Handler) GetOrgTimeSeriesDetailed(c *gin.Context) {
	org := c.Param("org")
	timeRange := parseTimeRange(c)

	data, err := h.aggregator.GetOrgTimeSeries(c.Request.Context(), org, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}

// GetRepoTimeSeriesDetailed returns detailed time series data for a repository
// GET /api/v1/orgs/:org/repos/:repo/metrics/timeseries
func (h *Handler) GetRepoTimeSeriesDetailed(c *gin.Context) {
	org := c.Param("org")
	repo := c.Param("repo")
	timeRange := parseTimeRange(c)

	data, err := h.aggregator.GetRepoTimeSeries(c.Request.Context(), org, repo, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}

// GetMemberTimeSeriesDetailed returns detailed time series data for a member
// GET /api/v1/orgs/:org/members/:member/metrics/timeseries
func (h *Handler) GetMemberTimeSeriesDetailed(c *gin.Context) {
	org := c.Param("org")
	member := c.Param("member")
	timeRange := parseTimeRange(c)

	data, err := h.aggregator.GetMemberTimeSeries(c.Request.Context(), org, member, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}

// GetUserTimeSeriesDetailed returns detailed time series data for a user
// GET /api/v1/users/:user/metrics/timeseries/detailed
func (h *Handler) GetUserTimeSeriesDetailed(c *gin.Context) {
	user := c.Param("user")
	timeRange := parseTimeRange(c)

	// Use org time series aggregator (user is stored as org in the database)
	data, err := h.aggregator.GetOrgTimeSeries(c.Request.Context(), user, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}

// GetUserRepoTimeSeriesDetailed returns detailed time series data for a user repository
// GET /api/v1/users/:user/repos/:repo/metrics/timeseries
func (h *Handler) GetUserRepoTimeSeriesDetailed(c *gin.Context) {
	user := c.Param("user")
	repo := c.Param("repo")
	timeRange := parseTimeRange(c)

	// Use org repo time series aggregator (user is stored as org in the database)
	data, err := h.aggregator.GetRepoTimeSeries(c.Request.Context(), user, repo, timeRange)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}

// GetMemberRanking returns member rankings
// GET /api/v1/orgs/:org/rankings/members/:type
func (h *Handler) GetMemberRanking(c *gin.Context) {
	org := c.Param("org")
	rankingTypeStr := c.Param("type")
	timeRange := parseTimeRange(c)
	limit := parseIntQuery(c, "limit", 10)

	var rankingType domain.RankingType
	switch rankingTypeStr {
	case "commits":
		rankingType = domain.RankingTypeCommits
	case "prs":
		rankingType = domain.RankingTypePRs
	case "code-changes":
		rankingType = domain.RankingTypeCodeChanges
	case "deploys":
		rankingType = domain.RankingTypeDeploys
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_RANKING_TYPE",
				"message": "ranking type must be one of: commits, prs, code-changes, deploys",
			},
		})
		return
	}

	rankings, err := h.aggregator.GetMemberRanking(c.Request.Context(), org, rankingType, timeRange, limit)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": rankings,
	})
}

// GetRepoRanking returns repository rankings
// GET /api/v1/orgs/:org/rankings/repos/:type
func (h *Handler) GetRepoRanking(c *gin.Context) {
	org := c.Param("org")
	rankingTypeStr := c.Param("type")
	timeRange := parseTimeRange(c)
	limit := parseIntQuery(c, "limit", 10)

	var rankingType domain.RankingType
	switch rankingTypeStr {
	case "commits":
		rankingType = domain.RankingTypeCommits
	case "prs":
		rankingType = domain.RankingTypePRs
	case "code-changes":
		rankingType = domain.RankingTypeCodeChanges
	case "deploys":
		rankingType = domain.RankingTypeDeploys
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_RANKING_TYPE",
				"message": "ranking type must be one of: commits, prs, code-changes, deploys",
			},
		})
		return
	}

	rankings, err := h.aggregator.GetRepoRanking(c.Request.Context(), org, rankingType, timeRange, limit)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": rankings,
	})
}

// GetUserMemberRanking returns member rankings for a user account
// GET /api/v1/users/:user/rankings/members/:type
func (h *Handler) GetUserMemberRanking(c *gin.Context) {
	user := c.Param("user")
	rankingTypeStr := c.Param("type")
	timeRange := parseTimeRange(c)
	limit := parseIntQuery(c, "limit", 10)

	var rankingType domain.RankingType
	switch rankingTypeStr {
	case "commits":
		rankingType = domain.RankingTypeCommits
	case "prs":
		rankingType = domain.RankingTypePRs
	case "code-changes":
		rankingType = domain.RankingTypeCodeChanges
	case "deploys":
		rankingType = domain.RankingTypeDeploys
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_RANKING_TYPE",
				"message": "ranking type must be one of: commits, prs, code-changes, deploys",
			},
		})
		return
	}

	rankings, err := h.aggregator.GetMemberRanking(c.Request.Context(), user, rankingType, timeRange, limit)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": rankings,
	})
}

// GetUserRepoRanking returns repository rankings for a user account
// GET /api/v1/users/:user/rankings/repos/:type
func (h *Handler) GetUserRepoRanking(c *gin.Context) {
	user := c.Param("user")
	rankingTypeStr := c.Param("type")
	timeRange := parseTimeRange(c)
	limit := parseIntQuery(c, "limit", 10)

	var rankingType domain.RankingType
	switch rankingTypeStr {
	case "commits":
		rankingType = domain.RankingTypeCommits
	case "prs":
		rankingType = domain.RankingTypePRs
	case "code-changes":
		rankingType = domain.RankingTypeCodeChanges
	case "deploys":
		rankingType = domain.RankingTypeDeploys
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_RANKING_TYPE",
				"message": "ranking type must be one of: commits, prs, code-changes, deploys",
			},
		})
		return
	}

	rankings, err := h.aggregator.GetRepoRanking(c.Request.Context(), user, rankingType, timeRange, limit)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": rankings,
	})
}

// parseIntQuery parses an integer query parameter with a default value
func parseIntQuery(c *gin.Context, key string, defaultValue int) int {
	valueStr := c.Query(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil || value <= 0 {
		return defaultValue
	}
	return value
}

// HealthCheck returns the health status of the API
// GET /health
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// parseTimeRange parses time range from query parameters
func parseTimeRange(c *gin.Context) domain.TimeRange {
	// Default to last 30 days
	now := time.Now()
	defaultStart := now.AddDate(0, -1, 0)
	defaultEnd := now

	startStr := c.Query("start")
	endStr := c.Query("end")
	granularity := c.DefaultQuery("granularity", "day")

	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			start = defaultStart
		}
	} else {
		start = defaultStart
	}

	if endStr != "" {
		end, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			end = defaultEnd
		}
	} else {
		end = defaultEnd
	}

	// Validate granularity
	if granularity != "day" && granularity != "week" && granularity != "month" {
		granularity = "day"
	}

	return domain.TimeRange{
		Start:       start,
		End:         end,
		Granularity: granularity,
	}
}

// respondError sends an error response
func respondError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		status := http.StatusInternalServerError
		switch appErr.Code {
		case apperrors.ErrCodeNotFound:
			status = http.StatusNotFound
		case apperrors.ErrCodeUnauthorized:
			status = http.StatusUnauthorized
		case apperrors.ErrCodeForbidden:
			status = http.StatusForbidden
		case apperrors.ErrCodeBadRequest:
			status = http.StatusBadRequest
		case apperrors.ErrCodeRateLimited:
			status = http.StatusTooManyRequests
		}
		c.JSON(status, gin.H{
			"error": gin.H{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{
		"error": gin.H{
			"code":    "INTERNAL_ERROR",
			"message": err.Error(),
		},
	})
}
