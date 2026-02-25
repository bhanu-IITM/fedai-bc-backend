package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"golang-fabric-service/internal/fabric"

	"github.com/gin-gonic/gin"
	fabricgw "github.com/hyperledger/fabric-gateway/pkg/client"
)

type LoginHandler struct {
	gw *fabric.Gateway
}

func NewLoginHandler(gw *fabric.Gateway) *LoginHandler {
	return &LoginHandler{gw: gw}
}

type LoginRequest struct {
	ClientID string `json:"client_id" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Status  string `json:"status"`
	Token   string `json:"token"`
	Message string `json:"message"`
}

// Generate a random 8-digit token with mixed numbers and alphabets
func generateRandomToken(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// POST /api/v1/auth/login
// Validates client credentials and generates an access token
func (h *LoginHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client_id and password are required"})
		return
	}

	// Validate credentials against the ledger
	contract := h.gw.Contract()

	// Query the chaincode to validate login
	credentialsJSON := fmt.Sprintf(`{"client_id":"%s","password":"%s"}`, req.ClientID, req.Password)

	result, err := contract.EvaluateWithContext(
		context.Background(),
		"ValidateLogin",
		fabricgw.WithArguments(credentialsJSON),
	)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login validation failed: " + err.Error()})
		return
	}

	// Parse the validation response
	var validationResult map[string]interface{}
	if err := json.Unmarshal(result, &validationResult); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Check if validation passed
	valid, ok := validationResult["valid"].(bool)
	if !ok || !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid client_id or password"})
		return
	}

	// Generate a random token
	token := generateRandomToken(32)

	// Store the token in the ledger
	loginEventJSON := fmt.Sprintf(`{"client_id":"%s","token":"%s","login_at":"%s"}`,
		req.ClientID, token, time.Now().UTC().Format(time.RFC3339))

	submitter := fabric.NewTxSubmitter(contract)
	submitResult := submitter.SubmitWithOpts(
		context.Background(),
		"StoreLoginToken",
		fabric.SubmitOpts{
			Mode: fabric.TxAsyncNoWait,
		},
		loginEventJSON,
	)

	if submitResult.Status == "FAILED" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store token: " + submitResult.Error})
		return
	}

	response := LoginResponse{
		Status:  "success",
		Token:   token,
		Message: "Login successful. Use this token for NVFlare operations.",
	}

	c.JSON(http.StatusOK, response)
}

// POST /api/v1/auth/verify-token
// Verifies if a token is valid and active
func (h *LoginHandler) VerifyToken(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required in Authorization header"})
		return
	}

	// Remove "Bearer " prefix if present
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	// Query the chaincode to verify token
	contract := h.gw.Contract()

	result, err := contract.EvaluateWithContext(
		context.Background(),
		"VerifyLoginToken",
		fabricgw.WithArguments(token),
	)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token verification failed"})
		return
	}

	var verificationResult map[string]interface{}
	if err := json.Unmarshal(result, &verificationResult); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	valid, ok := verificationResult["valid"].(bool)
	if !ok || !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token is invalid or expired"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "valid",
		"token":   token,
		"message": "Token is valid",
	})
}
