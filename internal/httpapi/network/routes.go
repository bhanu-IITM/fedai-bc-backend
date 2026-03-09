package network

import (
	"golang-fabric-service/internal/fabric"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, gw *fabric.Gateway, basePath string) {
	networkHandler := NewNetworkHandler(gw)

	r.POST(basePath+"/network/shutdown/client/:client_name", networkHandler.ShutdownClient)

	// System info logging endpoint
	r.POST(basePath+"/network/system-info", networkHandler.GetSystemInfo)
	// Connected client list logging endpoint
	r.POST(basePath+"/network/connected-clients", networkHandler.GetConnectedClientList)
	// Client environment logging endpoint
	r.POST(basePath+"/network/client-env", networkHandler.GetClientEnv)
	// System shutdown logging endpoint
	r.POST(basePath+"/network/shutdown-system", networkHandler.ShutdownSystem)

}
