package main

import (
	"log"
	"os"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

/* ------------------------------ Main ------------------------------ */

func main() {
	// Build contract chaincode (same as before)
	cc, err := contractapi.NewChaincode(&SmartContract{})
	if err != nil {
		log.Panicf("error creating chaincode: %v", err)
	}

	// CCaaS env vars (set by your docker run)
	ccid := os.Getenv("CHAINCODE_ID")             // usually PACKAGE_ID
	addr := os.Getenv("CHAINCODE_SERVER_ADDRESS") // e.g. 0.0.0.0:9999

	if ccid == "" {
		log.Panic("missing env CHAINCODE_ID")
	}
	if addr == "" {
		log.Panic("missing env CHAINCODE_SERVER_ADDRESS")
	}

	server := &shim.ChaincodeServer{
		CCID:    ccid,
		Address: addr,
		CC:      cc,
		TLSProps: shim.TLSProperties{
			Disabled: true, // keep false TLS for now (matches connection.json tls_required=false)
			// If later you enable TLS, you’ll set:
			// Disabled: false,
			// Key:  ..., Cert: ..., ClientCACerts: ...
		},
	}

	log.Printf("Starting CCaaS chaincode server: CCID=%s addr=%s", ccid, addr)
	if err := server.Start(); err != nil {
		log.Panicf("chaincode server failed to start: %v", err)
	}
}
