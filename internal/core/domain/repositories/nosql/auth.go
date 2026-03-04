package nosql


type RegisterEnrollResult struct {
	Status string `json:"status"` // CREATED / PENDING / FAILED

	Fabric struct {
		EnrollID    string `json:"enroll_id"`
		MSPDir      string `json:"msp_dir"`
		CertPath    string `json:"cert_path"`
		KeyPath     string `json:"key_path"`
		Secret      string `json:"secret"`
		Fingerprint string `json:"fingerprint"`
	} `json:"fabric"`

	Ledger struct {
		Anchored  bool   `json:"anchored"`
		TxID      string `json:"txid,omitempty"`
		Committed bool   `json:"committed"`
		Error     string `json:"error,omitempty"`
		Record    string `json:"record,omitempty"`
	} `json:"ledger"`

	NVFlare struct {
		Name string `json:"name"`
		Org  string `json:"org,omitempty"`
	} `json:"nvflare"`

	Auth struct {
		ClientID string `json:"client_id"`
		Password string `json:"password"`
	} `json:"auth"`
}

type MockHospital struct {
	Name string `json:"name"`
	Org  string `json:"org,omitempty"`
}

type RegisterEnrollReq struct {
	ProjectName   string         `json:"project_name"`
	MockHospitals []MockHospital `json:"mock_hospitals"`

	// If true, we will best-effort wait for commit confirmation.
	// NOTE: actual commit wait duration is bounded by gateway WithCommitStatusTimeout(cfg.CommitTimeoutSec)
	WaitForCommit bool `json:"wait_for_commit,omitempty"`
}

type LoginRequest struct {
	ClientID string `json:"client_id" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Status  string `json:"status"`
	Token   string `json:"token"`
	Message string `json:"message"`
}