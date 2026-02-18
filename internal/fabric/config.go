package fabric

import (
	"log"
	"os"
)

type Config struct {
	PeerEndpoint string
	TLSHostName  string
	MSPID        string
	Channel      string
	Chaincode    string

	SignCertPath string
	SignKeyPath  string
	TLSCAPath    string

	// timeouts (seconds)
	EvaluateTimeoutSec int
	EndorseTimeoutSec  int
	SubmitTimeoutSec   int
	CommitTimeoutSec   int
}

func MustLoadConfigFromEnv() Config {
	cfg := Config{
		PeerEndpoint: getenv("FABRIC_PEER_ENDPOINT", "127.0.0.1:7051"),
		TLSHostName:  getenv("FABRIC_TLS_HOSTNAME", "peer0.org1.example.com"),
		MSPID:        getenv("FABRIC_MSP_ID", "Org1MSP"),
		Channel:      getenv("FABRIC_CHANNEL", "mychannel"),
		Chaincode:    getenv("FABRIC_CHAINCODE", "poc"),

		SignCertPath: getenv("FABRIC_SIGNCERT", "./certs/org1/admin/signcert.pem"),
		SignKeyPath:  getenv("FABRIC_KEY", "./certs/org1/admin/key.pem"),
		TLSCAPath:    getenv("FABRIC_TLS_CA", "./certs/peer0/tls/ca.crt"),

		EvaluateTimeoutSec: atoi(getenv("HLF_TIMEOUT_EVALUATE", "10"), 10),
		EndorseTimeoutSec:  atoi(getenv("HLF_TIMEOUT_ENDORSE", "15"), 15),
		SubmitTimeoutSec:   atoi(getenv("HLF_TIMEOUT_SUBMIT", "30"), 30),
		CommitTimeoutSec:   atoi(getenv("HLF_TIMEOUT_COMMIT", "60"), 60),
	}

	// minimal validation
	if cfg.SignCertPath == "" || cfg.SignKeyPath == "" || cfg.TLSCAPath == "" {
		log.Fatal("missing cert/key paths")
	}
	return cfg
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func atoi(s string, d int) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return d
		}
		n = n*10 + int(ch-'0')
	}
	if n <= 0 {
		return d
	}
	return n
}
