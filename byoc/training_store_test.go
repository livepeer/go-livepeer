package byoc

import (
	"encoding/json"
	"os"
	"path/filepath"
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
