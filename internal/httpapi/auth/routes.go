package auth

import (
	"golang-fabric-service/internal/ca"
	"golang-fabric-service/internal/fabric"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all authentication-related routes
func RegisterRoutes(r *gin.Engine, gw *fabric.Gateway, caCfg ca.Config, cfg fabric.Config) {
	// CA Admin Handler
	adminHandler := NewCAAdminHandler(caCfg)
	r.POST("/api/v1/ca/enroll-admin", adminHandler.EnrollAdmin)

	// CA Hospital Handler
	caHospitalHandler := NewAuthHandler(caCfg, gw, cfg)
	r.POST("/api/v1/ca/register-enroll", caHospitalHandler.RegisterEnroll)

	// Login Handler
	loginHandler := NewLoginHandler(gw)
	r.POST("/api/v1/auth/login", loginHandler.Login)
	r.POST("/api/v1/auth/verify-token", loginHandler.VerifyToken)
}
