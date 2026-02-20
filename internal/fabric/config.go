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

		SignCertPath: getenv("FABRIC_SIGNCERT", ""),
		SignKeyPath:  getenv("FABRIC_KEY", ""),
		TLSCAPath:    getenv("FABRIC_TLS_CA", ""),

		

		EvaluateTimeoutSec: atoi(getenv("HLF_TIMEOUT_EVALUATE", "10"), 10),
		EndorseTimeoutSec:  atoi(getenv("HLF_TIMEOUT_ENDORSE", "15"), 15),
		SubmitTimeoutSec:   atoi(getenv("HLF_TIMEOUT_SUBMIT", "30"), 30),
		CommitTimeoutSec:   atoi(getenv("HLF_TIMEOUT_COMMIT", "60"), 60),
	}

	// minimal validation
	if cfg.SignCertPath == "" {
	log.Fatal("FABRIC_SIGNCERT not set")
	}
	if cfg.SignKeyPath == "" {
		log.Fatal("FABRIC_KEY not set")
	}
	if cfg.TLSCAPath == "" {
		log.Fatal("FABRIC_TLS_CA not set")
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
