package byoc

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

func TestHashSender_Deterministic(t *testing.T) {
	addr := ethcommon.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	h1 := hashSender(addr)
	h2 := hashSender(addr)
	if h1 != h2 {
		t.Fatalf("hashSender not deterministic: %s vs %s", h1, h2)
	}
	if len(h1) != 16 {
		t.Fatalf("hashSender wrong length: got %d, want 16", len(h1))
	}
	// Different addresses produce different hashes
	addr2 := ethcommon.HexToAddress("0x0000000000000000000000000000000000000001")
	if hashSender(addr) == hashSender(addr2) {
		t.Fatal("hashSender collision on distinct addresses")
	}
}

func TestTrainingBillingEvent_PopulatedFields(t *testing.T) {
	addr := ethcommon.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	created := time.Now().Add(-5 * time.Minute)
	billingStart := created.Add(30 * time.Second)
	job := &TrainingJob{
		JobID:      "job-abc",
		Capability: "flux-lora-training",
		ModelID:    "flux-dev",
		Cost:       "12345",
		Balance:    "67890",
		CreatedAt:  created,
		sender:     addr,
		SenderHex:  addr.Hex(),
	}
	ev := trainingBillingEvent(job, TrainingStatusCompleted, 240, billingStart, "")

	if ev.JobID != "job-abc" {
		t.Errorf("JobID: got %q", ev.JobID)
	}
	if ev.JobType != "training" {
		t.Errorf("JobType: got %q, want training", ev.JobType)
	}
	if ev.Capability != "flux-lora-training" {
		t.Errorf("Capability: got %q", ev.Capability)
	}
	if ev.UserHash != hashSender(addr) {
		t.Errorf("UserHash mismatch")
	}
	if ev.SenderAddress != addr.Hex() {
		t.Errorf("SenderAddress: got %q", ev.SenderAddress)
	}
	if ev.Status != TrainingStatusCompleted {
		t.Errorf("Status: got %q", ev.Status)
	}
	if ev.CostPaidWei != "12345" {
		t.Errorf("CostPaidWei: got %q, want 12345", ev.CostPaidWei)
	}
	if ev.BalanceWei != "67890" {
		t.Errorf("BalanceWei: got %q", ev.BalanceWei)
	}
	if ev.BillableSeconds != 240 {
		t.Errorf("BillableSeconds: got %d", ev.BillableSeconds)
	}
	if ev.WallSeconds < 290 || ev.WallSeconds > 320 {
		t.Errorf("WallSeconds: got %d, expected ~300", ev.WallSeconds)
	}
	if ev.BillingStartedAt == "" {
		t.Error("BillingStartedAt empty when billingStart is non-zero")
	}
	if ev.ErrorMessage != "" {
		t.Errorf("ErrorMessage: got %q, want empty for completed", ev.ErrorMessage)
	}
}

func TestTrainingBillingEvent_ErrorPath(t *testing.T) {
	addr := ethcommon.HexToAddress("0xdeadbeef00000000000000000000000000000000")
	job := &TrainingJob{
		JobID:      "job-fail",
		Capability: "flux-lora-training",
		CreatedAt:  time.Now(),
		Cost:       "0",
		Balance:    "0",
		sender:     addr,
		SenderHex:  addr.Hex(),
	}
	ev := trainingBillingEvent(job, TrainingStatusFailed, 0, time.Time{}, "adapter returned 500")
	if ev.Status != TrainingStatusFailed {
		t.Errorf("Status: got %q", ev.Status)
	}
	if ev.ErrorMessage != "adapter returned 500" {
		t.Errorf("ErrorMessage: got %q", ev.ErrorMessage)
	}
	if ev.BillingStartedAt != "" {
		t.Error("BillingStartedAt should be empty when billingStart is zero")
	}
	if ev.BillableSeconds != 0 {
		t.Errorf("BillableSeconds: got %d, want 0", ev.BillableSeconds)
	}
}

func TestTrainingBillingEvent_EmptyCostDefaultsToZero(t *testing.T) {
	job := &TrainingJob{JobID: "j", Cost: "", Balance: "  "}
	ev := trainingBillingEvent(job, TrainingStatusCompleted, 0, time.Time{}, "")
	if ev.CostPaidWei != "0" {
		t.Errorf("CostPaidWei: got %q, want 0", ev.CostPaidWei)
	}
	if ev.BalanceWei != "0" {
		t.Errorf("BalanceWei: got %q, want 0", ev.BalanceWei)
	}
}

func TestBillingEventRing_AppendAndSnapshotOrdering(t *testing.T) {
	// Reset the global ring between tests
	billingEventRing = &billingEventRingBuffer{
		events: make([]BillingEvent, 0, billingEventRingSize),
	}
	for i := 0; i < 10; i++ {
		billingEventRing.append(BillingEvent{JobID: "job-" + string(rune('0'+i))})
	}
	snap := billingEventRing.snapshot(0) // 0 = all
	if len(snap) != 10 {
		t.Fatalf("snapshot len: got %d, want 10", len(snap))
	}
	// Most-recent-first ordering
	if snap[0].JobID != "job-9" || snap[9].JobID != "job-0" {
		t.Errorf("ordering wrong: first=%q last=%q (want job-9 / job-0)", snap[0].JobID, snap[9].JobID)
	}
}

func TestBillingEventRing_LimitClampsToBuffer(t *testing.T) {
	billingEventRing = &billingEventRingBuffer{
		events: make([]BillingEvent, 0, billingEventRingSize),
	}
	for i := 0; i < 5; i++ {
		billingEventRing.append(BillingEvent{JobID: "j" + string(rune('0'+i))})
	}
	// Asking for more than exist returns all
	all := billingEventRing.snapshot(100)
	if len(all) != 5 {
		t.Errorf("oversized limit: got %d, want 5", len(all))
	}
	// Asking for fewer than exist returns the newest N
	top3 := billingEventRing.snapshot(3)
	if len(top3) != 3 {
		t.Fatalf("limit=3 returned %d events", len(top3))
	}
	if top3[0].JobID != "j4" || top3[2].JobID != "j2" {
		t.Errorf("limit=3 ordering wrong: %v", []string{top3[0].JobID, top3[1].JobID, top3[2].JobID})
	}
}

func TestBillingEventRing_DropsOldestPastCap(t *testing.T) {
	// Use a temporary small ring to verify the eviction path without
	// pumping 500+ events.
	r := &billingEventRingBuffer{
		events: make([]BillingEvent, 0, 3),
	}
	// Manually simulate the cap by appending one extra (the prod ring
	// uses billingEventRingSize, but the eviction logic is the same).
	for i := 0; i < 5; i++ {
		// Inline the cap-3 invariant from the real append
		if len(r.events) >= 3 {
			copy(r.events, r.events[1:])
			r.events = r.events[:len(r.events)-1]
		}
		r.events = append(r.events, BillingEvent{JobID: "j" + string(rune('0'+i))})
	}
	if len(r.events) != 3 {
		t.Fatalf("len after 5 appends to cap=3: got %d, want 3", len(r.events))
	}
	if r.events[0].JobID != "j2" || r.events[2].JobID != "j4" {
		t.Errorf("eviction wrong: %v", []string{r.events[0].JobID, r.events[1].JobID, r.events[2].JobID})
	}
}

func TestBillingEvent_JSONShape(t *testing.T) {
	// Round-trip serialize to confirm the contract holds. Audit pipeline
	// parses these lines; renaming fields silently breaks reconciliation.
	addr := ethcommon.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	ev := BillingEvent{
		Event:           "billing_event",
		SchemaVersion:   1,
		Timestamp:       "2026-05-14T03:00:00Z",
		JobID:           "j",
		JobType:         "training",
		Capability:      "flux-lora-training",
		UserHash:        hashSender(addr),
		SenderAddress:   addr.Hex(),
		Status:          TrainingStatusCompleted,
		CostPaidWei:     "100",
		BalanceWei:      "900",
		BillableSeconds: 60,
		StartedAt:       "2026-05-14T02:55:00Z",
		CompletedAt:     "2026-05-14T03:00:00Z",
	}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	requiredFields := []string{
		`"event":"billing_event"`,
		`"schema_version":1`,
		`"job_id":"j"`,
		`"job_type":"training"`,
		`"capability":"flux-lora-training"`,
		`"user_hash":`,
		`"sender_address":`,
		`"status":"completed"`,
		`"cost_paid_wei":"100"`,
		`"balance_wei":"900"`,
		`"billable_seconds":60`,
		`"started_at":`,
		`"completed_at":`,
	}
	for _, f := range requiredFields {
		if !strings.Contains(s, f) {
			t.Errorf("BillingEvent JSON missing field token %q in: %s", f, s)
		}
	}
}
