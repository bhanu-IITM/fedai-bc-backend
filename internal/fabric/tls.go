package fabric

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

func newGrpcTLSCredentials(tlsCaPath, tlsServerName string) (credentials.TransportCredentials, error) {
	caPEM, err := os.ReadFile(tlsCaPath)
	if err != nil {
		return nil, fmt.Errorf("read tls ca: %w", err)
	}

	cp := x509.NewCertPool()
	if ok := cp.AppendCertsFromPEM(caPEM); !ok {
		return nil, fmt.Errorf("failed to append tls ca pem")
	}

	tlsCfg := &tls.Config{
		RootCAs:    cp,
		ServerName: tlsServerName, // IMPORTANT: matches peer TLS cert SAN/CN
		MinVersion: tls.VersionTLS12,
	}
	return credentials.NewTLS(tlsCfg), nil
}
