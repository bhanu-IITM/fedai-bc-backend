package main

import (
	"log"
	"os"

	"golang-fabric-service/internal/ca"
	"golang-fabric-service/internal/fabric"
	"golang-fabric-service/internal/httpapi"
	"golang-fabric-service/internal/httpapi/jobs"
	"golang-fabric-service/internal/httpapi/network"
	"golang-fabric-service/internal/httpapi/auth"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := fabric.MustLoadConfigFromEnv()

	gw, err := fabric.NewGateway(cfg)
	if err != nil {
		log.Fatalf("gateway init failed: %v", err)
	}
	defer gw.Close()

	// ---- CA config ----
	caCfg := ca.MustLoadFromEnv()

	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	h := httpapi.NewHandler(gw, cfg)
	r.POST("/api/submit/:fn", h.Submit)
	r.POST("/api/eval/:fn", h.Evaluate)



	// ---- CA endpoints ----
	adminHandler := auth.NewCAAdminHandler(caCfg)
	r.POST("/api/ca/enroll-admin", adminHandler.EnrollAdmin)

	//Using this endpoint once the identity generation works, we can also add the chaincode call to
	//RegisterSite() using the admin identity (Pattern A) before returning response.
	caHospitalHandler := auth.NewCAHospitalHandler(caCfg, gw, cfg)
	r.POST("/api/ca/register-enroll", caHospitalHandler.RegisterEnroll)

	// ---- Job submission endpoints ----
	jobHandler := jobs.NewSubmitJobHandler(gw)
	r.POST("/api/v1/jobs/submit", jobHandler.Submit)

	// ---- Job list endpoints ----
	listHandler := jobs.NewListJobsHandler(gw)
	r.POST("/api/v1/jobs", listHandler.ListJobs)

	// ---- Job status endpoints ----
	statusHandler := jobs.NewStatusHandler(gw)
	r.POST("/api/v1/jobs/status", statusHandler.GetStatus)

	// ---- Job abort endpoints ----
	abortHandler := jobs.NewAbortHandler(gw)
	r.POST("/api/v1/jobs/abort", abortHandler.Abort)

	// ---- Network shutdown endpoints ----
	shutdownHandler := network.NewShutdownHandler(gw)
	r.POST("/api/v1/network/shutdown/client/:client_name", shutdownHandler.ShutdownClient)

	// ---- Authentication endpoints ----
	loginHandler := auth.NewLoginHandler(gw)
	r.POST("/api/v1/auth/login", loginHandler.Login)
	r.POST("/api/v1/auth/verify-token", loginHandler.VerifyToken)

	

	addr := getenv("HTTP_ADDR", "127.0.0.1:8080")
	log.Printf("Fabric Gateway Service running on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
