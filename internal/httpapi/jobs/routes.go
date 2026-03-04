package jobs

import (
	"golang-fabric-service/internal/fabric"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all job-related routes
func RegisterRoutes(r *gin.Engine, gw *fabric.Gateway) {
	jobHandler := NewJobHandler(gw)

	// Job submission endpoint
	r.POST("/api/v1/jobs/submit", jobHandler.Submit)

	// Job list endpoint
	r.POST("/api/v1/jobs", jobHandler.ListJobs)

	// Job status endpoint
	r.POST("/api/v1/jobs/status", jobHandler.GetStatus)

	// Job abort endpoint
	r.POST("/api/v1/jobs/abort", jobHandler.Abort)
}
