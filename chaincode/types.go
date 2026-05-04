package main

// IdentityBinding links Fabric MSP identity to application role
type IdentityBinding struct {
	ClientID           string   `json:"client_id"`           // Your app-level ID (e.g., "hospital_chennai_admin")
	MSPID              string   `json:"msp_id"`              // Fabric MSP (e.g., "Org1MSP")
	CertFingerprint    string   `json:"cert_fingerprint"`    // SHA-256 of X.509 cert
	Role               string   `json:"role"`                // "admin", "trainer", "auditor", "governance"
	SiteID             string   `json:"site_id,omitempty"`   // Link to your Site registry
	BoundAt            string   `json:"bound_at"`
	BoundByMSP         string   `json:"bound_by_msp"`        // Which MSP authorized this binding
	LastVerifiedAt     string   `json:"last_verified_at"`    // Last time cert was validated
	RevokedAt          string   `json:"revoked_at,omitempty"`
	Status             string   `json:"status"`              // "active", "revoked", "suspended"
}

// SessionNonce for replay protection (optional enhancement)
type SessionNonce struct {
	Nonce       string `json:"nonce"`
	ClientID    string `json:"client_id"`
	CreatedAt   string `json:"created_at"`
	ExpiresAt   string `json:"expires_at"`
	Used        bool   `json:"used"`
}