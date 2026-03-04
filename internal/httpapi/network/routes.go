package network

import (
	"github.com/gin-gonic/gin"
	"golang-fabric-service/internal/fabric"
)

func RegisterRoutes(r *gin.Engine, gw *fabric.Gateway) {
	networkHandler := NewNetworkHandler(gw)

	r.POST("/api/v1/network/shutdown/client/:client_name", networkHandler.ShutdownClient)
	
}