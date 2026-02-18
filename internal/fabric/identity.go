package fabric

import (
	"fmt"
	"os"

	"github.com/hyperledger/fabric-gateway/pkg/identity"
)

func loadIdentityAndSign(mspID, certPath, keyPath string) (*identity.X509Identity, identity.Sign, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read signcert: %w", err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read key: %w", err)
	}

	cert, err := identity.CertificateFromPEM(certPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("CertificateFromPEM: %w", err)
	}
	id, err := identity.NewX509Identity(mspID, cert)
	if err != nil {
		return nil, nil, fmt.Errorf("NewX509Identity: %w", err)
	}

	privKey, err := identity.PrivateKeyFromPEM(keyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("PrivateKeyFromPEM: %w", err)
	}
	sign, err := identity.NewPrivateKeySign(privKey)
	if err != nil {
		return nil, nil, fmt.Errorf("NewPrivateKeySign: %w", err)
	}

	return id, sign, nil
}
