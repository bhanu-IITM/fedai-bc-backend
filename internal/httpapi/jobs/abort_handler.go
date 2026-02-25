package jobs

import (
	"context"
	"encoding/json"
	"net/http"

	"golang-fabric-service/internal/fabric"

	"github.com/gin-gonic/gin"
)

type AbortHandler struct {
	gw *fabric.Gateway
}

func NewAbortHandler(gw *fabric.Gateway) *AbortHandler {
	return &AbortHandler{gw: gw}
}

type AbortJobResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// POST /api/v1/jobs/abort
// Receives NVFlare job abort confirmation and logs it to the ledger
func (h *AbortHandler) Abort(c *gin.Context) {
	var nvflareResponse AbortJobResponse
	if err := c.ShouldBindJSON(&nvflareResponse); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Convert the NVFlare abort response to JSON string for ledger storage
	abortJSON, err := json.Marshal(nvflareResponse)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal abort response"})
		return
	}

	// Log to ledger via fabric gateway using TxSubmitter
	contract := h.gw.Contract()
	submitter := fabric.NewTxSubmitter(contract)

	result := submitter.SubmitWithOpts(
		context.Background(),
		"LogJobAbort",
		fabric.SubmitOpts{
			Mode: fabric.TxAsyncNoWait,
		},
		string(abortJSON),
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
