package fabric

import (
	"context"
	"time"

	fabricgw "github.com/hyperledger/fabric-gateway/pkg/client"
)

type TxMode string


const (
	TxAsyncNoWait   TxMode = "ASYNC_NO_WAIT"   // return immediately after submit
	TxAsyncWaitCommit TxMode = "ASYNC_WAIT_COMMIT" // block on commit.Status() (bounded by gateway commit timeout)
)

type SubmitOpts struct {
	Mode TxMode

	// Applies ONLY to endorse+submit phase (SubmitAsyncWithContext)
	EndorseSubmitTimeout time.Duration
}

type SubmitResult struct {
	TxID        string `json:"txid,omitempty"`
	Status      string `json:"status"` // PENDING / COMMITTED / FAILED
	Result      string `json:"result,omitempty"`
	CommitCode  int    `json:"commit_code,omitempty"`
	BlockNumber uint64 `json:"commit_block,omitempty"`
	Error       string `json:"error,omitempty"`
	Committed   bool   `json:"committed"`
}

type TxSubmitter struct {
	contract *fabricgw.Contract
}

func NewTxSubmitter(contract *fabricgw.Contract) *TxSubmitter {
	return &TxSubmitter{contract: contract}
}

// SubmitWithOpts:
// - Uses ctx+EndorseSubmitTimeout for endorse+submit
// - If opts.Mode == TxAsyncWaitCommit, waits for commit via commit.Status()
//   NOTE: commit wait duration is bounded by Gateway option WithCommitStatusTimeout.
func (s *TxSubmitter) SubmitWithOpts(ctx context.Context, fn string, opts SubmitOpts, args ...string) SubmitResult {
	if opts.Mode == "" {
		opts.Mode = TxAsyncNoWait
	}
	if opts.EndorseSubmitTimeout <= 0 {
		opts.EndorseSubmitTimeout = 10 * time.Second
	}

	// Endorse + submit bounded by context
	esCtx, cancel := context.WithTimeout(ctx, opts.EndorseSubmitTimeout)
	defer cancel()

	resBytes, commit, err := s.contract.SubmitAsyncWithContext(
		esCtx,
		fn,
		fabricgw.WithArguments(args...),
	)
	if err != nil {
		return SubmitResult{
			Status:    "FAILED",
			Error:     err.Error(),
			Committed: false,
		}
	}

	out := SubmitResult{
		TxID:      commit.TransactionID(),
		Result:    string(resBytes),
		Status:    "PENDING",
		Committed: false,
	}

	if opts.Mode == TxAsyncNoWait {
		return out
	}

	// Commit wait is bounded by gateway-level commit status timeout (WithCommitStatusTimeout)
	st, err := commit.Status()
	if err != nil {
		out.Status = "FAILED"
		out.Error = err.Error()
		return out
	}

	out.CommitCode = int(st.Code)
	out.BlockNumber = st.BlockNumber

	if out.CommitCode == 0 {
		out.Status = "COMMITTED"
		out.Committed = true
	} else {
		out.Status = "FAILED"
		out.Error = "commit failed"
	}

	return out
}