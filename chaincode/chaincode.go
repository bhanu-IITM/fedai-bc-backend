// chaincode.go
// Plenome / FedAI PoC Governance Chaincode (v2)
// Implements: Site + Policy + Job + Access Grants + Round/Update/Artifacts + Queries/List APIs
//
// Module expectation (your go.mod):
//   module poc-chaincode
//   go 1.25.5
//   require github.com/hyperledger/fabric-contract-api-go/v2 v2.2.0
//
// NOTE: This is a PoC-friendly implementation:
// - Mutating “platform” ops are protected by a simple MSP allowlist (PLATFORM_ADMIN_MSP env var or default Org1MSP).
// - Site-submitted ops (SubmitSiteUpdate) are validated against an ACTIVE grant + time window.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

/* ------------------------------- Contract ------------------------------- */

type SmartContract struct {
	contractapi.Contract
}

/* ------------------------------- Constants ------------------------------ */

type SiteStatus string

const (
	SiteActive    SiteStatus = "ACTIVE"
	SiteSuspended SiteStatus = "SUSPENDED"
	SiteRevoked   SiteStatus = "REVOKED"
)

type PolicyStatus string

const (
	PolicyDraft      PolicyStatus = "DRAFT"
	PolicyActive     PolicyStatus = "ACTIVE"
	PolicyDeprecated PolicyStatus = "DEPRECATED"
)

type JobStatus string

const (
	JobCreated   JobStatus = "CREATED"
	JobRunning   JobStatus = "RUNNING"
	JobCompleted JobStatus = "COMPLETED"
	JobHalted    JobStatus = "HALTED"
)

type GrantStatus string

const (
	GrantActive  GrantStatus = "ACTIVE"
	GrantExpired GrantStatus = "EXPIRED"
	GrantRevoked GrantStatus = "REVOKED"
)

type ArtifactType string

const (
	ArtifactINIT       ArtifactType = "INIT"
	ArtifactCHECKPOINT ArtifactType = "CHECKPOINT"
	ArtifactFINAL      ArtifactType = "FINAL"
	ArtifactEVAL       ArtifactType = "EVAL"
)

/* --------------------------------- Types -------------------------------- */

type Site struct {
	SiteID                    string     `json:"site_id"`
	OrgMSP                    string     `json:"org_msp"`
	Jurisdiction              string     `json:"jurisdiction"`
	CapabilitiesHash          string     `json:"capabilities_hash"`
	EnrollmentCertFingerprint string     `json:"enrollment_cert_fingerprint,omitempty"`
	Status                    SiteStatus `json:"status"`
	CreatedAt                 string     `json:"created_at"`
	UpdatedAt                 string     `json:"updated_at"`
}

type SiteAttestation struct {
	AttestationID string `json:"attestation_id"`
	SiteID        string `json:"site_id"`
	EvidenceHash  string `json:"evidence_hash"` // hash of SBOM/env report/TPM quote bundle etc.
	AttestedAt    string `json:"attested_at"`
	AttestedByMSP string `json:"attested_by_msp"`
}

type Policy struct {
	PolicyID      string       `json:"policy_id"`
	Version       string       `json:"version"`
	PolicyHash    string       `json:"policy_hash"`
	IssuerMSP     string       `json:"issuer_msp"`
	Status        PolicyStatus `json:"status"`
	EffectiveFrom string       `json:"effective_from,omitempty"`
	EffectiveTo   string       `json:"effective_to,omitempty"`
	CreatedAt     string       `json:"created_at"`
}

type FederationJob struct {
	JobID         string    `json:"job_id"`
	ModelID       string    `json:"model_id"`
	ModelInitHash string    `json:"model_init_hash"`
	PolicyID      string    `json:"policy_id"`
	PolicyHash    string    `json:"policy_hash"`
	RoundsPlanned int       `json:"rounds_planned"`
	Cadence       string    `json:"cadence"` // e.g., WEEKLY, DAILY, CRON-like, or descriptive
	Status        JobStatus `json:"status"`
	CreatedAt     string    `json:"created_at"`
}

type JobParticipants struct {
	JobID         string   `json:"job_id"`
	Participants  []string `json:"participants"` // deterministic site_id set
	ParticipantsH string   `json:"participants_hash,omitempty"`
	RegisteredAt  string   `json:"registered_at"`
	RegisteredBy  string   `json:"registered_by_msp"`
}

type TrainingConstraints struct {
	Runtime       string   `json:"runtime"` // CPU_ONLY / GPU_ENABLED / TEE_REQUIRED etc.
	MaxRounds     int      `json:"max_rounds,omitempty"`
	MaxEpochs     int      `json:"max_epochs,omitempty"`
	Modalities    []string `json:"modalities,omitempty"`
	DataScopeHash string   `json:"data_scope_hash,omitempty"`
}

type TrainingAccessGrant struct {
	GrantID     string             `json:"grant_id"`
	JobID       string             `json:"job_id"`
	ModelID     string             `json:"model_id"`
	SiteID      string             `json:"site_id"`
	OrgMSP      string             `json:"org_msp"`
	PolicyID    string             `json:"policy_id"`
	PolicyHash  string             `json:"policy_hash"`
	Purpose     string             `json:"purpose"` // TRAIN / VALIDATE
	Constraints TrainingConstraints `json:"constraints"`

	ValidFrom string      `json:"valid_from"` // RFC3339
	ValidTo   string      `json:"valid_to"`   // RFC3339
	Status    GrantStatus `json:"status"`

	IssuedByMSP string `json:"issued_by_msp"`
	IssuedAt    string `json:"issued_at"`
	RenewalHash string `json:"renewal_hash,omitempty"`
	RevokedReason string `json:"revoked_reason,omitempty"`
}

type EligibilityDecision struct {
	JobID     string `json:"job_id"`
	SiteID    string `json:"site_id"`
	Decision  string `json:"decision"` // ALLOW / DENY
	Reason    string `json:"reason"`
	GrantID   string `json:"grant_id,omitempty"`
	CheckedAt string `json:"checked_at"` // RFC3339
}

type RoundRecord struct {
	JobID             string `json:"job_id"`
	RoundID           int    `json:"round_id"`
	GlobalModelHashIn string `json:"global_model_hash_in"`
	StartedAt         string `json:"started_at"`
	StartedByMSP      string `json:"started_by_msp"`
}

type SiteUpdate struct {
	UpdateID        string `json:"update_id"`
	JobID           string `json:"job_id"`
	RoundID         int    `json:"round_id"`
	SiteID          string `json:"site_id"`
	UpdateHash      string `json:"update_hash"`      // hash of weights/gradients delta package
	MetricsHash     string `json:"metrics_hash"`     // hash of metrics blob
	DataDigestHash  string `json:"data_digest_hash"` // optional: hash of dataset signature / cohort digest
	SubmittedAt     string `json:"submitted_at"`
	SubmittedByMSP  string `json:"submitted_by_msp"`
	GrantID         string `json:"grant_id,omitempty"`
}

type RoundCommit struct {
	JobID                string `json:"job_id"`
	RoundID              int    `json:"round_id"`
	GlobalModelHashOut   string `json:"global_model_hash_out"`
	AggregationProofHash string `json:"aggregation_proof_hash"` // hash of aggregation transcript/proof
	CommittedAt          string `json:"committed_at"`
	CommittedByMSP       string `json:"committed_by_msp"`
}

type ModelArtifact struct {
	ArtifactID    string       `json:"artifact_id"`
	JobID         string       `json:"job_id"`
	Type          ArtifactType `json:"type"` // INIT/CHECKPOINT/FINAL/EVAL
	Hash          string       `json:"hash"`
	MetaHash      string       `json:"meta_hash,omitempty"` // optional: hash of JSON metadata
	AnchoredAt    string       `json:"anchored_at"`
	AnchoredByMSP string       `json:"anchored_by_msp"`
}

/* ------------------------------- Key Design ----------------------------- */

func keySite(siteID string) string        { return "site:" + siteID }
func keyAtt(attID string) string          { return "att:" + attID }
func keyPolicy(policyID string) string    { return "policy:" + policyID }
func keyJob(jobID string) string          { return "job:" + jobID }
func keyParticipants(jobID string) string { return "participants:" + jobID }
func keyGrant(grantID string) string      { return "grant:" + grantID }
func keyRound(jobID string, roundID int) string {
	return fmt.Sprintf("round:%s:%d", jobID, roundID)
}
func keyUpdate(updateID string) string    { return "update:" + updateID }
func keyCommit(jobID string, roundID int) string {
	return fmt.Sprintf("commit:%s:%d", jobID, roundID)
}
func keyArtifact(artifactID string) string { return "artifact:" + artifactID }

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

/* ============================== A) Site Lifecycle ============================== */

// RegisterSite(siteJSON)
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

// CreateFederationJob(jobJSON)
func (s *SmartContract) CreateFederationJob(ctx contractapi.TransactionContextInterface, jobJSON string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	var j FederationJob
	if err := unmarshal(jobJSON, &j); err != nil {
		return err
	}
	if err := mustNonEmpty(j.JobID, "job_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(j.ModelID, "model_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(j.PolicyID, "policy_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(j.PolicyHash, "policy_hash"); err != nil {
		return err
	}
	if j.RoundsPlanned <= 0 {
		return fmt.Errorf("rounds_planned must be > 0")
	}

	// Ensure policy exists (and hash matches)
	var p Policy
	ok, err := getJSON(ctx, keyPolicy(j.PolicyID), &p)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("policy not found: %s", j.PolicyID)
	}
	if p.PolicyHash != j.PolicyHash {
		return fmt.Errorf("policy hash mismatch (policy record vs job)")
	}

	if j.Status == "" {
		j.Status = JobCreated
	}
	if strings.TrimSpace(j.CreatedAt) == "" {
		j.CreatedAt = nowRFC3339()
	}

	if err := putJSON(ctx, keyJob(j.JobID), j); err != nil {
		return err
	}
	emit(ctx, "JobCreated", j)
	return nil
}

// RegisterJobParticipants(job_id, participantsJSON)
func (s *SmartContract) RegisterJobParticipants(ctx contractapi.TransactionContextInterface, jobID, participantsJSON string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	// Ensure job exists
	var j FederationJob
	ok, err := getJSON(ctx, keyJob(jobID), &j)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	var payload struct {
		Participants []string `json:"participants"`
		Hash         string   `json:"participants_hash,omitempty"`
	}
	if err := unmarshal(participantsJSON, &payload); err != nil {
		return err
	}
	if len(payload.Participants) == 0 {
		return fmt.Errorf("participants must be non-empty")
	}
	// Normalize + sort for determinism
	uniq := make(map[string]struct{})
	var parts []string
	for _, sID := range payload.Participants {
		sID = strings.TrimSpace(sID)
		if sID == "" {
			continue
		}
		if _, ok := uniq[sID]; ok {
			continue
		}
		uniq[sID] = struct{}{}
		parts = append(parts, sID)
	}
	sort.Strings(parts)
	if len(parts) == 0 {
		return fmt.Errorf("participants must be non-empty after normalization")
	}

	msp, _ := s.getClientMSP(ctx)
	rec := JobParticipants{
		JobID:         jobID,
		Participants:  parts,
		ParticipantsH: payload.Hash,
		RegisteredAt:  nowRFC3339(),
		RegisteredBy:  msp,
	}

	if err := putJSON(ctx, keyParticipants(jobID), rec); err != nil {
		return err
	}
	emit(ctx, "JobParticipantsRegistered", map[string]any{"job_id": jobID, "count": len(parts)})
	_ = j // referenced above (keeps compilation clean if you expand logic later)
	return nil
}

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

// StartRound(job_id, round_id, global_model_hash_in)
func (s *SmartContract) StartRound(ctx contractapi.TransactionContextInterface, jobID string, roundID int, globalModelHashIn string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	if err := mustNonEmpty(jobID, "job_id"); err != nil {
		return err
	}
	if roundID < 0 {
		return fmt.Errorf("round_id must be >= 0")
	}
	if err := mustNonEmpty(globalModelHashIn, "global_model_hash_in"); err != nil {
		return err
	}

	// Ensure job exists
	var job FederationJob
	ok, err := getJSON(ctx, keyJob(jobID), &job)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}
	_ = job

	msp, _ := s.getClientMSP(ctx)
	r := RoundRecord{
		JobID:             jobID,
		RoundID:           roundID,
		GlobalModelHashIn: globalModelHashIn,
		StartedAt:         nowRFC3339(),
		StartedByMSP:      msp,
	}
	if err := putJSON(ctx, keyRound(jobID, roundID), r); err != nil {
		return err
	}

	// Index: round~job(jobID, roundID)
	idx, _ := ctx.GetStub().CreateCompositeKey("round~job", []string{jobID, fmt.Sprintf("%d", roundID)})
	_ = ctx.GetStub().PutState(idx, []byte{0x00})

	emit(ctx, "RoundStarted", r)
	return nil
}

// SubmitSiteUpdate(updateJSON)
func (s *SmartContract) SubmitSiteUpdate(ctx contractapi.TransactionContextInterface, updateJSON string) error {
	// Site action: validate against eligibility at current time (or submitted_at)
	var u SiteUpdate
	if err := unmarshal(updateJSON, &u); err != nil {
		return err
	}
	if err := mustNonEmpty(u.UpdateID, "update_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(u.JobID, "job_id"); err != nil {
		return err
	}
	if u.RoundID < 0 {
		return fmt.Errorf("round_id must be >= 0")
	}
	if err := mustNonEmpty(u.SiteID, "site_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(u.UpdateHash, "update_hash"); err != nil {
		return err
	}
	if strings.TrimSpace(u.SubmittedAt) == "" {
		u.SubmittedAt = nowRFC3339()
	}

	// Caller MSP
	msp, err := s.getClientMSP(ctx)
	if err != nil {
		return err
	}
	u.SubmittedByMSP = msp

	// Eligibility check at submitted_at time
	decJSON, err := s.VerifyTrainingEligibility(ctx, u.JobID, u.SiteID, u.SubmittedAt)
	if err != nil {
		return err
	}
	var dec EligibilityDecision
	if err := unmarshal(decJSON, &dec); err != nil {
		return err
	}
	if dec.Decision != "ALLOW" {
		return fmt.Errorf("update rejected: eligibility DENY (%s)", dec.Reason)
	}
	u.GrantID = dec.GrantID

	// Persist update
	if err := putJSON(ctx, keyUpdate(u.UpdateID), u); err != nil {
		return err
	}

	// Index: update~job~round(jobID, roundID, updateID)
	idx, _ := ctx.GetStub().CreateCompositeKey("update~job~round", []string{u.JobID, fmt.Sprintf("%d", u.RoundID), u.UpdateID})
	_ = ctx.GetStub().PutState(idx, []byte{0x00})

	emit(ctx, "SiteUpdateSubmitted", map[string]any{"job_id": u.JobID, "round_id": u.RoundID, "site_id": u.SiteID, "update_id": u.UpdateID})
	return nil
}

// CommitRound(commitJSON)
func (s *SmartContract) CommitRound(ctx contractapi.TransactionContextInterface, commitJSON string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	var c RoundCommit
	if err := unmarshal(commitJSON, &c); err != nil {
		return err
	}
	if err := mustNonEmpty(c.JobID, "job_id"); err != nil {
		return err
	}
	if c.RoundID < 0 {
		return fmt.Errorf("round_id must be >= 0")
	}
	if err := mustNonEmpty(c.GlobalModelHashOut, "global_model_hash_out"); err != nil {
		return err
	}
	if err := mustNonEmpty(c.AggregationProofHash, "aggregation_proof_hash"); err != nil {
		return err
	}
	if strings.TrimSpace(c.CommittedAt) == "" {
		c.CommittedAt = nowRFC3339()
	}
	msp, _ := s.getClientMSP(ctx)
	c.CommittedByMSP = msp

	// Ensure round exists (optional but good)
	var r RoundRecord
	ok, err := getJSON(ctx, keyRound(c.JobID, c.RoundID), &r)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("round not found: job=%s round=%d", c.JobID, c.RoundID)
	}
	_ = r

	if err := putJSON(ctx, keyCommit(c.JobID, c.RoundID), c); err != nil {
		return err
	}
	emit(ctx, "RoundCommitted", c)
	return nil
}

// AnchorModelArtifact(artifactJSON)
func (s *SmartContract) AnchorModelArtifact(ctx contractapi.TransactionContextInterface, artifactJSON string) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	var a ModelArtifact
	if err := unmarshal(artifactJSON, &a); err != nil {
		return err
	}
	if err := mustNonEmpty(a.ArtifactID, "artifact_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(a.JobID, "job_id"); err != nil {
		return err
	}
	if err := mustNonEmpty(string(a.Type), "type"); err != nil {
		return err
	}
	if err := mustNonEmpty(a.Hash, "hash"); err != nil {
		return err
	}
	if strings.TrimSpace(a.AnchoredAt) == "" {
		a.AnchoredAt = nowRFC3339()
	}
	msp, _ := s.getClientMSP(ctx)
	a.AnchoredByMSP = msp

	// Ensure job exists
	var job FederationJob
	ok, err := getJSON(ctx, keyJob(a.JobID), &job)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("job not found: %s", a.JobID)
	}
	_ = job

	if err := putJSON(ctx, keyArtifact(a.ArtifactID), a); err != nil {
		return err
	}

	// Index: artifact~job(jobID, artifactID)
	idx, _ := ctx.GetStub().CreateCompositeKey("artifact~job", []string{a.JobID, a.ArtifactID})
	_ = ctx.GetStub().PutState(idx, []byte{0x00})

	emit(ctx, "ArtifactAnchored", map[string]any{"job_id": a.JobID, "artifact_id": a.ArtifactID, "type": a.Type})
	return nil
}

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

// GetRound(job_id, round_id)
func (s *SmartContract) GetRound(ctx contractapi.TransactionContextInterface, jobID string, roundID int) (string, error) {
	var r RoundRecord
	ok, err := getJSON(ctx, keyRound(jobID, roundID), &r)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("round not found: job=%s round=%d", jobID, roundID)
	}
	return marshal(r)
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

// ListUpdatesForRound(job_id, round_id)
func (s *SmartContract) ListUpdatesForRound(ctx contractapi.TransactionContextInterface, jobID string, roundID int) (string, error) {
	iter, err := ctx.GetStub().GetStateByPartialCompositeKey("update~job~round", []string{jobID, fmt.Sprintf("%d", roundID)})
	if err != nil {
		return "", err
	}
	defer iter.Close()

	var updates []SiteUpdate
	for iter.HasNext() {
		kv, _ := iter.Next()
		_, parts, _ := ctx.GetStub().SplitCompositeKey(kv.Key)
		if len(parts) != 3 {
			continue
		}
		updateID := parts[2]
		var u SiteUpdate
		ok, err := getJSON(ctx, keyUpdate(updateID), &u)
		if err != nil || !ok {
			continue
		}
		updates = append(updates, u)
	}
	return marshal(updates)
}

// ListArtifactsForJob(job_id)
func (s *SmartContract) ListArtifactsForJob(ctx contractapi.TransactionContextInterface, jobID string) (string, error) {
	iter, err := ctx.GetStub().GetStateByPartialCompositeKey("artifact~job", []string{jobID})
	if err != nil {
		return "", err
	}
	defer iter.Close()

	var artifacts []ModelArtifact
	for iter.HasNext() {
		kv, _ := iter.Next()
		_, parts, _ := ctx.GetStub().SplitCompositeKey(kv.Key)
		if len(parts) != 2 {
			continue
		}
		artifactID := parts[1]
		var a ModelArtifact
		ok, err := getJSON(ctx, keyArtifact(artifactID), &a)
		if err != nil || !ok {
			continue
		}
		artifacts = append(artifacts, a)
	}
	return marshal(artifacts)
}

/* ------------------------------ Main ------------------------------ */

func main() {
	cc, err := contractapi.NewChaincode(&SmartContract{})
	if err != nil {
		panic(err)
	}
	if err := cc.Start(); err != nil {
		panic(err)
	}
}
