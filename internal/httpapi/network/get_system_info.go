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

// SystemInfoRequest represents the request for logging system info
type SystemInfoRequest struct {
	SystemInfo map[string]interface{} `json:"system_info" binding:"required"`
}

// logSystemInfo logs the system info event to the ledger
func (h *NetworkHandler) logSystemInfo(systemInfo map[string]interface{}) {
	contract := h.gw.Contract()
	submitter := fabric.NewTxSubmitter(contract)

	logData := map[string]interface{}{
		"system_info": systemInfo,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	logJSON, _ := json.Marshal(logData)

	result := submitter.SubmitWithOpts(
		context.Background(),
		"LogSystemInfo",
		fabric.SubmitOpts{
			Mode: fabric.TxAsyncNoWait,
		},
		string(logJSON),
	)

	if result.Status == "FAILED" {
		// Log error, but since it's async, perhaps just print
		fmt.Printf("Failed to log system info: %s\n", result.Error)
	}
}

// GetSystemInfo handles the system info logging request
func (h *NetworkHandler) GetSystemInfo(c *gin.Context) {
	var req SystemInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Log the system info to the ledger
	h.logSystemInfo(req.SystemInfo)

	c.JSON(http.StatusOK, gin.H{"message": "System info logged successfully"})
}
