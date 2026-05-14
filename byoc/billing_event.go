package byoc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
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

// emitBillingEvent writes a single JSONL line to the orchestrator log
// AND appends to an in-memory ring buffer that the
// /admin/billing-events endpoint exposes for the storyboard /payments
// page.
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
	// Stash in the ring buffer BEFORE marshaling — the buffer holds the
	// struct, not the JSON, so dashboard queries don't pay a re-parse
	// cost per event.
	billingEventRing.append(ev)
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

// --------------------------------------------------------------------
// In-memory ring buffer for the live dashboard
// --------------------------------------------------------------------

// billingEventRingSize caps in-process memory at ~500 events. At ~600B
// per event that's ~300 KB resident — fine. Older events spill to
// stdout (docker logs) only; nothing is lost from the audit trail.
const billingEventRingSize = 500

type billingEventRingBuffer struct {
	mu     sync.RWMutex
	events []BillingEvent // most-recent-first append, capped
}

var billingEventRing = &billingEventRingBuffer{
	events: make([]BillingEvent, 0, billingEventRingSize),
}

func (r *billingEventRingBuffer) append(ev BillingEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) >= billingEventRingSize {
		// Drop the oldest event. Keep the slice fixed-length so the
		// allocator doesn't churn under steady traffic.
		copy(r.events, r.events[1:])
		r.events = r.events[:len(r.events)-1]
	}
	r.events = append(r.events, ev)
}

func (r *billingEventRingBuffer) snapshot(limit int) []BillingEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := len(r.events)
	if limit <= 0 || limit > n {
		limit = n
	}
	// Return most-recent-first: take the last `limit` entries and
	// reverse-copy them so the dashboard renders newest-at-top
	// without re-sorting.
	out := make([]BillingEvent, limit)
	src := r.events[n-limit:]
	for i, ev := range src {
		out[limit-1-i] = ev
	}
	return out
}

// BillingEventsHandler returns a JSON snapshot of the most recent
// BillingEvents. Path: `GET /admin/billing-events?limit=N`. Default
// limit is the ring buffer size.
//
// This is intentionally an UNAUTHENTICATED endpoint on the orch's
// internal HTTP mux — the storyboard /payments page proxies through
// the storyboard's bearer-auth layer, so cap on exposure is at the
// edge (Caddy) not in this handler. Don't expose the orch HTTP port
// publicly without that proxy.
func (bso *BYOCOrchestratorServer) BillingEventsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := 0
		if q := r.URL.Query().Get("limit"); q != "" {
			if n, err := strconv.Atoi(q); err == nil && n > 0 {
				limit = n
			}
		}
		events := billingEventRing.snapshot(limit)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"events":     events,
			"count":      len(events),
			"ring_size":  billingEventRingSize,
			"schema_ver": billingEventSchemaVersion,
		})
	})
}

// inferenceBillingEvent builds a BillingEvent for an inference job
// terminal state. PR-10b — inference path emits BillingEvent so the
// /payments dashboard surfaces canary-flip traffic.
//
// `costWei` is the delta between pre- and post-call balance (balanceBefore
// minus balanceAfter). Caller computes this so we don't re-do the
// chargeForCompute math.
func inferenceBillingEvent(
	jobID string,
	capability string,
	sender ethcommon.Address,
	start time.Time,
	status string,
	costWei string,
	balanceWei string,
	errMsg string,
) BillingEvent {
	billableSeconds := int64(0)
	if !start.IsZero() {
		billableSeconds = int64(time.Since(start).Seconds())
		if billableSeconds < 0 {
			billableSeconds = 0
		}
	}
	if costWei == "" {
		costWei = "0"
	}
	if balanceWei == "" {
		balanceWei = "0"
	}
	return BillingEvent{
		JobID:           jobID,
		JobType:         "inference",
		Capability:      capability,
		UserHash:        hashSender(sender),
		SenderAddress:   sender.Hex(),
		Status:          status,
		CostPaidWei:     costWei,
		BalanceWei:      balanceWei,
		BillableSeconds: billableSeconds,
		WallSeconds:     billableSeconds,
		StartedAt:       start.UTC().Format(time.RFC3339),
		ErrorMessage:    errMsg,
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
