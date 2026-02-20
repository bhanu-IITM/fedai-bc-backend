package httpapi

import (
	"golang-fabric-service/internal/ca"
	// "golang-fabric-service/internal/fabric"
	"net/http"

	"github.com/gin-gonic/gin"
)



type RegisterHospitalReq struct {
	Name string `json:"name"` // hospital_chennai
	Org  string `json:"org"`  // apollo_ent (NVFlare org)
}

func (h *Handler) RegisterHospital(c *gin.Context) {
	var req RegisterHospitalReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.Org == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	// 1) Create Fabric CA identity (API method)
	caCfg := ca.MustLoadFromEnv()
	id, err := ca.RegisterAndEnrollHospital(caCfg, req.Name, map[string]string{
		"role":    "hospital",
		"site_id": req.Name,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// 2) OPTIONAL NOW: write to chaincode RegisterSite()
	//    Using your existing gateway service identity (admin) – Pattern A.
	//    You can add this next after we confirm CA identity generation works.
	//
	// siteJSON := fmt.Sprintf(`{"site_id":"%s","org_msp":"%s","jurisdiction":"IN","capabilities_hash":"","enrollment_cert_fingerprint":""}`, req.Name, h.cfg.MSPID)
	// _ = h.gw.SubmitAsync(ctx,"RegisterSite", siteJSON)

	c.JSON(200, gin.H{
		"status": "CREATED",
		"nvflare": gin.H{
			"name": req.Name,
			"org":  req.Org,
		},
		"fabric": gin.H{
			"enroll_id": id.EnrollID,
			"msp_dir":   id.MSPDir,
			"cert_path": id.CertPath,
			"key_path":  id.KeyPath,
		},
	})
}
