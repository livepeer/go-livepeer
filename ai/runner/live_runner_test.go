package runner

import "testing"

func liveRunnerTestHeartbeat(runnerID string) LiveRunnerHeartbeatRequest {
	return LiveRunnerHeartbeatRequest{
		RunnerID:  runnerID,
		Label:     "test-runner",
		RunnerURL: "https://runner.example.com",
		Version:   "1.2.3",
		Status:    "ready",
		GPU: &LiveRunnerGPU{
			ID:     "gpu-0",
			Name:   "NVIDIA L40S",
			VRAMMB: 46068,
		},
		App:      "new-ai-pipeline/model-a",
		Capacity: 1,
		PriceInfo: LiveRunnerPriceInfo{
			PricePerUnit:  1250,
			PixelsPerUnit: 1,
			Unit:          "USD",
		},
	}
}

func TestLiveRunnerRegistry_HeartbeatUpsertCapacity(t *testing.T) {
	registry := NewLiveRunnerRegistry()

	resp, err := registry.Heartbeat(liveRunnerTestHeartbeat(""))
	if err != nil {
		t.Fatal(err)
	}
	if resp.RunnerID == "" {
		t.Fatal("expected runner id")
	}
	if got := registry.GetCapacity("new-ai-pipeline", "model-a"); got.ContainersIdle != 1 || got.ContainersInUse != 0 {
		t.Fatalf("unexpected initial capacity: %+v", got)
	}

	_, err = registry.Heartbeat(LiveRunnerHeartbeatRequest{
		RunnerID:  resp.RunnerID,
		Label:     "test-runner",
		RunnerURL: "https://runner.example.com",
		Status:    "ready",
		App:       "new-ai-pipeline/model-a",
		Capacity:  2,
		InUse:     []string{"job-1"},
		PriceInfo: LiveRunnerPriceInfo{
			PricePerUnit:  1250,
			PixelsPerUnit: 1,
			Unit:          "USD",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.GetCapacity("new-ai-pipeline", "model-a"); got.ContainersIdle != 1 || got.ContainersInUse != 1 {
		t.Fatalf("unexpected heartbeat capacity: %+v", got)
	}
}

func TestLiveRunnerRegistry_HeartbeatUnknownIDCreatesRunner(t *testing.T) {
	registry := NewLiveRunnerRegistry()
	req := liveRunnerTestHeartbeat("runner_custom_1")

	resp, err := registry.Heartbeat(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.RunnerID != "runner_custom_1" {
		t.Fatalf("expected supplied runner id to be used, got %s", resp.RunnerID)
	}
	if got := registry.GetCapacity("new-ai-pipeline", "model-a"); got.ContainersIdle != 1 || got.ContainersInUse != 0 {
		t.Fatalf("unexpected capacity after create-by-unknown-id heartbeat: %+v", got)
	}
}

func TestLiveRunnerRegistry_RunnersDiscoveryShape(t *testing.T) {
	registry := NewLiveRunnerRegistry()

	if _, err := registry.Heartbeat(liveRunnerTestHeartbeat("runner_discovery_1")); err != nil {
		t.Fatal(err)
	}

	runners := registry.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected one runner, got %d", len(runners))
	}
	runner := runners[0]
	if runner.Endpoint != "https://runner.example.com" {
		t.Fatalf("unexpected endpoint: %s", runner.Endpoint)
	}
	if runner.GPU == nil || runner.GPU.Name != "NVIDIA L40S" {
		t.Fatalf("unexpected gpu: %+v", runner.GPU)
	}
	if runner.App != "new-ai-pipeline/model-a" {
		t.Fatalf("unexpected app: %s", runner.App)
	}
	if runner.PriceInfo.Unit != "USD" {
		t.Fatalf("unexpected runner price info: %+v", runner.PriceInfo)
	}
	if runner.Version != "1.2.3" {
		t.Fatalf("unexpected runner version: %s", runner.Version)
	}
}
