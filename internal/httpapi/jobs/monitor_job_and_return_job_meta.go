package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang-fabric-service/internal/fabric"

	"github.com/gin-gonic/gin"
)

// MonitorRequest represents the request for logging a monitor event
type MonitorRequest struct {
	JobID      string            `json:"job_id" binding:"required"`
	ReturnCode MonitorReturnCode `json:"return_code"`
	JobMeta    JobMeta           `json:"job_meta"`
}

// MonitorReturnCode represents the result of monitoring
type MonitorReturnCode int

const (
	JobFinished MonitorReturnCode = iota
	Timeout
	EndedByCB
)

// JobMeta represents the job metadata
type JobMeta map[string]interface{}

// logMonitorEvent logs the monitoring event to the ledger
func (h *JobHandler) logMonitorEvent(jobID string, event string, jobMeta JobMeta) {
	contract := h.gw.Contract()
	submitter := fabric.NewTxSubmitter(contract)

	logData := map[string]interface{}{
		"job_id":    jobID,
		"event":     event,
		"timestamp": time.Now().Format(time.RFC3339),
		"job_meta":  jobMeta,
	}

	logJSON, _ := json.Marshal(logData)

	result := submitter.SubmitWithOpts(
		context.Background(),
		"LogJobMonitor",
		fabric.SubmitOpts{
			Mode: fabric.TxAsyncNoWait,
		},
		string(logJSON),
	)

	if result.Status == "FAILED" {
		// Log error, but since it's async, perhaps just print
		fmt.Printf("Failed to log monitor event: %s\n", result.Error)
	}
}

// Monitor handles the job monitoring event logging request
func (h *JobHandler) Monitor(c *gin.Context) {
	var req MonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Determine event type based on return code
	var event string
	switch req.ReturnCode {
	case JobFinished:
		event = "JOB_FINISHED"
	case Timeout:
		event = "TIMEOUT"
	case EndedByCB:
		event = "ENDED_BY_CB"
	default:
		event = "UNKNOWN"
	}

	// Log the monitor event to the ledger
	h.logMonitorEvent(req.JobID, event, req.JobMeta)

	c.JSON(http.StatusOK, gin.H{"message": "Monitor event logged successfully"})
}
