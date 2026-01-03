package api

import (
	"github.com/gin-gonic/gin"
)

// SetupRoutes sets up the API routes
func SetupRoutes(handler *Handler) *gin.Engine {
	router := gin.New()

	// Middleware
	router.Use(Recovery())
	router.Use(CORS())
	router.Use(gin.Logger())

	// Health check
	router.GET("/health", handler.HealthCheck)

	// API v1
	v1 := router.Group("/api/v1")
	{
		orgs := v1.Group("/orgs/:org")
		{
			// Organization metrics
			orgs.GET("/metrics", handler.GetOrgMetrics)
			orgs.GET("/metrics/timeseries", handler.GetTimeSeriesMetrics)

			// Members metrics
			members := orgs.Group("/members")
			{
				members.GET("/metrics", handler.GetMembersMetrics)
				members.GET("/:member/metrics", handler.GetMemberMetrics)
			}

			// Repositories metrics
			repos := orgs.Group("/repos")
			{
				repos.GET("/metrics", handler.GetReposMetrics)
				repos.GET("/:repo/metrics", handler.GetRepoMetrics)
			}
		}
	}

	return router
}
