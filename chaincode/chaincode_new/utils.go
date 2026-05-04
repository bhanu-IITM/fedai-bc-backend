package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

/* ------------------------------- Key Design ----------------------------- */

func keySite(siteID string) string     { return "site:" + siteID }
func keyAtt(attID string) string       { return "att:" + attID }
func keyPolicy(policyID string) string { return "policy:" + policyID }
func keyJob(jobID string) string       { return "job:" + jobID }

// func keyParticipants(jobID string) string { return "participants:" + jobID }
func keyGrant(grantID string) string { return "grant:" + grantID }

// func keyRound(jobID string, roundID int) string {
// 	return fmt.Sprintf("round:%s:%d", jobID, roundID)
// }
// func keyUpdate(updateID string) string { return "update:" + updateID }
// func keyCommit(jobID string, roundID int) string {
// 	return fmt.Sprintf("commit:%s:%d", jobID, roundID)
// }
// func keyArtifact(artifactID string) string { return "artifact:" + artifactID }

/* ------------------------------ Helpers --------------------------------- */

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func parseRFC3339(ts string) (time.Time, error) {
	return time.Parse(time.RFC3339, ts)
}

func (s *SmartContract) getClientMSP(ctx contractapi.TransactionContextInterface) (string, error) {
	return ctx.GetClientIdentity().GetMSPID()
}

// Platform admin MSP can be configured by env var at chaincode container runtime.
// For PoC, default is Org1MSP.
func platformAdminMSP() string {
	if v := strings.TrimSpace(os.Getenv("PLATFORM_ADMIN_MSP")); v != "" {
		return v
	}
	return "Org1MSP"
}

func requirePlatform(ctx contractapi.TransactionContextInterface) error {
	msp, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return err
	}
	if msp != platformAdminMSP() {
		return fmt.Errorf("access denied: requires platform admin MSP (%s)", platformAdminMSP())
	}
	return nil
}

func putJSON(ctx contractapi.TransactionContextInterface, k string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState(k, b)
}

func getJSON[T any](ctx contractapi.TransactionContextInterface, k string, out *T) (bool, error) {
	b, err := ctx.GetStub().GetState(k)
	if err != nil {
		return false, err
	}
	if b == nil {
		return false, nil
	}
	return true, json.Unmarshal(b, out)
}

func mustNonEmpty(field, name string) error {
	if strings.TrimSpace(field) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func emit(ctx contractapi.TransactionContextInterface, name string, payload any) {
	b, _ := json.Marshal(payload)
	_ = ctx.GetStub().SetEvent(name, b)
}

func marshal(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func unmarshal(in string, out any) error {
	return json.Unmarshal([]byte(in), out)
}
