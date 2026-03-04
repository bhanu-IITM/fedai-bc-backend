package network

import (
	"golang-fabric-service/internal/fabric"
)

type NetworkHandler struct {
	gw *fabric.Gateway
}

func NewNetworkHandler(gw *fabric.Gateway) *NetworkHandler {
	return &NetworkHandler{gw: gw}
}