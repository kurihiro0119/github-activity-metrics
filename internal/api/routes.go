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
		// Organization endpoints
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

			// Rankings
			rankings := orgs.Group("/rankings")
			{
				rankings.GET("/members/:type", handler.GetMemberRanking)
				rankings.GET("/repos/:type", handler.GetRepoRanking)
			}
		}

		// User endpoints
		users := v1.Group("/users/:user")
		{
			// User metrics (same as org metrics, but for user account)
			users.GET("/metrics", handler.GetUserMetrics)
			users.GET("/metrics/timeseries", handler.GetUserTimeSeriesMetrics)

			// Repositories metrics
			repos := users.Group("/repos")
			{
				repos.GET("/metrics", handler.GetUserReposMetrics)
				repos.GET("/:repo/metrics", handler.GetUserRepoMetrics)
			}

			// Rankings
			rankings := users.Group("/rankings")
			{
				rankings.GET("/members/:type", handler.GetUserMemberRanking)
				rankings.GET("/repos/:type", handler.GetUserRepoRanking)
			}
		}
	}

	return router
}
