package byoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestTrainingStoreInMemory verifies backward compat — store with no
// checkpoint dir works identically to the pre-PR-6 in-memory store.
func TestTrainingStoreInMemory(t *testing.T) {
	s := NewTrainingJobStore(1 * time.Hour)
	job := &TrainingJob{
		JobID:      "abc",
		Capability: "flux-lora-training",
		Status:     TrainingStatusSubmitted,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	s.Store(job)
	got, ok := s.Get("abc")
	if !ok || got.JobID != "abc" {
		t.Fatalf("get after store: ok=%v job=%+v", ok, got)
	}
	if !s.Update("abc", TrainingStatusRunning, 50, nil, "") {
		t.Fatal("update failed")
	}
	got, _ = s.Get("abc")
	if got.Status != TrainingStatusRunning || got.Progress != 50 {
		t.Fatalf("update did not apply: %+v", got)
	}
}

// TestTrainingStoreCheckpoint verifies Store + Update persist to disk
// and Get reads from in-memory after recovery.
func TestTrainingStoreCheckpoint(t *testing.T) {
	dir := t.TempDir()
	s, err := NewTrainingJobStoreWithCheckpoint(1*time.Hour, dir)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	job := &TrainingJob{
		JobID:      "persist-1",
		Capability: "flux-lora-training",
		Status:     TrainingStatusRunning,
		Progress:   25,
		Cost:       "0",
		Balance:    "1000",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	s.Store(job)

	// Verify file exists with correct content
	path := filepath.Join(dir, "persist-1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read checkpoint: %v", err)
	}
	var disk TrainingJob
	if err := json.Unmarshal(data, &disk); err != nil {
		t.Fatalf("parse checkpoint: %v", err)
	}
	if disk.JobID != "persist-1" || disk.Status != TrainingStatusRunning || disk.Progress != 25 {
		t.Fatalf("disk shape mismatch: %+v", disk)
	}

	// Update and verify disk reflects the new state
	s.Update("persist-1", TrainingStatusCompleted, 100, nil, "")
	data2, _ := os.ReadFile(path)
	var disk2 TrainingJob
	_ = json.Unmarshal(data2, &disk2)
	if disk2.Status != TrainingStatusCompleted || disk2.Progress != 100 {
		t.Fatalf("update not persisted: %+v", disk2)
	}
}

// TestTrainingStoreSweepRecoversInflightAsFailed — the load-bearing
// invariant I8 check. After a restart, jobs that were running become
// failed_orchestrator_restart.
func TestTrainingStoreSweepRecoversInflightAsFailed(t *testing.T) {
	dir := t.TempDir()

	// Simulate state from a prior orch run by manually writing checkpoints
	preCrashJob := TrainingJob{
		JobID:      "inflight-1",
		Capability: "flux-lora-training",
		Status:     TrainingStatusRunning,
		Progress:   50,
		CreatedAt:  time.Now().Add(-10 * time.Minute),
		UpdatedAt:  time.Now().Add(-1 * time.Minute),
	}
	data, _ := json.Marshal(preCrashJob)
	if err := os.WriteFile(filepath.Join(dir, "inflight-1.json"), data, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	terminalJob := TrainingJob{
		JobID:     "done-1",
		Status:    TrainingStatusCompleted,
		Progress:  100,
		UpdatedAt: time.Now().Add(-30 * time.Minute),
	}
	data2, _ := json.Marshal(terminalJob)
	_ = os.WriteFile(filepath.Join(dir, "done-1.json"), data2, 0o644)

	// New store starts up
	s, err := NewTrainingJobStoreWithCheckpoint(1*time.Hour, dir)
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	// inflight-1 should now be failed_orchestrator_restart
	got, ok := s.Get("inflight-1")
	if !ok {
		t.Fatal("inflight-1 missing after sweep")
	}
	if got.Status != "failed_orchestrator_restart" {
		t.Fatalf("inflight-1 status = %q, want failed_orchestrator_restart", got.Status)
	}
	if got.Error == "" {
		t.Fatal("inflight-1 has no error message")
	}

	// terminal job is untouched
	doneGot, _ := s.Get("done-1")
	if doneGot.Status != TrainingStatusCompleted {
		t.Fatalf("done-1 status corrupted: %q", doneGot.Status)
	}

	// And the inflight checkpoint file is rewritten with the new status
	rewriteData, _ := os.ReadFile(filepath.Join(dir, "inflight-1.json"))
	var rewrite TrainingJob
	_ = json.Unmarshal(rewriteData, &rewrite)
	if rewrite.Status != "failed_orchestrator_restart" {
		t.Fatalf("inflight-1.json not rewritten: %q", rewrite.Status)
	}
}

// TestTrainingStoreCorruptCheckpointSkipped — a malformed JSON file
// should be logged + skipped, not panic the orch.
func TestTrainingStoreCorruptCheckpointSkipped(t *testing.T) {
	dir := t.TempDir()
	good := TrainingJob{JobID: "good", Status: TrainingStatusCompleted, UpdatedAt: time.Now()}
	gd, _ := json.Marshal(good)
	_ = os.WriteFile(filepath.Join(dir, "good.json"), gd, 0o644)
	_ = os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not-json{{{"), 0o644)

	s, err := NewTrainingJobStoreWithCheckpoint(1*time.Hour, dir)
	if err != nil {
		t.Fatalf("init should not fail on corrupt file: %v", err)
	}
	if _, ok := s.Get("good"); !ok {
		t.Fatal("good job not loaded")
	}
	if _, ok := s.Get("bad"); ok {
		t.Fatal("bad job was loaded somehow")
	}
}

// TestTrainingStoreConcurrentUpdatesPreserveOrder — the load-bearing
// invariant from review C1: concurrent Update calls to the same job
// must not write checkpoints out-of-order. The earlier impl released
// `mu` after snapshot and re-acquired `checkpointMu` for fs write —
// allowing a slow writer of an earlier snapshot to overwrite a faster
// writer of a later snapshot. End state on disk would be stale.
//
// This test races 100 status transitions submitted→running→completed
// against the same job, then asserts: final disk state == final memory
// state. With the fix (checkpointMu held across mutation+write), the
// last Update's snapshot is always the last write.
func TestTrainingStoreConcurrentUpdatesPreserveOrder(t *testing.T) {
	dir := t.TempDir()
	s, err := NewTrainingJobStoreWithCheckpoint(1*time.Hour, dir)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	job := &TrainingJob{
		JobID:     "race-1",
		Status:    TrainingStatusSubmitted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.Store(job)

	var wg sync.WaitGroup
	const N = 100
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(progress int) {
			defer wg.Done()
			status := TrainingStatusRunning
			if progress == N-1 {
				status = TrainingStatusCompleted
			}
			s.Update("race-1", status, progress, nil, "")
		}(i)
	}
	wg.Wait()

	// Final in-memory state
	memJob, _ := s.Get("race-1")

	// Final disk state
	data, err := os.ReadFile(filepath.Join(dir, "race-1.json"))
	if err != nil {
		t.Fatalf("read disk: %v", err)
	}
	var diskJob TrainingJob
	if err := json.Unmarshal(data, &diskJob); err != nil {
		t.Fatalf("parse disk: %v", err)
	}

	// Invariant: disk state matches memory state. If write-reordering
	// was happening, disk would show an arbitrary intermediate state
	// while memory showed the last update.
	if diskJob.Status != memJob.Status {
		t.Errorf("status drift: disk=%q memory=%q (race in checkpoint write order)",
			diskJob.Status, memJob.Status)
	}
	if diskJob.Progress != memJob.Progress {
		t.Errorf("progress drift: disk=%d memory=%d",
			diskJob.Progress, memJob.Progress)
	}
}

// TestTrainingStoreAtomicWriteOnRestart — interrupted write leaves a .tmp
// behind. Sweep should skip the .tmp (only .json files load).
func TestTrainingStoreAtomicWriteIgnoresTmp(t *testing.T) {
	dir := t.TempDir()
	// Simulate a previously-interrupted write
	_ = os.WriteFile(filepath.Join(dir, "interrupted.json.tmp"), []byte("garbage"), 0o644)
	good := TrainingJob{JobID: "ok", Status: TrainingStatusCompleted, UpdatedAt: time.Now()}
	gd, _ := json.Marshal(good)
	_ = os.WriteFile(filepath.Join(dir, "ok.json"), gd, 0o644)

	s, err := NewTrainingJobStoreWithCheckpoint(1*time.Hour, dir)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, ok := s.Get("ok"); !ok {
		t.Fatal("ok job missing")
	}
	// .tmp file is not loaded as a job (sweep filters on .json suffix)
	if _, ok := s.Get("interrupted"); ok {
		t.Fatal(".tmp file was loaded as a job")
	}
}
