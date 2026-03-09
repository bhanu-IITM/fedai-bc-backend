package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang-fabric-service/internal/fabric"

	"github.com/gin-gonic/gin"
)

// ShutdownSystemRequest represents the request for logging system shutdown
type ShutdownSystemRequest struct {
	ShutdownInfo map[string]interface{} `json:"shutdown_info" binding:"required"`
}

// logShutdownSystem logs the system shutdown event to the ledger
func (h *NetworkHandler) logShutdownSystem(shutdownInfo map[string]interface{}) {
	contract := h.gw.Contract()
	submitter := fabric.NewTxSubmitter(contract)

	logData := map[string]interface{}{
		"shutdown_info": shutdownInfo,
		"timestamp":     time.Now().Format(time.RFC3339),
	}

	logJSON, _ := json.Marshal(logData)

	result := submitter.SubmitWithOpts(
		context.Background(),
		"LogShutdownSystem",
		fabric.SubmitOpts{
			Mode: fabric.TxAsyncNoWait,
		},
		string(logJSON),
	)

	if result.Status == "FAILED" {
		// Log error, but since it's async, perhaps just print
		fmt.Printf("Failed to log shutdown system event: %s\n", result.Error)
	}
}

// ShutdownSystem handles the system shutdown event logging request
func (h *NetworkHandler) ShutdownSystem(c *gin.Context) {
	var req ShutdownSystemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Log the shutdown system event to the ledger
	h.logShutdownSystem(req.ShutdownInfo)

	c.JSON(http.StatusOK, gin.H{"message": "System shutdown event logged successfully"})
}
