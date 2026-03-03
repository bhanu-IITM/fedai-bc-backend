package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang-fabric-service/internal/ca"
)

type CAAdminHandler struct {
	cfg ca.Config
}

func NewCAAdminHandler(cfg ca.Config) *CAAdminHandler {
	return &CAAdminHandler{cfg: cfg}
}

func (h *CAAdminHandler) EnrollAdmin(c *gin.Context) {

	err := ca.EnsureAdminEnrolled(h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "FAILED",
			"error":  err.Error(),
		})
		return
	}

	adminMSPDir := h.cfg.BaseDir + "/_ca_admin/" + h.cfg.MSPID + "/admin"

	c.JSON(http.StatusOK, gin.H{
		"status":     "SUCCESS",
		"admin_id":   h.cfg.AdminID,
		"msp_dir":    adminMSPDir,
		"cert_path":  adminMSPDir + "/signcerts/cert.pem",
		"key_dir":    adminMSPDir + "/keystore/",
	})
}