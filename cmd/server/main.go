package main

import (
	"log"
	"os"

	"golang-fabric-service/internal/ca"
	"golang-fabric-service/internal/fabric"
	"golang-fabric-service/internal/httpapi"
	"golang-fabric-service/internal/httpapi/auth"
	"golang-fabric-service/internal/httpapi/jobs"
	"golang-fabric-service/internal/httpapi/network"
	"golang-fabric-service/internal/httpapi/stats"

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

	// Base API path - can be changed from environment variable or config
	apiBasePath := getenv("API_BASE_PATH", "/api/v1")

	h := httpapi.NewHandler(gw, cfg)
	r.POST("/api/submit/:fn", h.Submit)
	r.POST("/api/eval/:fn", h.Evaluate)

	// ---- Auth endpoints ----
	auth.RegisterRoutes(r, gw, caCfg, cfg, apiBasePath)

	// ---- Job endpoints ----
	jobs.RegisterRoutes(r, gw, apiBasePath)

	// ---- Network endpoints ----
	network.RegisterRoutes(r, gw, apiBasePath)

	// ---- Stats endpoints ----
	stats.RegisterRoutes(r, gw, apiBasePath)

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
