package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang-fabric-service/internal/fabric"

	"github.com/gin-gonic/gin"
)



// StatsRequest represents the request for logging job stats
type StatsRequest struct {
	JobID      string                 `json:"job_id" binding:"required"`
	TargetType string                 `json:"target_type" binding:"required"`
	Targets    []string               `json:"targets"`
	Stats      map[string]interface{} `json:"stats" binding:"required"`
}

// logStats logs the job stats event to the ledger
func (h *StatsHandler) logStats(jobID string, targetType string, targets []string, stats map[string]interface{}) {
	contract := h.gw.Contract()
	submitter := fabric.NewTxSubmitter(contract)

	logData := map[string]interface{}{
		"job_id":      jobID,
		"target_type": targetType,
		"targets":     targets,
		"stats":       stats,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	logJSON, _ := json.Marshal(logData)

	result := submitter.SubmitWithOpts(
		context.Background(),
		"LogStats",
		fabric.SubmitOpts{
			Mode: fabric.TxAsyncNoWait,
		},
		string(logJSON),
	)

	if result.Status == "FAILED" {
		// Log error, but since it's async, perhaps just print
		fmt.Printf("Failed to log stats: %s\n", result.Error)
	}
}

// ShowStats handles the job stats logging request
func (h *StatsHandler) ShowStats(c *gin.Context) {
	var req StatsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Log the job stats to the ledger
	h.logStats(req.JobID, req.TargetType, req.Targets, req.Stats)

	c.JSON(http.StatusOK, gin.H{"message": "Job stats logged successfully"})
}
