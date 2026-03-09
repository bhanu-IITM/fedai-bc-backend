package stats

import (
	"golang-fabric-service/internal/fabric"
)

type StatsHandler struct {
	gw *fabric.Gateway
}

func NewStatsHandler(gw *fabric.Gateway) *StatsHandler {
	return &StatsHandler{gw: gw}
}