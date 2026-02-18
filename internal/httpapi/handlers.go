package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	fabricgw "github.com/hyperledger/fabric-gateway/pkg/client"
	"golang-fabric-service/internal/fabric"
)

type Handler struct {
	gw  *fabric.Gateway
	cfg fabric.Config
}

func NewHandler(gw *fabric.Gateway, cfg fabric.Config) *Handler {
	return &Handler{gw: gw, cfg: cfg}
}

type TxReq struct {
	Args []string `json:"args"`
}

// POST /submit/:fn
func (h *Handler) Submit(c *gin.Context) {
	fn := c.Param("fn")

	var req TxReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	// endorse+submit timeout; do not wait for commit (fast pattern)
	timeout := time.Duration(h.cfg.EndorseTimeoutSec+h.cfg.SubmitTimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()

	contract := h.gw.Contract()

	// Submit async (no commit wait)
	res, commit, err := contract.SubmitAsyncWithContext(
		ctx,
		fn,
		fabricgw.WithArguments(req.Args...),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "PENDING",
		"txid":   commit.TransactionID(),
		"result": string(res),
	})
}

// POST /evaluate/:fn
func (h *Handler) Evaluate(c *gin.Context) {
	fn := c.Param("fn")

	var req TxReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	ctx, cancel := context.WithTimeout(
		c.Request.Context(),
		time.Duration(h.cfg.EvaluateTimeoutSec)*time.Second,
	)
	defer cancel()

	contract := h.gw.Contract()

	res, err := contract.EvaluateWithContext(
		ctx,
		fn,
		fabricgw.WithArguments(req.Args...),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "SUCCESS",
		"result": string(res),
	})
}
