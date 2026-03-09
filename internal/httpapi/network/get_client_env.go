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

// ClientEnvRequest represents the request for logging client environment
type ClientEnvRequest struct {
	ClientEnvs map[string]interface{} `json:"client_envs" binding:"required"`
}

// logClientEnv logs the client environment event to the ledger
func (h *NetworkHandler) logClientEnv(clientEnvs map[string]interface{}) {
	contract := h.gw.Contract()
	submitter := fabric.NewTxSubmitter(contract)

	logData := map[string]interface{}{
		"client_envs": clientEnvs,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	logJSON, _ := json.Marshal(logData)

	result := submitter.SubmitWithOpts(
		context.Background(),
		"LogClientEnv",
		fabric.SubmitOpts{
			Mode: fabric.TxAsyncNoWait,
		},
		string(logJSON),
	)

	if result.Status == "FAILED" {
		// Log error, but since it's async, perhaps just print
		fmt.Printf("Failed to log client env: %s\n", result.Error)
	}
}

// GetClientEnv handles the client environment logging request
func (h *NetworkHandler) GetClientEnv(c *gin.Context) {
	var req ClientEnvRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Log the client environments to the ledger
	h.logClientEnv(req.ClientEnvs)

	c.JSON(http.StatusOK, gin.H{"message": "Client environment logged successfully"})
}
