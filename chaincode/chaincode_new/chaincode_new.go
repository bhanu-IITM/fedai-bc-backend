package main

import (
	"fmt"
	"strings"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

/* ============================== A) Site Lifecycle ============================== */

// RegisterSite(siteJSON)   ------> DONE in chaincode, called from CA handler after hospital enrollment
func (s *SmartContract) RegisterSite(ctx contractapi.TransactionContextInterface, siteJSON string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}

	var site Site
	if err := unmarshal(siteJSON, &site); err != nil {
		return err
	}
	if err := mustNonEmpty(site.SiteID, "site_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(site.OrgMSP, "org_msp"); err != nil {
		return err
	}

	var existing Site
	ok, err := getJSON(ctx, keySite(site.SiteID), &existing)
	if err != nil {
		return err
	}

	ts := nowRFC3339()
	if !ok {
		site.Status = SiteActive
		site.CreatedAt = ts
	} else {
		// preserve CreatedAt if present
		site.CreatedAt = existing.CreatedAt
		// preserve status unless explicitly set
		if site.Status == "" {
			site.Status = existing.Status
		}
	}
	site.UpdatedAt = ts

	if err := putJSON(ctx, keySite(site.SiteID), site); err != nil {
		return err
	}
	emit(ctx, "SiteRegistered", site)
	return nil
}

// AttestSite(attestationJSON)
func (s *SmartContract) AttestSite(ctx contractapi.TransactionContextInterface, attestationJSON string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	var a SiteAttestation
	if err := unmarshal(attestationJSON, &a); err != nil {
		return err
	}
	if err := mustNonEmpty(a.AttestationID, "attestation_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(a.SiteID, "site_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(a.EvidenceHash, "evidence_hash"); err != nil {
		return err
	}

	// Ensure site exists
	var site Site
	ok, err := getJSON(ctx, keySite(a.SiteID), &site)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("site not found: %s", a.SiteID)
	}

	msp, _ := s.getClientMSP(ctx)
	a.AttestedByMSP = msp
	if strings.TrimSpace(a.AttestedAt) == "" {
		a.AttestedAt = nowRFC3339()
	}

	if err := putJSON(ctx, keyAtt(a.AttestationID), a); err != nil {
		return err
	}

	// Index: att~site(siteID, attestationID)
	idx, _ := ctx.GetStub().CreateCompositeKey("att~site", []string{a.SiteID, a.AttestationID})
	_ = ctx.GetStub().PutState(idx, []byte{0x00})

	emit(ctx, "SiteAttested", a)
	return nil
}

// SuspendSite(site_id, reason)
func (s *SmartContract) SuspendSite(ctx contractapi.TransactionContextInterface, siteID, reason string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	var site Site
	ok, err := getJSON(ctx, keySite(siteID), &site)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("site not found: %s", siteID)
	}
	site.Status = SiteSuspended
	site.UpdatedAt = nowRFC3339()
	if err := putJSON(ctx, keySite(siteID), site); err != nil {
		return err
	}
	emit(ctx, "SiteSuspended", map[string]any{"site_id": siteID, "reason": reason, "at": nowRFC3339()})
	return nil
}

// RevokeSite(site_id, reason)
func (s *SmartContract) RevokeSite(ctx contractapi.TransactionContextInterface, siteID, reason string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	var site Site
	ok, err := getJSON(ctx, keySite(siteID), &site)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("site not found: %s", siteID)
	}
	site.Status = SiteRevoked
	site.UpdatedAt = nowRFC3339()
	if err := putJSON(ctx, keySite(siteID), site); err != nil {
		return err
	}
	emit(ctx, "SiteRevoked", map[string]any{"site_id": siteID, "reason": reason, "at": nowRFC3339()})
	return nil
}

// GetIdentityBinding(client_id) - Retrieve binding details for authorization
// func (s *SmartContract) GetIdentityBinding(ctx contractapi.TransactionContextInterface, clientID string) (string, error) {
// 	key := fmt.Sprintf("identity:binding:%s", clientID)
// 	var binding IdentityBinding
// 	ok, err := getJSON(ctx, key, &binding)
// 	if err != nil {
// 		return "", err
// 	}
// 	if !ok {
// 		return "", fmt.Errorf("identity not found: %s", clientID)
// 	}
// 	return marshal(binding)
// }

// ListIdentitiesForMSP(msp_id) - List all identities for an organization: Administrative Query (e.g., for governance dashboard)
// func (s *SmartContract) ListIdentitiesForMSP(ctx contractapi.TransactionContextInterface, mspID string) (string, error) {
// 	iter, err := ctx.GetStub().GetStateByPartialCompositeKey("identity~msp", []string{mspID})
// 	if err != nil {
// 		return "", err
// 	}
// 	defer iter.Close()

// 	var bindings []IdentityBinding
// 	for iter.HasNext() {
// 		kv, _ := iter.Next()
// 		_, parts, _ := ctx.GetStub().SplitCompositeKey(kv.Key)
// 		if len(parts) != 2 {
// 			continue
// 		}
// 		clientID := parts[1]

// 		var binding IdentityBinding
// 		ok, err := getJSON(ctx, fmt.Sprintf("identity:binding:%s", clientID), &binding)
// 		if err != nil || !ok {
// 			continue
// 		}
// 		bindings = append(bindings, binding)
// 	}
// 	return marshal(bindings)
// }

/* ========================= B) Policy Lifecycle (Governance) ========================= */

// RegisterPolicy(policyJSON)
func (s *SmartContract) RegisterPolicy(ctx contractapi.TransactionContextInterface, policyJSON string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	var p Policy
	if err := unmarshal(policyJSON, &p); err != nil {
		return err
	}
	if err := mustNonEmpty(p.PolicyID, "policy_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(p.PolicyHash, "policy_hash"); err != nil {
		return err
	}
	msp, _ := s.getClientMSP(ctx)
	p.IssuerMSP = msp
	if p.Status == "" {
		p.Status = PolicyDraft
	}
	if strings.TrimSpace(p.CreatedAt) == "" {
		p.CreatedAt = nowRFC3339()
	}

	if err := putJSON(ctx, keyPolicy(p.PolicyID), p); err != nil {
		return err
	}
	emit(ctx, "PolicyRegistered", p)
	return nil
}

// ActivatePolicy(policy_id)
func (s *SmartContract) ActivatePolicy(ctx contractapi.TransactionContextInterface, policyID string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	var p Policy
	ok, err := getJSON(ctx, keyPolicy(policyID), &p)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("policy not found: %s", policyID)
	}
	p.Status = PolicyActive
	if err := putJSON(ctx, keyPolicy(policyID), p); err != nil {
		return err
	}
	emit(ctx, "PolicyActivated", p)
	return nil
}

// DeprecatePolicy(policy_id)
func (s *SmartContract) DeprecatePolicy(ctx contractapi.TransactionContextInterface, policyID string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	var p Policy
	ok, err := getJSON(ctx, keyPolicy(policyID), &p)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("policy not found: %s", policyID)
	}
	p.Status = PolicyDeprecated
	if err := putJSON(ctx, keyPolicy(policyID), p); err != nil {
		return err
	}
	emit(ctx, "PolicyDeprecated", p)
	return nil
}

/* ============================== C) Federation Job Lifecycle ============================== */

/* ==================== D) Training Authorization (Access-Grant APIs) ==================== */

// GrantTrainingAccess(grantJSON)
func (s *SmartContract) GrantTrainingAccess(ctx contractapi.TransactionContextInterface, grantJSON string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	var g TrainingAccessGrant
	if err := unmarshal(grantJSON, &g); err != nil {
		return err
	}
	if err := mustNonEmpty(g.GrantID, "grant_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(g.JobID, "job_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(g.SiteID, "site_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(g.PolicyID, "policy_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(g.PolicyHash, "policy_hash"); err != nil {
		return err
	}
	if err := mustNonEmpty(g.ValidFrom, "valid_from"); err != nil {
		return err
	}
	if err := mustNonEmpty(g.ValidTo, "valid_to"); err != nil {
		return err
	}

	// Validate time window
	vf, err := parseRFC3339(g.ValidFrom)
	if err != nil {
		return fmt.Errorf("valid_from must be RFC3339: %v", err)
	}
	vt, err := parseRFC3339(g.ValidTo)
	if err != nil {
		return fmt.Errorf("valid_to must be RFC3339: %v", err)
	}
	if !vt.After(vf) {
		return fmt.Errorf("valid_to must be after valid_from")
	}

	// Ensure site exists and active
	var site Site
	ok, err := getJSON(ctx, keySite(g.SiteID), &site)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("site not found: %s", g.SiteID)
	}
	if site.Status != SiteActive {
		return fmt.Errorf("site not ACTIVE: %s", g.SiteID)
	}

	// Ensure job exists + policy matches
	var job FederationJob
	ok, err = getJSON(ctx, keyJob(g.JobID), &job)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("job not found: %s", g.JobID)
	}
	if job.PolicyID != g.PolicyID || job.PolicyHash != g.PolicyHash {
		return fmt.Errorf("policy mismatch (job vs grant)")
	}

	// Ensure policy exists and hash matches
	var pol Policy
	ok, err = getJSON(ctx, keyPolicy(g.PolicyID), &pol)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("policy not found: %s", g.PolicyID)
	}
	if pol.PolicyHash != g.PolicyHash {
		return fmt.Errorf("policy hash mismatch (policy record vs grant)")
	}

	msp, _ := s.getClientMSP(ctx)
	g.OrgMSP = site.OrgMSP
	g.IssuedByMSP = msp
	g.IssuedAt = nowRFC3339()
	g.Status = GrantActive

	// Store grant
	if err := putJSON(ctx, keyGrant(g.GrantID), g); err != nil {
		return err
	}

	// Composite keys for fast lookup:
	// grant~job(jobID, grantID)
	idx1, _ := ctx.GetStub().CreateCompositeKey("grant~job", []string{g.JobID, g.GrantID})
	_ = ctx.GetStub().PutState(idx1, []byte{0x00})
	// grant~job~site(jobID, siteID, grantID)
	idx2, _ := ctx.GetStub().CreateCompositeKey("grant~job~site", []string{g.JobID, g.SiteID, g.GrantID})
	_ = ctx.GetStub().PutState(idx2, []byte{0x00})

	emit(ctx, "GrantIssued", map[string]any{"grant_id": g.GrantID, "job_id": g.JobID, "site_id": g.SiteID})
	return nil
}

// VerifyTrainingEligibility(job_id, site_id, at_time)
func (s *SmartContract) VerifyTrainingEligibility(ctx contractapi.TransactionContextInterface, jobID, siteID, atTime string) (string, error) {
	now, err := parseRFC3339(atTime)
	if err != nil {
		return "", fmt.Errorf("at_time must be RFC3339: %v", err)
	}

	// Site must exist + ACTIVE
	var site Site
	ok, err := getJSON(ctx, keySite(siteID), &site)
	if err != nil {
		return "", err
	}
	if !ok {
		dec := EligibilityDecision{JobID: jobID, SiteID: siteID, Decision: "DENY", Reason: "SITE_NOT_FOUND", CheckedAt: atTime}
		emit(ctx, "EligibilityChecked", dec)
		return marshal(dec)
	}
	if site.Status != SiteActive {
		dec := EligibilityDecision{JobID: jobID, SiteID: siteID, Decision: "DENY", Reason: "SITE_NOT_ACTIVE", CheckedAt: atTime}
		emit(ctx, "EligibilityChecked", dec)
		return marshal(dec)
	}

	// Job must exist
	var job FederationJob
	ok, err = getJSON(ctx, keyJob(jobID), &job)
	if err != nil {
		return "", err
	}
	if !ok {
		dec := EligibilityDecision{JobID: jobID, SiteID: siteID, Decision: "DENY", Reason: "JOB_NOT_FOUND", CheckedAt: atTime}
		emit(ctx, "EligibilityChecked", dec)
		return marshal(dec)
	}

	// Policy must exist; for PoC: allow even if DRAFT, but prefer ACTIVE for new jobs
	var pol Policy
	ok, err = getJSON(ctx, keyPolicy(job.PolicyID), &pol)
	if err != nil {
		return "", err
	}
	if !ok {
		dec := EligibilityDecision{JobID: jobID, SiteID: siteID, Decision: "DENY", Reason: "POLICY_NOT_FOUND", CheckedAt: atTime}
		emit(ctx, "EligibilityChecked", dec)
		return marshal(dec)
	}
	if pol.PolicyHash != job.PolicyHash {
		dec := EligibilityDecision{JobID: jobID, SiteID: siteID, Decision: "DENY", Reason: "POLICY_HASH_MISMATCH", CheckedAt: atTime}
		emit(ctx, "EligibilityChecked", dec)
		return marshal(dec)
	}

	// Find first ACTIVE, in-window grant for (job, site)
	iter, err := ctx.GetStub().GetStateByPartialCompositeKey("grant~job~site", []string{jobID, siteID})
	if err != nil {
		return "", err
	}
	defer iter.Close()

	for iter.HasNext() {
		kv, _ := iter.Next()
		_, parts, _ := ctx.GetStub().SplitCompositeKey(kv.Key)
		if len(parts) != 3 {
			continue
		}
		grantID := parts[2]
		var g TrainingAccessGrant
		ok, err := getJSON(ctx, keyGrant(grantID), &g)
		if err != nil || !ok {
			continue
		}

		// status check
		if g.Status == GrantRevoked {
			continue
		}
		// window check
		vf, err1 := parseRFC3339(g.ValidFrom)
		vt, err2 := parseRFC3339(g.ValidTo)
		if err1 != nil || err2 != nil {
			continue
		}
		if now.Before(vf) || now.After(vt) {
			continue
		}
		// policy binding check
		if g.PolicyHash != job.PolicyHash || g.JobID != jobID || g.SiteID != siteID {
			continue
		}
		// site MSP binding check
		if g.OrgMSP != site.OrgMSP {
			continue
		}

		dec := EligibilityDecision{JobID: jobID, SiteID: siteID, Decision: "ALLOW", Reason: "OK", GrantID: grantID, CheckedAt: atTime}
		emit(ctx, "EligibilityChecked", dec)
		return marshal(dec)
	}

	dec := EligibilityDecision{JobID: jobID, SiteID: siteID, Decision: "DENY", Reason: "NO_ACTIVE_GRANT", CheckedAt: atTime}
	emit(ctx, "EligibilityChecked", dec)
	return marshal(dec)
}

// RenewTrainingAccess(grant_id, new_valid_to, renewal_hash)
func (s *SmartContract) RenewTrainingAccess(ctx contractapi.TransactionContextInterface, grantID, newValidTo, renewalHash string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	var g TrainingAccessGrant
	ok, err := getJSON(ctx, keyGrant(grantID), &g)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("grant not found: %s", grantID)
	}
	if g.Status == GrantRevoked {
		return fmt.Errorf("grant is revoked")
	}

	oldTo, err := parseRFC3339(g.ValidTo)
	if err != nil {
		return fmt.Errorf("stored grant valid_to invalid: %v", err)
	}
	newTo, err := parseRFC3339(newValidTo)
	if err != nil {
		return fmt.Errorf("new_valid_to must be RFC3339: %v", err)
	}
	if !newTo.After(oldTo) {
		return fmt.Errorf("new_valid_to must extend the existing window")
	}

	g.ValidTo = newValidTo
	g.RenewalHash = renewalHash
	g.Status = GrantActive

	if err := putJSON(ctx, keyGrant(grantID), g); err != nil {
		return err
	}
	emit(ctx, "GrantRenewed", map[string]any{"grant_id": grantID, "new_valid_to": newValidTo})
	return nil
}

// RevokeTrainingAccess(grant_id, reason)
func (s *SmartContract) RevokeTrainingAccess(ctx contractapi.TransactionContextInterface, grantID, reason string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	var g TrainingAccessGrant
	ok, err := getJSON(ctx, keyGrant(grantID), &g)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("grant not found: %s", grantID)
	}
	g.Status = GrantRevoked
	g.RevokedReason = reason
	if err := putJSON(ctx, keyGrant(grantID), g); err != nil {
		return err
	}
	emit(ctx, "GrantRevoked", map[string]any{"grant_id": grantID, "reason": reason})
	return nil
}

/* ========================== E) Round + Provenance (Audit Spine) ========================== */

/* ============================== F) Queries (minimum set) ============================== */

// GetSite(site_id)
func (s *SmartContract) GetSite(ctx contractapi.TransactionContextInterface, siteID string) (string, error) {
	var site Site
	ok, err := getJSON(ctx, keySite(siteID), &site)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("site not found: %s", siteID)
	}
	return marshal(site)
}

// GetPolicy(policy_id)
func (s *SmartContract) GetPolicy(ctx contractapi.TransactionContextInterface, policyID string) (string, error) {
	var p Policy
	ok, err := getJSON(ctx, keyPolicy(policyID), &p)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("policy not found: %s", policyID)
	}
	return marshal(p)
}

// GetJob(job_id)
func (s *SmartContract) GetJob(ctx contractapi.TransactionContextInterface, jobID string) (string, error) {
	var j FederationJob
	ok, err := getJSON(ctx, keyJob(jobID), &j)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("job not found: %s", jobID)
	}
	return marshal(j)
}

// GetGrant(grant_id)
func (s *SmartContract) GetGrant(ctx contractapi.TransactionContextInterface, grantID string) (string, error) {
	var g TrainingAccessGrant
	ok, err := getJSON(ctx, keyGrant(grantID), &g)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("grant not found: %s", grantID)
	}
	return marshal(g)
}

// ListGrantsForJob(job_id)
func (s *SmartContract) ListGrantsForJob(ctx contractapi.TransactionContextInterface, jobID string) (string, error) {
	iter, err := ctx.GetStub().GetStateByPartialCompositeKey("grant~job", []string{jobID})
	if err != nil {
		return "", err
	}
	defer iter.Close()

	var grants []TrainingAccessGrant
	for iter.HasNext() {
		kv, _ := iter.Next()
		_, parts, _ := ctx.GetStub().SplitCompositeKey(kv.Key)
		if len(parts) != 2 {
			continue
		}
		grantID := parts[1]
		var g TrainingAccessGrant
		ok, err := getJSON(ctx, keyGrant(grantID), &g)
		if err != nil || !ok {
			continue
		}
		grants = append(grants, g)
	}
	return marshal(grants)
}

/* ============================== G) Logging/Audit Functions (NVFlare Integration) ============================== */

// StoreJobSubmission(jobSubmissionJSON)
// Logs the NVFlare job submission response to the ledger
func (s *SmartContract) StoreJobSubmission(ctx contractapi.TransactionContextInterface, jobSubmissionJSON string) error {
	var submission map[string]interface{}
	if err := unmarshal(jobSubmissionJSON, &submission); err != nil {
		return err
	}

	// Extract job_id if available
	jobID := ""
	if jID, ok := submission["job_id"].(string); ok {
		jobID = jID
	}

	ts := nowRFC3339()
	// Key format: nvflare:joblogs:jobid:timestamp for easy indexing
	key := fmt.Sprintf("nvflare:joblog:%s:%s", jobID, ts)

	if err := putJSON(ctx, key, submission); err != nil {
		return err
	}
	emit(ctx, "NVFlareJobSubmissionStored", submission)
	return nil
}

// LogJobStatus(jobStatusJSON)
// Logs the NVFlare job status update to the ledger
func (s *SmartContract) LogJobStatus(ctx contractapi.TransactionContextInterface, jobStatusJSON string) error {
	var status map[string]interface{}
	if err := unmarshal(jobStatusJSON, &status); err != nil {
		return err
	}

	// Extract job_id if available
	jobID := ""
	if jID, ok := status["job_id"].(string); ok {
		jobID = jID
	}

	ts := nowRFC3339()
	key := fmt.Sprintf("nvflare:jobstatus:%s:%s", jobID, ts)

	if err := putJSON(ctx, key, status); err != nil {
		return err
	}
	emit(ctx, "NVFlareJobStatusLogged", status)
	return nil
}

// LogJobAbort(abortResponseJSON)
// Logs the NVFlare job abort event to the ledger
func (s *SmartContract) LogJobAbort(ctx contractapi.TransactionContextInterface, abortResponseJSON string) error {
	var abort map[string]interface{}
	if err := unmarshal(abortResponseJSON, &abort); err != nil {
		return err
	}

	ts := nowRFC3339()
	key := fmt.Sprintf("nvflare:jobabort:%s", ts)

	if err := putJSON(ctx, key, abort); err != nil {
		return err
	}
	emit(ctx, "NVFlareJobAbortLogged", abort)
	return nil
}

// LogJobMonitor(monitorResponseJSON)
// Logs the NVFlare job monitor event to the ledger
func (s *SmartContract) LogJobMonitor(ctx contractapi.TransactionContextInterface, monitorResponseJSON string) error {
	var monitor map[string]interface{}
	if err := unmarshal(monitorResponseJSON, &monitor); err != nil {
		return err
	}

	ts := nowRFC3339()
	key := fmt.Sprintf("nvflare:jobmonitor:%s", ts)

	if err := putJSON(ctx, key, monitor); err != nil {
		return err
	}
	emit(ctx, "NVFlareJobMonitorLogged", monitor)
	return nil
}

// LogClientShutdown(shutdownResponseJSON)
// Logs the client graceful shutdown event to the ledger
func (s *SmartContract) LogClientShutdown(ctx contractapi.TransactionContextInterface, shutdownResponseJSON string) error {
	var shutdown map[string]interface{}
	if err := unmarshal(shutdownResponseJSON, &shutdown); err != nil {
		return err
	}

	ts := nowRFC3339()
	key := fmt.Sprintf("nvflare:clientshutdown:%s", ts)

	if err := putJSON(ctx, key, shutdown); err != nil {
		return err
	}
	emit(ctx, "NVFlareClientShutdownLogged", shutdown)
	return nil
}

// LogSystemInfo(systemInfoJSON)
// Logs the NVFlare system info event to the ledger
func (s *SmartContract) LogSystemInfo(ctx contractapi.TransactionContextInterface, systemInfoJSON string) error {
	var systemInfo map[string]interface{}
	if err := unmarshal(systemInfoJSON, &systemInfo); err != nil {
		return err
	}

	ts := nowRFC3339()
	key := fmt.Sprintf("nvflare:systeminfo:%s", ts)

	if err := putJSON(ctx, key, systemInfo); err != nil {
		return err
	}
	emit(ctx, "NVFlareSystemInfoLogged", systemInfo)
	return nil
}

// LogConnectedClientList(clientListJSON)
// Logs the NVFlare connected client list event to the ledger
func (s *SmartContract) LogConnectedClientList(ctx contractapi.TransactionContextInterface, clientListJSON string) error {
	var clientList map[string]interface{}
	if err := unmarshal(clientListJSON, &clientList); err != nil {
		return err
	}

	ts := nowRFC3339()
	key := fmt.Sprintf("nvflare:connectedclientlist:%s", ts)

	if err := putJSON(ctx, key, clientList); err != nil {
		return err
	}
	emit(ctx, "NVFlareConnectedClientListLogged", clientList)
	return nil
}

// LogClientEnv(clientEnvJSON)
// Logs the NVFlare client environment event to the ledger
func (s *SmartContract) LogClientEnv(ctx contractapi.TransactionContextInterface, clientEnvJSON string) error {
	var clientEnv map[string]interface{}
	if err := unmarshal(clientEnvJSON, &clientEnv); err != nil {
		return err
	}

	ts := nowRFC3339()
	key := fmt.Sprintf("nvflare:clientenv:%s", ts)

	if err := putJSON(ctx, key, clientEnv); err != nil {
		return err
	}
	emit(ctx, "NVFlareClientEnvLogged", clientEnv)
	return nil
}

// LogShutdownSystem(shutdownSystemJSON)
// Logs the NVFlare system shutdown event to the ledger
func (s *SmartContract) LogShutdownSystem(ctx contractapi.TransactionContextInterface, shutdownSystemJSON string) error {
	var shutdownSystem map[string]interface{}
	if err := unmarshal(shutdownSystemJSON, &shutdownSystem); err != nil {
		return err
	}

	ts := nowRFC3339()
	key := fmt.Sprintf("nvflare:shutdownsystem:%s", ts)

	if err := putJSON(ctx, key, shutdownSystem); err != nil {
		return err
	}
	emit(ctx, "NVFlareShutdownSystemLogged", shutdownSystem)
	return nil
}

// LogStats(statsJSON)
// Logs the NVFlare job stats event to the ledger
func (s *SmartContract) LogStats(ctx contractapi.TransactionContextInterface, statsJSON string) error {
	var stats map[string]interface{}
	if err := unmarshal(statsJSON, &stats); err != nil {
		return err
	}

	ts := nowRFC3339()
	key := fmt.Sprintf("nvflare:stats:%s", ts)

	if err := putJSON(ctx, key, stats); err != nil {
		return err
	}
	emit(ctx, "NVFlareStatsLogged", stats)
	return nil
}

// LogJobsList(jobsListJSON)
// Logs the NVFlare jobs list response to the ledger
func (s *SmartContract) LogJobsList(ctx contractapi.TransactionContextInterface, jobsListJSON string) error {
	var jobsList interface{}
	if err := unmarshal(jobsListJSON, &jobsList); err != nil {
		return err
	}

	ts := nowRFC3339()
	key := fmt.Sprintf("nvflare:jobslist:%s", ts)

	if err := putJSON(ctx, key, jobsList); err != nil {
		return err
	}
	emit(ctx, "NVFlareJobsListLogged", jobsList)
	return nil
}

/* ============================== H) Authentication Functions (Login/Token) ============================== */

// StoreLoginCredentials(credentialsJSON)
// Stores client login credentials (client_id and password) for future authentication
func (s *SmartContract) StoreLoginCredentials(ctx contractapi.TransactionContextInterface, credentialsJSON string) error {
	var creds map[string]interface{}
	if err := unmarshal(credentialsJSON, &creds); err != nil {
		return err
	}

	clientID := ""
	if cID, ok := creds["client_id"].(string); ok {
		clientID = cID
	}
	if clientID == "" {
		return fmt.Errorf("client_id is required")
	}

	key := fmt.Sprintf("auth:credentials:%s", clientID)
	if err := putJSON(ctx, key, creds); err != nil {
		return err
	}
	emit(ctx, "LoginCredentialsStored", map[string]any{"client_id": clientID})
	return nil
}

//This function can replace StoreLoginCredentials : Register MSP → Role mapping
// StoreIdentityBinding(bindingJSON) - Platform admin registers who can do what

// func (s *SmartContract) StoreIdentityBinding(ctx contractapi.TransactionContextInterface, bindingJSON string) error {
// 	if err := requirePlatform(ctx); err != nil {
// 		return err
// 	}

// 	var binding IdentityBinding
// 	if err := unmarshal(bindingJSON, &binding); err != nil {
// 		return err
// 	}
// 	if err := mustNonEmpty(binding.ClientID, "client_id"); err != nil {
// 		return err
// 	}
// 	if err := mustNonEmpty(binding.MSPID, "msp_id"); err != nil {
// 		return err
// 	}
// 	if err := mustNonEmpty(binding.Role, "role"); err != nil {
// 		return err
// 	}

// 	// Validate role
// 	validRoles := map[string]bool{"admin": true, "trainer": true, "auditor": true, "governance": true}
// 	if !validRoles[binding.Role] {
// 		return fmt.Errorf("invalid role: %s", binding.Role)
// 	}

// 	// If site-specific role, verify site exists
// 	if binding.SiteID != "" {
// 		var site Site
// 		ok, err := getJSON(ctx, keySite(binding.SiteID), &site)
// 		if err != nil || !ok {
// 			return fmt.Errorf("site not found: %s", binding.SiteID)
// 		}
// 		// Verify MSP matches site's MSP
// 		if site.OrgMSP != binding.MSPID {
// 			return fmt.Errorf("MSP mismatch: site %s has MSP %s, binding has %s",
// 				binding.SiteID, site.OrgMSP, binding.MSPID)
// 		}
// 	}

// 	callerMSP, _ := s.getClientMSP(ctx)
// 	binding.BoundAt = nowRFC3339()
// 	binding.BoundByMSP = callerMSP
// 	binding.Status = "active"
// 	binding.LastVerifiedAt = binding.BoundAt

// 	// If cert fingerprint not provided, extract from current caller (bootstrap)
// 	if binding.CertFingerprint == "" {
// 		cert, err := ctx.GetClientIdentity().GetX509Certificate()
// 		if err != nil {
// 			return fmt.Errorf("failed to get caller certificate: %v", err)
// 		}
// 		fp := sha256.Sum256(cert.Raw)
// 		binding.CertFingerprint = hex.EncodeToString(fp[:])
// 	}

// 	key := fmt.Sprintf("identity:binding:%s", binding.ClientID)
// 	if err := putJSON(ctx, key, binding); err != nil {
// 		return err
// 	}

// 	// Index: identity~msp(msp_id, client_id)
// 	idx, _ := ctx.GetStub().CreateCompositeKey("identity~msp", []string{binding.MSPID, binding.ClientID})
// 	_ = ctx.GetStub().PutState(idx, []byte{0x00})

// 	// Index: identity~site(site_id, client_id) if site-specific
// 	if binding.SiteID != "" {
// 		idx2, _ := ctx.GetStub().CreateCompositeKey("identity~site", []string{binding.SiteID, binding.ClientID})
// 		_ = ctx.GetStub().PutState(idx2, []byte{0x00})
// 	}

// 	emit(ctx, "IdentityBound", map[string]any{
// 		"client_id": binding.ClientID,
// 		"msp_id": binding.MSPID,
// 		"role": binding.Role,
// 		"site_id": binding.SiteID,
// 	})
// 	return nil
// }

// ValidateLogin(loginJSON)
// Validates client credentials against stored credentials
// Returns JSON with {valid: true/false}
func (s *SmartContract) ValidateLogin(ctx contractapi.TransactionContextInterface, loginJSON string) (string, error) {
	var login map[string]interface{}
	if err := unmarshal(loginJSON, &login); err != nil {
		return marshal(map[string]any{"valid": false, "error": "invalid request"})
	}

	clientID, ok := login["client_id"].(string)
	if !ok || clientID == "" {
		return marshal(map[string]any{"valid": false, "error": "client_id required"})
	}

	password, ok := login["password"].(string)
	if !ok || password == "" {
		return marshal(map[string]any{"valid": false, "error": "password required"})
	}

	// Retrieve stored credentials
	key := fmt.Sprintf("auth:credentials:%s", clientID)
	var storedCreds map[string]interface{}
	ok, err := getJSON(ctx, key, &storedCreds)
	if err != nil {
		return marshal(map[string]any{"valid": false, "error": "lookup failed"})
	}
	if !ok {
		return marshal(map[string]any{"valid": false, "error": "client not found"})
	}

	// Compare password
	storedPassword, ok := storedCreds["password"].(string)
	if !ok || storedPassword != password {
		return marshal(map[string]any{"valid": false, "error": "invalid credentials"})
	}

	return marshal(map[string]any{"valid": true})
}

// This function can replace ValidateLogin : VerifyIdentity(client_id) - Confirms tx caller matches registered identity
// VerifyIdentity(client_id) - Confirms tx caller matches registered identity

// func (s *SmartContract) VerifyIdentity(ctx contractapi.TransactionContextInterface, clientID string) (string, error) {
// 	if clientID == "" {
// 		return "", fmt.Errorf("client_id required")
// 	}

// 	// Get registered binding
// 	key := fmt.Sprintf("identity:binding:%s", clientID)
// 	var binding IdentityBinding
// 	ok, err := getJSON(ctx, key, &binding)
// 	if err != nil {
// 		return "", err
// 	}
// 	if !ok {
// 		return marshal(map[string]any{
// 			"valid": false,
// 			"error": "identity not registered",
// 		})
// 	}

// 	// Check status
// 	if binding.Status != "active" {
// 		return marshal(map[string]any{
// 			"valid": false,
// 			"error": fmt.Sprintf("identity status: %s", binding.Status),
// 		})
// 	}

// 	// Get current caller's MSP
// 	callerMSP, err := s.getClientMSP(ctx)
// 	if err != nil {
// 		return "", err
// 	}

// 	// Verify MSP matches
// 	if binding.MSPID != callerMSP {
// 		return marshal(map[string]any{
// 			"valid": false,
// 			"error": "MSP mismatch",
// 			"expected_msp": binding.MSPID,
// 			"actual_msp": callerMSP,
// 		})
// 	}

// 	// Optional: Verify certificate fingerprint matches (strong binding)
// 	cert, err := ctx.GetClientIdentity().GetX509Certificate()
// 	if err == nil {
// 		fp := sha256.Sum256(cert.Raw)
// 		callerFP := hex.EncodeToString(fp[:])

// 		if binding.CertFingerprint != "" && binding.CertFingerprint != callerFP {
// 			return marshal(map[string]any{
// 				"valid": false,
// 				"error": "certificate fingerprint mismatch - possible credential theft",
// 			})
// 		}
// 	}

// 	// Update last verified
// 	binding.LastVerifiedAt = nowRFC3339()
// 	_ = putJSON(ctx, key, binding) // Best effort update

// 	return marshal(map[string]any{
// 		"valid": true,
// 		"client_id": binding.ClientID,
// 		"msp_id": binding.MSPID,
// 		"role": binding.Role,
// 		"site_id": binding.SiteID,
// 	})
// }

// StoreLoginToken(tokenJSON)
// Stores a login token for a client
func (s *SmartContract) StoreLoginToken(ctx contractapi.TransactionContextInterface, tokenJSON string) error {
	var tokenData map[string]interface{}
	if err := unmarshal(tokenJSON, &tokenData); err != nil {
		return err
	}

	token, ok := tokenData["token"].(string)
	if !ok || token == "" {
		return fmt.Errorf("token is required")
	}

	clientID, ok := tokenData["client_id"].(string)
	if !ok || clientID == "" {
		return fmt.Errorf("client_id is required")
	}

	// Key format: auth:token:{token}
	key := fmt.Sprintf("auth:token:%s", token)
	if err := putJSON(ctx, key, tokenData); err != nil {
		return err
	}

	// Also add for quick lookup by client_id: auth:token~client:{client_id}:{token}
	idx, _ := ctx.GetStub().CreateCompositeKey("auth:token~client", []string{clientID, token})
	_ = ctx.GetStub().PutState(idx, []byte{0x00})

	emit(ctx, "LoginTokenStored", map[string]any{"client_id": clientID, "token": token})
	return nil
}

// This function can replace StoreLoginToken : GenerateSessionNonce(client_id) - GenerateSessionNonce creates ephemeral, expiring challenges for critical operations to prevent replay attacks and ensure the caller is the legitimate client
// GenerateSessionNonce(client_id) - Creates one-time nonce for critical operations

// func (s *SmartContract) GenerateSessionNonce(ctx contractapi.TransactionContextInterface, clientID string) (string, error) {
// 	// First verify identity is valid
// 	verifyJSON, err := s.VerifyIdentity(ctx, clientID)
// 	if err != nil {
// 		return "", err
// 	}
// 	var verifyResult map[string]interface{}
// 	if err := unmarshal(verifyJSON, &verifyResult); err != nil {
// 		return "", err
// 	}
// 	if valid, _ := verifyResult["valid"].(bool); !valid {
// 		return "", fmt.Errorf("identity verification failed")
// 	}

// 	// Generate nonce
// 	nonceBytes := make([]byte, 32)
// 	rand.Read(nonceBytes)
// 	nonce := hex.EncodeToString(nonceBytes)

// 	now := time.Now().UTC()
// 	session := SessionNonce{
// 		Nonce:     nonce,
// 		ClientID:  clientID,
// 		CreatedAt: now.Format(time.RFC3339),
// 		ExpiresAt: now.Add(5 * time.Minute).Format(time.RFC3339),
// 		Used:      false,
// 	}

// 	key := fmt.Sprintf("session:nonce:%s", nonce)
// 	if err := putJSON(ctx, key, session); err != nil {
// 		return "", err
// 	}

// 	return marshal(map[string]any{
// 		"nonce": nonce,
// 		"expires_at": session.ExpiresAt,
// 	})
// }

// // VerifyIdentity(client_id) - Confirms tx caller matches registered identity
// func (s *SmartContract) VerifyIdentity(ctx contractapi.TransactionContextInterface, clientID string) (string, error) {
// 	if clientID == "" {
// 		return "", fmt.Errorf("client_id required")
// 	}

// 	// Get registered binding
// 	key := fmt.Sprintf("identity:binding:%s", clientID)
// 	var binding IdentityBinding
// 	ok, err := getJSON(ctx, key, &binding)
// 	if err != nil {
// 		return "", err
// 	}
// 	if !ok {
// 		return marshal(map[string]any{
// 			"valid": false,
// 			"error": "identity not registered",
// 		})
// 	}

// 	// Check status
// 	if binding.Status != "active" {
// 		return marshal(map[string]any{
// 			"valid": false,
// 			"error": fmt.Sprintf("identity status: %s", binding.Status),
// 		})
// 	}

// 	// Get current caller's MSP
// 	callerMSP, err := s.getClientMSP(ctx)
// 	if err != nil {
// 		return "", err
// 	}

// 	// Verify MSP matches
// 	if binding.MSPID != callerMSP {
// 		return marshal(map[string]any{
// 			"valid": false,
// 			"error": "MSP mismatch",
// 			"expected_msp": binding.MSPID,
// 			"actual_msp": callerMSP,
// 		})
// 	}

// 	// Optional: Verify certificate fingerprint matches (strong binding)
// 	cert, err := ctx.GetClientIdentity().GetX509Certificate()
// 	if err == nil {
// 		fp := sha256.Sum256(cert.Raw)
// 		callerFP := hex.EncodeToString(fp[:])

// 		if binding.CertFingerprint != "" && binding.CertFingerprint != callerFP {
// 			return marshal(map[string]any{
// 				"valid": false,
// 				"error": "certificate fingerprint mismatch - possible credential theft",
// 			})
// 		}
// 	}

// 	// Update last verified
// 	binding.LastVerifiedAt = nowRFC3339()
// 	_ = putJSON(ctx, key, binding) // Best effort update

// 	return marshal(map[string]any{
// 		"valid": true,
// 		"client_id": binding.ClientID,
// 		"msp_id": binding.MSPID,
// 		"role": binding.Role,
// 		"site_id": binding.SiteID,
// 	})
// }

// VerifyLoginToken(token)
// Verifies if a token is valid
// Returns JSON with {valid: true/false}
func (s *SmartContract) VerifyLoginToken(ctx contractapi.TransactionContextInterface, token string) (string, error) {
	if token == "" {
		return marshal(map[string]any{"valid": false, "error": "token required"})
	}

	key := fmt.Sprintf("auth:token:%s", token)
	var tokenData map[string]interface{}
	ok, err := getJSON(ctx, key, &tokenData)
	if err != nil {
		return marshal(map[string]any{"valid": false, "error": "verification failed"})
	}
	if !ok {
		return marshal(map[string]any{"valid": false, "error": "token not found"})
	}

	return marshal(map[string]any{"valid": true, "data": tokenData})
}

// RevokeIdentityBinding(client_id, reason) - Disable an identity
// func (s *SmartContract) RevokeIdentityBinding(ctx contractapi.TransactionContextInterface, clientID, reason string) error {
// 	if err := requirePlatform(ctx); err != nil {
// 		// Or allow self-revocation? Policy decision.
// 		return err
// 	}

// 	key := fmt.Sprintf("identity:binding:%s", clientID)
// 	var binding IdentityBinding
// 	ok, err := getJSON(ctx, key, &binding)
// 	if err != nil {
// 		return err
// 	}
// 	if !ok {
// 		return fmt.Errorf("identity not found: %s", clientID)
// 	}

// 	binding.Status = "revoked"
// 	binding.RevokedAt = nowRFC3339()

// 	if err := putJSON(ctx, key, binding); err != nil {
// 		return err
// 	}

// 	emit(ctx, "IdentityRevoked", map[string]any{
// 		"client_id": clientID,
// 		"msp_id": binding.MSPID,
// 		"reason": reason,
// 	})
// 	return nil
// }
