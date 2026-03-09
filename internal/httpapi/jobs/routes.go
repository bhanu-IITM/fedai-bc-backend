package jobs

import (
	"golang-fabric-service/internal/fabric"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all job-related routes
func RegisterRoutes(r *gin.Engine, gw *fabric.Gateway, basePath string) {
	jobHandler := NewJobHandler(gw)

	// Job submission endpoint
	r.POST(basePath+"/jobs/submit", jobHandler.Submit)

	// Job list endpoint
	r.POST(basePath+"/jobs", jobHandler.ListJobs)

	// Job status endpoint
	r.POST(basePath+"/jobs/status", jobHandler.GetStatus)

	// Job abort endpoint
	r.POST(basePath+"/jobs/abort", jobHandler.Abort)

	// Job delete endpoint
	r.DELETE(basePath+"/jobs/:jobId", jobHandler.Delete)

	// Job monitor endpoint
	r.POST(basePath+"/jobs/monitor", jobHandler.Monitor)
}
