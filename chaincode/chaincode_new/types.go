package main

import "github.com/hyperledger/fabric-contract-api-go/v2/contractapi"

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
	GrantID     string              `json:"grant_id"`
	JobID       string              `json:"job_id"`
	ModelID     string              `json:"model_id"`
	SiteID      string              `json:"site_id"`
	OrgMSP      string              `json:"org_msp"`
	PolicyID    string              `json:"policy_id"`
	PolicyHash  string              `json:"policy_hash"`
	Purpose     string              `json:"purpose"` // TRAIN / VALIDATE
	Constraints TrainingConstraints `json:"constraints"`

	ValidFrom string      `json:"valid_from"` // RFC3339
	ValidTo   string      `json:"valid_to"`   // RFC3339
	Status    GrantStatus `json:"status"`

	IssuedByMSP   string `json:"issued_by_msp"`
	IssuedAt      string `json:"issued_at"`
	RenewalHash   string `json:"renewal_hash,omitempty"`
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

// type RoundRecord struct {
// 	JobID             string `json:"job_id"`
// 	RoundID           int    `json:"round_id"`
// 	GlobalModelHashIn string `json:"global_model_hash_in"`
// 	StartedAt         string `json:"started_at"`
// 	StartedByMSP      string `json:"started_by_msp"`
// }

type SiteUpdate struct {
	UpdateID       string `json:"update_id"`
	JobID          string `json:"job_id"`
	RoundID        int    `json:"round_id"`
	SiteID         string `json:"site_id"`
	UpdateHash     string `json:"update_hash"`      // hash of weights/gradients delta package
	MetricsHash    string `json:"metrics_hash"`     // hash of metrics blob
	DataDigestHash string `json:"data_digest_hash"` // optional: hash of dataset signature / cohort digest
	SubmittedAt    string `json:"submitted_at"`
	SubmittedByMSP string `json:"submitted_by_msp"`
	GrantID        string `json:"grant_id,omitempty"`
}

// type RoundCommit struct {
// 	JobID                string `json:"job_id"`
// 	RoundID              int    `json:"round_id"`
// 	GlobalModelHashOut   string `json:"global_model_hash_out"`
// 	AggregationProofHash string `json:"aggregation_proof_hash"` // hash of aggregation transcript/proof
// 	CommittedAt          string `json:"committed_at"`
// 	CommittedByMSP       string `json:"committed_by_msp"`
// }

// type ModelArtifact struct {
// 	ArtifactID    string       `json:"artifact_id"`
// 	JobID         string       `json:"job_id"`
// 	Type          ArtifactType `json:"type"` // INIT/CHECKPOINT/FINAL/EVAL
// 	Hash          string       `json:"hash"`
// 	MetaHash      string       `json:"meta_hash,omitempty"` // optional: hash of JSON metadata
// 	AnchoredAt    string       `json:"anchored_at"`
// 	AnchoredByMSP string       `json:"anchored_by_msp"`
// }
