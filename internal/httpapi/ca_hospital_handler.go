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

type MockHospital struct {
	Name string `json:"name"`
	Org  string `json:"org,omitempty"`
}

type RegisterEnrollReq struct {
	ProjectName   string         `json:"project_name"`
	MockHospitals []MockHospital `json:"mock_hospitals"`
}

func (h *CAHospitalHandler) RegisterEnroll(c *gin.Context) {
	var req RegisterEnrollReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ProjectName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json: project_name required"})
		return
	}
	if len(req.MockHospitals) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one mock_hospital is required"})
		return
	}

	for _, hospital := range req.MockHospitals {
		id, err := ca.RegisterAndEnrollHospital(h.cfg, hospital.Name, map[string]string{
			"role":    "hospital",
			"site_id": hospital.Name,
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
				"enroll_id": id.EnrollID,
				"msp_dir":   id.MSPDir,
				"cert_path": id.CertPath,
				"key_path":  id.KeyPath,
				"secret":    id.Secret,
			},
			"nvflare": gin.H{
				"name": hospital.Name,
				"org":  hospital.Org,
			},
		})
	}
}