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

// ConnectedClientListRequest represents the request for logging connected client list
type ConnectedClientListRequest struct {
	ClientList map[string]interface{} `json:"client_list" binding:"required"`
}

// logConnectedClientList logs the connected client list event to the ledger
func (h *NetworkHandler) logConnectedClientList(clientList map[string]interface{}) {
	contract := h.gw.Contract()
	submitter := fabric.NewTxSubmitter(contract)

	logData := map[string]interface{}{
		"client_list": clientList,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	logJSON, _ := json.Marshal(logData)

	result := submitter.SubmitWithOpts(
		context.Background(),
		"LogConnectedClientList",
		fabric.SubmitOpts{
			Mode: fabric.TxAsyncNoWait,
		},
		string(logJSON),
	)

	if result.Status == "FAILED" {
		// Log error, but since it's async, perhaps just print
		fmt.Printf("Failed to log connected client list: %s\n", result.Error)
	}
}

// GetConnectedClientList handles the connected client list logging request
func (h *NetworkHandler) GetConnectedClientList(c *gin.Context) {
	var req ConnectedClientListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Log the connected client list to the ledger
	h.logConnectedClientList(req.ClientList)

	c.JSON(http.StatusOK, gin.H{"message": "Connected client list logged successfully"})
}
