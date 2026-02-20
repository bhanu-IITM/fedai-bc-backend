// ca/ca.go
package ca

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hyperledger/fabric-ca/api"
	"github.com/hyperledger/fabric-ca/lib"
	"github.com/hyperledger/fabric-ca/lib/tls"
)

type Config struct {
	CAURL        string
	CATLSCert    string
	AdminID      string
	AdminSecret  string
	MSPID        string
	BaseDir      string // e.g. /opt/golang-fabric-service/identities
}

func MustLoadFromEnv() Config {
	cfg := Config{
		CAURL:        mustEnv("FABRIC_CA_URL"),
		CATLSCert:    mustEnv("FABRIC_CA_TLS_CERT"),
		AdminID:      mustEnv("FABRIC_CA_ADMIN_ID"),
		AdminSecret:  mustEnv("FABRIC_CA_ADMIN_SECRET"),
		MSPID:        getenv("FABRIC_CA_MSP_ID", "Org1MSP"),
		BaseDir:      getenv("FABRIC_IDENTITY_BASEDIR", "/opt/golang-fabric-service/identities"),
	}
	return cfg
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		panic("missing env: " + k)
	}
	return v
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// Ensures CA admin is enrolled on disk so we can register new users.
func EnsureAdminEnrolled(cfg Config) error {
	adminMSPDir := filepath.Join(cfg.BaseDir, "_ca_admin", cfg.MSPID, "admin")

	// make sure target dirs exist
	if err := os.MkdirAll(filepath.Join(adminMSPDir, "keystore"), 0755); err != nil {
		return fmt.Errorf("mkdir admin keystore: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(adminMSPDir, "signcerts"), 0755); err != nil {
		return fmt.Errorf("mkdir admin signcerts: %w", err)
	}

	tlsCfg := &tls.ClientTLSConfig{
		Enabled:   true,
		CertFiles: []string{cfg.CATLSCert},
	}

	c := &lib.Client{
		HomeDir: cfg.BaseDir,
		Config: &lib.ClientConfig{
			URL:    cfg.CAURL,
			TLS:    *tlsCfg,
			MSPDir: adminMSPDir,
			Debug:  true,
			ID: api.RegistrationRequest{
				Name:   cfg.AdminID,
				Secret: cfg.AdminSecret,
			},
		},
	}
	if err := c.Init(); err != nil {
		return fmt.Errorf("ca init: %w", err)
	}

	// If already enrolled on disk, skip.
	keyDir := filepath.Join(adminMSPDir, "keystore")
	certFile := filepath.Join(adminMSPDir, "signcerts", "cert.pem")
	if fileExists(certFile) && dirHasFiles(keyDir) {
		return nil
	}

	// Enroll + explicitly store identity to disk
	enrollResp, err := c.Enroll(&api.EnrollmentRequest{
		Name:   cfg.AdminID,
		Secret: cfg.AdminSecret,
		Type:   "x509",
	})
	if err != nil {
		return fmt.Errorf("admin enroll: %w", err)
	}
	if enrollResp == nil || enrollResp.Identity == nil {
		return fmt.Errorf("admin enroll: empty enrollment response/identity")
	}
	if err := enrollResp.Identity.Store(); err != nil {
		return fmt.Errorf("admin store: %w", err)
	}

	return nil
}

type EnrolledIdentity struct {
	EnrollID string
	Secret   string
	MSPDir   string
	CertPath string
	KeyPath  string // first file inside keystore
}

// Register+Enroll a hospital identity and write MSP material on disk.
// attrs are optional CA attributes (stored as ecert attrs if ECert=true).
func RegisterAndEnrollHospital(cfg Config, siteID string, attrs map[string]string) (*EnrolledIdentity, error) {
	if err := EnsureAdminEnrolled(cfg); err != nil {
		return nil, err
	}

	siteID = sanitizeSiteID(siteID)
	if siteID == "" {
		return nil, fmt.Errorf("siteID is empty after sanitization")
	}

	adminMSPDir := filepath.Join(cfg.BaseDir, "_ca_admin", cfg.MSPID, "admin")
	adminKeyDir := filepath.Join(adminMSPDir, "keystore")
	adminCert := filepath.Join(adminMSPDir, "signcerts", "cert.pem")

	adminKey, err := firstFile(adminKeyDir)
	if err != nil {
		return nil, fmt.Errorf("admin key: %w", err)
	}
	if !fileExists(adminCert) {
		return nil, fmt.Errorf("admin cert missing: %s", adminCert)
	}

	tlsCfg := &tls.ClientTLSConfig{
		Enabled:   true,
		CertFiles: []string{cfg.CATLSCert},
	}

	// admin client used to load admin identity and register users
	adminClient := &lib.Client{
		HomeDir: cfg.BaseDir,
		Config: &lib.ClientConfig{
			URL:    cfg.CAURL,
			TLS:    *tlsCfg,
			MSPDir: adminMSPDir,
			Debug:  true,
		},
	}
	if err := adminClient.Init(); err != nil {
		return nil, fmt.Errorf("admin client init: %w", err)
	}

	adminIdentity, err := adminClient.LoadIdentity(adminKey, adminCert, "")
	if err != nil {
		return nil, fmt.Errorf("load admin identity: %w", err)
	}

	enrollID := "site-" + siteID
	secret := randomSecret(16)

	// build attrs as ecert attrs
	var attrList []api.Attribute
	for k, v := range attrs {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		attrList = append(attrList, api.Attribute{
			Name:  k,
			Value: v,
			ECert: true,
		})
	}

	regReq := &api.RegistrationRequest{
		Name:           enrollID,
		Type:           "client",
		Secret:         secret,
		MaxEnrollments: -1,
		// NOTE: Fabric-CA uses `Attrs` in api.RegistrationRequest for attributes.
		// (Some older forks used `Attributes`, but the upstream uses `Attrs`.)
		Attributes: attrList,
	}

	regResp, err := adminIdentity.Register(regReq)
	if err != nil {
		return nil, fmt.Errorf("register %s: %w", enrollID, err)
	}

	// enroll user to its MSP dir
	userMSPDir := filepath.Join(cfg.BaseDir, "hospitals", siteID, "msp")

	// Ensure expected MSP subdirs exist (optional, but avoids edge cases)
	_ = os.MkdirAll(filepath.Join(userMSPDir, "keystore"), 0755)
	_ = os.MkdirAll(filepath.Join(userMSPDir, "signcerts"), 0755)
	_ = os.MkdirAll(filepath.Join(userMSPDir, "cacerts"), 0755)

	userClient := &lib.Client{
		HomeDir: cfg.BaseDir,
		Config: &lib.ClientConfig{
			URL:    cfg.CAURL,
			TLS:    *tlsCfg,
			MSPDir: userMSPDir,
			Debug:  true,
			ID: api.RegistrationRequest{
				Name:   enrollID,
				Secret: regResp.Secret,
			},
		},
	}
	if err := userClient.Init(); err != nil {
		return nil, fmt.Errorf("user client init: %w", err)
	}

	enrollResp, err := userClient.Enroll(&api.EnrollmentRequest{
		Name:   enrollID,
		Secret: regResp.Secret,
		Type:   "x509",
	})
	if err != nil {
		return nil, fmt.Errorf("enroll %s: %w", enrollID, err)
	}
	if enrollResp == nil || enrollResp.Identity == nil {
		return nil, fmt.Errorf("enroll %s: empty enrollment response/identity", enrollID)
	}

	// persist to disk
	if err := enrollResp.Identity.Store(); err != nil {
		return nil, fmt.Errorf("store identity: %w", err)
	}

	certPath := filepath.Join(userMSPDir, "signcerts", "cert.pem")
	keyPath, err := firstFile(filepath.Join(userMSPDir, "keystore"))
	if err != nil {
		return nil, fmt.Errorf("user key: %w", err)
	}
	if !fileExists(certPath) {
		return nil, fmt.Errorf("expected cert missing after enroll: %s", certPath)
	}

	// Also write flat copies (handy for NVFlare kit or gateway usage)
	flatDir := filepath.Join(cfg.BaseDir, "hospitals", siteID)
	if err := os.MkdirAll(flatDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir flatDir: %w", err)
	}
	_ = copyFile(certPath, filepath.Join(flatDir, "signcert.pem"), 0644)
	_ = copyFile(keyPath, filepath.Join(flatDir, "key.pem"), 0600)

	return &EnrolledIdentity{
		EnrollID: enrollID,
		Secret:   regResp.Secret,
		MSPDir:   userMSPDir,
		CertPath: certPath,
		KeyPath:  keyPath,
	}, nil
}

func sanitizeSiteID(siteID string) string {
	s := strings.ToLower(strings.TrimSpace(siteID))
	if s == "" {
		return ""
	}
	// replace whitespace with dash
	s = strings.Join(strings.Fields(s), "-")
	// allow only [a-z0-9-_.]
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	out = strings.Trim(out, "-")
	return out
}

func randomSecret(nbytes int) string {
	b := make([]byte, nbytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func dirHasFiles(d string) bool {
	ents, err := os.ReadDir(d)
	return err == nil && len(ents) > 0
}

func firstFile(dir string) (string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range ents {
		if !e.IsDir() {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no file in %s", dir)
}

func copyFile(src, dst string, perm os.FileMode) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, perm)
}