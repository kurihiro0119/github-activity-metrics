package api

import (
	"net/http"
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
