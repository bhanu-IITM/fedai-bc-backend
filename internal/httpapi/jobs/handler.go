package jobs

import (
	"golang-fabric-service/internal/fabric"
)

// JobHandler is the unified handler for all job-related operations
type JobHandler struct {
	gw *fabric.Gateway
}

// NewJobHandler creates a new JobHandler instance
func NewJobHandler(gw *fabric.Gateway) *JobHandler {
	return &JobHandler{gw: gw}
}
