package auth

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"golang-fabric-service/internal/ca"
	"golang-fabric-service/internal/core/domain/repositories/nosql"
	"golang-fabric-service/internal/fabric"
)

func (h *AuthHandler) RegisterEnroll(c *gin.Context) {
	var req nosql.RegisterEnrollReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ProjectName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json: project_name required"})
		return
	}
	if len(req.MockHospitals) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one mock_hospital is required"})
		return
	}

	contract := h.gw.Contract()
	submitter := fabric.NewTxSubmitter(contract)

	// endorse+submit timeout (commit timeout is configured at gateway connect time)
	esTimeout := time.Duration(h.cfg.EndorseTimeoutSec+h.cfg.SubmitTimeoutSec) * time.Second

	results := make([]nosql.RegisterEnrollResult, 0, len(req.MockHospitals))

	for _, hospital := range req.MockHospitals {
		var res nosql.RegisterEnrollResult
		res.NVFlare.Name = hospital.Name
		res.NVFlare.Org = hospital.Org

		// 0) Generate login credentials
		clientID := hospital.Name
		password := generateRandomPassword(8)
		res.Auth.ClientID = clientID
		res.Auth.Password = password

		// 1) CA register+enroll
		id, err := ca.RegisterAndEnrollHospital(h.caCfg, hospital.Name, map[string]string{
			"role":    "hospital",
			"site_id": hospital.Name,
		})
		if err != nil {
			res.Status = "FAILED"
			res.Ledger.Error = fmt.Sprintf("CA register/enroll failed: %v", err)
			results = append(results, res)
			continue
		}

		res.Fabric.EnrollID = id.EnrollID
		res.Fabric.MSPDir = id.MSPDir
		res.Fabric.CertPath = id.CertPath
		res.Fabric.KeyPath = id.KeyPath
		res.Fabric.Secret = id.Secret

		// 2) Fingerprint cert
		fp, err := certFingerprintSHA256(id.CertPath)
		if err != nil {
			res.Status = "FAILED"
			res.Ledger.Error = fmt.Sprintf("cert fingerprint failed: %v", err)
			results = append(results, res)
			continue
		}
		res.Fabric.Fingerprint = fp

		// 3) Build Site JSON for chaincode RegisterSite(siteJSON string)
		site := map[string]any{
			"site_id":                     hospital.Name,
			"org_msp":                     h.caCfg.MSPID, // typically Org1MSP
			"enrollment_cert_fingerprint": fp,
		}
		siteJSON, _ := json.Marshal(site)

		mode := fabric.TxAsyncNoWait
		if req.WaitForCommit {
			mode = fabric.TxAsyncWaitCommit
		}

		tx := submitter.SubmitWithOpts(
			c.Request.Context(),
			"RegisterSite",
			fabric.SubmitOpts{
				Mode:                 mode,
				EndorseSubmitTimeout: esTimeout,
			},
			string(siteJSON),
		)

		// Map SubmitResult -> handler response
		res.Ledger.TxID = tx.TxID

		if tx.Status == "FAILED" {
			res.Status = "FAILED"
			res.Ledger.Anchored = false
			res.Ledger.Committed = false
			res.Ledger.Error = tx.Error
			results = append(results, res)
			continue
		}

		// Tx submitted successfully => anchored=true (it reached orderer)
		res.Ledger.Anchored = true

		if tx.Status == "COMMITTED" {
			res.Status = "CREATED"
			res.Ledger.Committed = true

			// Optional: read back ledger record
			if b, e := contract.EvaluateTransaction("GetSite", hospital.Name); e == nil {
				res.Ledger.Record = string(b)
			}
		} else {
			// PENDING (async no-wait OR commit not confirmed within gateway commit timeout)
			res.Status = "PENDING"
			res.Ledger.Committed = false
			if tx.Error != "" {
				res.Ledger.Error = tx.Error // e.g., "commit not confirmed"
			}
		}

		// Store login credentials in chaincode (regardless of RegisterSite commit status)
		// Credentials storage is independent and should work in both sync and async modes
		credentialsJSON, _ := json.Marshal(map[string]string{
			"client_id": res.Auth.ClientID,
			"password":  res.Auth.Password,
		})
		credTx := submitter.SubmitWithOpts(
			c.Request.Context(),
			"StoreLoginCredentials",
			fabric.SubmitOpts{
				Mode:                 mode,
				EndorseSubmitTimeout: esTimeout,
			},
			string(credentialsJSON),
		)
		if credTx.Status == "FAILED" {
			if res.Ledger.Error == "" {
				res.Ledger.Error = fmt.Sprintf("credential storage failed: %s", credTx.Error)
			} else {
				res.Ledger.Error = res.Ledger.Error + "; credential storage failed: " + credTx.Error
			}
		}

		results = append(results, res)
	}

	// Check if any registration failed
	hasFailures := false
	for _, res := range results {
		if res.Status == "FAILED" {
			hasFailures = true
			break
		}
	}

	// Return appropriate status code
	statusCode := http.StatusOK
	if hasFailures {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, gin.H{
		"status":  "DONE",
		"project": req.ProjectName,
		"results": results,
	})
}

// generateRandomPassword generates an 8-character password with mixed numbers and alphabets
func generateRandomPassword(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// certFingerprintSHA256 returns "sha256:<hex>" over certificate DER bytes.
func certFingerprintSHA256(certPath string) (string, error) {
	pemBytes, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("read cert: %w", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return "", fmt.Errorf("invalid PEM certificate at %s", certPath)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse cert: %w", err)
	}
	sum := sha256.Sum256(cert.Raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
