package runner

import "testing"

func liveRunnerTestHeartbeat(runnerID string) LiveRunnerHeartbeatRequest {
	return LiveRunnerHeartbeatRequest{
		RunnerID:  runnerID,
		Label:     "test-runner",
		RunnerURL: "https://runner.example.com",
		Version:   "1.2.3",
		Status:    "ready",
		GPUs: []LiveRunnerGPU{{
			ID:     "gpu-0",
			Name:   "NVIDIA L40S",
			VRAMMB: 46068,
		}},
		PriceInfo: &LiveRunnerPriceInfo{
			PricePerUnit:  1000,
			PixelsPerUnit: 1,
		},
		Models: []LiveRunnerModel{{
			Pipeline: "new-ai-pipeline",
			Model:    "model-a",
			Capacity: 1,
		}},
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
		Models: []LiveRunnerModel{{
			Pipeline: "new-ai-pipeline",
			Model:    "model-a",
			Capacity: 2,
			InUse:    []string{"job-1"},
		}},
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
	if runner.PriceInfo == nil || runner.PriceInfo.PricePerUnit != 1000 || runner.PriceInfo.PixelsPerUnit != 1 {
		t.Fatalf("unexpected price info: %+v", runner.PriceInfo)
	}
	if len(runner.Capabilities) != 1 {
		t.Fatalf("expected one capability, got %d", len(runner.Capabilities))
	}
	capability := runner.Capabilities[0]
	if capability.Pipeline != "new-ai-pipeline" || capability.Model != "model-a" {
		t.Fatalf("unexpected capability identity: %+v", capability)
	}
	if capability.Capacity != 1 || capability.CapacityAvailable != 1 || capability.CapacityInUse != 0 {
		t.Fatalf("unexpected capability capacity: %+v", capability)
	}
	if capability.Version != "1.2.3" {
		t.Fatalf("unexpected capability version: %s", capability.Version)
	}
}

func TestLiveRunnerRegistry_SelectRunnerRequiresAvailableCapacity(t *testing.T) {
	registry := NewLiveRunnerRegistry()
	req := liveRunnerTestHeartbeat("runner_busy_1")
	req.Models[0].Capacity = 1
	req.Models[0].InUse = []string{"job-1"}

	if _, err := registry.Heartbeat(req); err != nil {
		t.Fatal(err)
	}

	if _, err := registry.SelectRunner("new-ai-pipeline", "model-a"); err != ErrNoLiveRunnerCapacity {
		t.Fatalf("expected no capacity error, got %v", err)
	}

	req.Models[0].Capacity = 2
	if _, err := registry.Heartbeat(req); err != nil {
		t.Fatal(err)
	}
	runner, err := registry.SelectRunner("new-ai-pipeline", "model-a")
	if err != nil {
		t.Fatal(err)
	}
	if runner.Endpoint != "https://runner.example.com" {
		t.Fatalf("unexpected selected endpoint: %s", runner.Endpoint)
	}
}
