package network

import (
	"context"
	"encoding/json"
	"net/http"

	"golang-fabric-service/internal/fabric"

	"github.com/gin-gonic/gin"
)

type ShutdownHandler struct {
	gw *fabric.Gateway
}

func NewShutdownHandler(gw *fabric.Gateway) *ShutdownHandler {
	return &ShutdownHandler{gw: gw}
}

type ClientShutdownResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// POST /api/v1/network/shutdown/client/{client_name}
// Receives client shutdown notification and logs it to the ledger
func (h *ShutdownHandler) ShutdownClient(c *gin.Context) {
	clientName := c.Param("client_name")
	if clientName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client_name is required"})
		return
	}

	var clientResponse ClientShutdownResponse
	if err := c.ShouldBindJSON(&clientResponse); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Convert the client shutdown response to JSON string for ledger storage
	shutdownJSON, err := json.Marshal(clientResponse)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal shutdown response"})
		return
	}

	// Log to ledger via fabric gateway using TxSubmitter
	contract := h.gw.Contract()
	submitter := fabric.NewTxSubmitter(contract)

	result := submitter.SubmitWithOpts(
		context.Background(),
		"LogClientShutdown",
		fabric.SubmitOpts{
			Mode: fabric.TxAsyncNoWait,
		},
		string(shutdownJSON),
	)

	if result.Status == "FAILED" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": result.Error,
		})
		return
	}

	// Return the result along with the client response
	c.JSON(http.StatusOK, gin.H{
		"client_response": clientResponse,
		"ledger_result":   result,
	})
}
