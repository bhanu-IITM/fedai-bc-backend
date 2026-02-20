package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang-fabric-service/internal/ca"
)

type CAHospitalHandler struct {
	cfg ca.Config
}

func NewCAHospitalHandler(cfg ca.Config) *CAHospitalHandler {
	return &CAHospitalHandler{cfg: cfg}
}

type RegisterEnrollReq struct {
	Name string `json:"name"` // hospital_chennai
	// Optional metadata (not used by CA itself)
	Org string `json:"org,omitempty"` // apollo_ent (NVFlare org)
}

func (h *CAHospitalHandler) RegisterEnroll(c *gin.Context) {
	var req RegisterEnrollReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json: name required"})
		return
	}

	id, err := ca.RegisterAndEnrollHospital(h.cfg, req.Name, map[string]string{
		"role":    "hospital",
		"site_id": req.Name,
		// you can also store nvflare org as an attr if you want:
		// "nvflare_org": req.Org,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "FAILED",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status": "CREATED",
		"fabric": gin.H{
			"enroll_id":  id.EnrollID,
			"msp_dir":    id.MSPDir,
			"cert_path":  id.CertPath,
			"key_path":   id.KeyPath,
			"secret":     id.Secret, // optional, but useful for audit/debug
		},
		"nvflare": gin.H{
			"name": req.Name,
			"org":  req.Org,
		},
	})
}