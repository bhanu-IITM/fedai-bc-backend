package jobs

import (
	"context"
	"encoding/json"
	"net/http"

	"golang-fabric-service/internal/fabric"

	"github.com/gin-gonic/gin"
)

type ListJobsHandler struct {
	gw *fabric.Gateway
}

func NewListJobsHandler(gw *fabric.Gateway) *ListJobsHandler {
	return &ListJobsHandler{gw: gw}
}

type JobInfo struct {
	JobID      string `json:"job_id"`
	JobName    string `json:"job_name"`
	Status     string `json:"status"`
	SubmitTime string `json:"submit_time"`
}

// POST /api/v1/jobs
// Receives NVFlare job list response and logs it to the ledger
func (h *ListJobsHandler) ListJobs(c *gin.Context) {
	var nvflareJobsList []JobInfo
	if err := c.ShouldBindJSON(&nvflareJobsList); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Convert the NVFlare jobs list response to JSON string for ledger storage
	jobsJSON, err := json.Marshal(nvflareJobsList)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal jobs list"})
		return
	}

	// Log to ledger via fabric gateway using TxSubmitter
	contract := h.gw.Contract()
	submitter := fabric.NewTxSubmitter(contract)

	result := submitter.SubmitWithOpts(
		context.Background(),
		"LogJobsList",
		fabric.SubmitOpts{
			Mode: fabric.TxAsyncNoWait,
		},
		string(jobsJSON),
	)

	if result.Status == "FAILED" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": result.Error,
		})
		return
	}

	// Return the result along with the NVFlare jobs list
	c.JSON(http.StatusOK, gin.H{
		"nvflare_jobs":  nvflareJobsList,
		"ledger_result": result,
	})
}
