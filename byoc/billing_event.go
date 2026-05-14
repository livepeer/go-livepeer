package byoc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/clog"
)

// BillingEvent is the structured audit record emitted on every terminal
// state of a billable job. PR-10 of byoc-payment-fleet-2026-05 (design
// §16, §3.B).
//
// Why this exists: Invariant I3 of the payment model — "every billable
// job emits exactly one BillingEvent on its terminal state, suitable for
// off-chain reconciliation against orch's ticket-redemption ledger".
//
// The emit path is a single JSON line via clog/glog. Promtail tails
// orchestrator stdout, ships to GCP Cloud Logging where the events are
// queryable by job_id, user_hash, capability. No new sink, no new
// dependency.
//
// Field stability: this struct is the audit contract. Renaming or
// removing fields breaks the reconciliation pipeline. Adding fields is
// safe (downstream parsers should ignore unknown keys).
type BillingEvent struct {
	Event           string `json:"event"`                       // always "billing_event"
	SchemaVersion   int    `json:"schema_version"`              // bumped on breaking changes
	Timestamp       string `json:"timestamp"`                   // RFC3339 UTC
	JobID           string `json:"job_id"`
	JobType         string `json:"job_type"`                    // "training" (PR-10 scope)
	Capability      string `json:"capability"`
	ModelID         string `json:"model_id,omitempty"`
	UserHash        string `json:"user_hash"`                   // sha256(sender)[:16], not reversible
	SenderAddress   string `json:"sender_address"`              // full 0x… for orch-side reconciliation
	Status          string `json:"status"`                      // completed | failed | cancelled | failed_orchestrator_restart | timed_out
	CostPaidWei     string `json:"cost_paid_wei"`               // base-10 integer; matches TrainingJob.Cost
	BalanceWei      string `json:"balance_wei"`                 // post-charge sender balance
	BillableSeconds int64  `json:"billable_seconds"`            // seconds the orch charged for
	WallSeconds     int64  `json:"wall_seconds,omitempty"`      // wall-clock from submit to terminal
	StartedAt       string `json:"started_at"`                  // RFC3339 — job submit
	CompletedAt     string `json:"completed_at"`                // RFC3339 — terminal transition
	BillingStartedAt string `json:"billing_started_at,omitempty"` // RFC3339 — when seenInProgress flipped
	ErrorMessage    string `json:"error_message,omitempty"`
}

const billingEventSchemaVersion = 1

// hashSender produces a stable, non-reversible 16-char user identifier
// from an Ethereum address. Used for user-level rollups without leaking
// the sender. Full address is also emitted for orch-side reconciliation
// — anyone with read access to BillingEvent already has read access to
// the on-chain ledger, so the address isn't a new disclosure.
func hashSender(addr ethcommon.Address) string {
	h := sha256.Sum256(addr.Bytes())
	return hex.EncodeToString(h[:])[:16]
}

// emitBillingEvent writes a single JSONL line to the orchestrator log.
// Tagged with the "BillingEvent" string prefix so log shippers can
// regex-filter without parsing every line.
//
// Marshaling failures are logged but never bubble up — the caller is
// emitting an audit record on an already-terminal job and shouldn't
// fail-loud here. A missing event will show up as a reconciliation
// gap, which the audit pipeline must already handle for the
// orch-restart-lost case.
func emitBillingEvent(ctx context.Context, ev BillingEvent) {
	ev.Event = "billing_event"
	ev.SchemaVersion = billingEventSchemaVersion
	if ev.Timestamp == "" {
		ev.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if ev.CompletedAt == "" {
		ev.CompletedAt = ev.Timestamp
	}
	b, err := json.Marshal(ev)
	if err != nil {
		glog.Warningf("billing_event: marshal failed for job=%s: %v", ev.JobID, err)
		return
	}
	// Prefix lets log shippers identify the event class cheaply. The full
	// JSON follows on the same line so a single regex extracts it.
	if ctx != nil {
		clog.Infof(ctx, "BillingEvent %s", string(b))
	} else {
		glog.Infof("BillingEvent %s", string(b))
	}
}

// trainingBillingEvent builds a BillingEvent for a training job. Helper
// because every terminal-state caller in training.go needs the same set
// of fields populated from TrainingJob + a few locals.
func trainingBillingEvent(job *TrainingJob, status string, billableSeconds int64, billingStartedAt time.Time, errMsg string) BillingEvent {
	cost := strings.TrimSpace(job.Cost)
	if cost == "" {
		cost = "0"
	}
	bal := strings.TrimSpace(job.Balance)
	if bal == "" {
		bal = "0"
	}
	ev := BillingEvent{
		JobID:           job.JobID,
		JobType:         "training",
		Capability:      job.Capability,
		ModelID:         job.ModelID,
		UserHash:        hashSender(job.sender),
		SenderAddress:   job.sender.Hex(),
		Status:          status,
		CostPaidWei:     cost,
		BalanceWei:      bal,
		BillableSeconds: billableSeconds,
		WallSeconds:     int64(time.Since(job.CreatedAt).Seconds()),
		StartedAt:       job.CreatedAt.UTC().Format(time.RFC3339),
		ErrorMessage:    errMsg,
	}
	if !billingStartedAt.IsZero() {
		ev.BillingStartedAt = billingStartedAt.UTC().Format(time.RFC3339)
	}
	return ev
}
