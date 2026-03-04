package auth

import (
	"golang-fabric-service/internal/ca"
	"golang-fabric-service/internal/fabric"
)

type CAAdminHandler struct {
	cfg ca.Config
}

func NewCAAdminHandler(cfg ca.Config) *CAAdminHandler {
	return &CAAdminHandler{cfg: cfg}
}

type AuthHandler struct {
	caCfg ca.Config
	gw    *fabric.Gateway
	cfg   fabric.Config
}

func NewAuthHandler(caCfg ca.Config, gw *fabric.Gateway, cfg fabric.Config) *AuthHandler {
	return &AuthHandler{caCfg: caCfg, gw: gw, cfg: cfg}
}
