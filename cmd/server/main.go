package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"golang-fabric-service/internal/fabric"
	"golang-fabric-service/internal/httpapi"
)

func main() {
	cfg := fabric.MustLoadConfigFromEnv()

	gw, err := fabric.NewGateway(cfg)
	if err != nil {
		log.Fatalf("gateway init failed: %v", err)
	}
	defer gw.Close()

	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	h := httpapi.NewHandler(gw, cfg)
	r.POST("/submit/:fn", h.Submit)
	r.POST("/eval/:fn", h.Evaluate)

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
