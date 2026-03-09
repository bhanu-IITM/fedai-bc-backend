package jobs

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang-fabric-service/internal/fabric"
)

// DELETE /api/v1/jobs/:jobId
// Deletes a job from the ledger based on the provided job ID
func (h *JobHandler) Delete(c *gin.Context) {
	jobId := c.Param("jobId")
	if jobId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jobId is required"})
		return
	}

	// Submit delete transaction to ledger via fabric gateway using TxSubmitter
	contract := h.gw.Contract()
	submitter := fabric.NewTxSubmitter(contract)

	result := submitter.SubmitWithOpts(
		context.Background(),
		"DeleteJob",
		fabric.SubmitOpts{
			Mode: fabric.TxAsyncNoWait,
		},
		jobId,
	)

	if result.Status == "FAILED" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Job deleted successfully",
		"ledger_result": result,
	})
}