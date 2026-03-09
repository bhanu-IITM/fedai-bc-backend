package stats

import (
	"golang-fabric-service/internal/fabric"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, gw *fabric.Gateway, basePath string) {
	statsHandler := NewStatsHandler(gw)

	// Job stats logging endpoint
	r.POST(basePath+"/stats/show", statsHandler.ShowStats)
}
