package jobs

import (
	"context"
	"encoding/json"
	"net/http"

	"golang-fabric-service/internal/core/domain/repositories/nosql"
	"golang-fabric-service/internal/fabric"

	"github.com/gin-gonic/gin"
)

// POST /api/v1/jobs/submit
// Accepts the NVFlare API response and stores it to the ledger
func (h *JobHandler) Submit(c *gin.Context) {
	var nvflareResponse nosql.SubmitJobResponse
	if err := c.ShouldBindJSON(&nvflareResponse); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Convert the NVFlare response to JSON string for ledger storage
	responseJSON, err := json.Marshal(nvflareResponse)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal response"})
		return
	}

	// Submit to ledger via fabric gateway using TxSubmitter
	contract := h.gw.Contract()
	submitter := fabric.NewTxSubmitter(contract)

	result := submitter.SubmitWithOpts(
		context.Background(),
		"StoreJobSubmission",
		fabric.SubmitOpts{
			Mode: fabric.TxAsyncNoWait,
		},
		string(responseJSON),
	)

	if result.Status == "FAILED" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": result.Error,
		})
		return
	}

	// Return the result along with the NVFlare response
	c.JSON(http.StatusOK, gin.H{
		"nvflare_response": nvflareResponse,
		"ledger_result":    result,
	})
}
