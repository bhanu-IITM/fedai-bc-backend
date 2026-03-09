package jobs

import (
	"context"
	"encoding/json"
	"net/http"

	"golang-fabric-service/internal/core/domain/repositories/nosql"
	"golang-fabric-service/internal/fabric"

	"github.com/gin-gonic/gin"
)

// POST /api/v1/jobs/status
// Receives NVFlare job status update and logs it to the ledger
func (h *JobHandler) GetStatus(c *gin.Context) {
	var nvflareStatus nosql.JobStatusResponse
	if err := c.ShouldBindJSON(&nvflareStatus); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Convert the NVFlare status response to JSON string for ledger storage
	statusJSON, err := json.Marshal(nvflareStatus)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal status"})
		return
	}

	// Log to ledger via fabric gateway using TxSubmitter
	contract := h.gw.Contract()
	submitter := fabric.NewTxSubmitter(contract)

	result := submitter.SubmitWithOpts(
		context.Background(),
		"LogJobStatus",
		fabric.SubmitOpts{
			Mode: fabric.TxAsyncNoWait,
		},
		string(statusJSON),
	)

	if result.Status == "FAILED" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": result.Error,
		})
		return
	}

	// Return the result along with the NVFlare status
	c.JSON(http.StatusOK, gin.H{
		"nvflare_status": nvflareStatus,
		"ledger_result":  result,
	})
}
