package runner

import (
	"context"
	"math/big"
	"testing"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
)

type stubPriceFeedWatcher struct {
	price eth.PriceData
}

func (s stubPriceFeedWatcher) Currencies() (string, string, error) {
	return "ETH", "USD", nil
}

func (s stubPriceFeedWatcher) Current() (eth.PriceData, error) {
	return s.price, nil
}

func (s stubPriceFeedWatcher) Subscribe(context.Context, chan<- eth.PriceData) {}

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
	runner := registry.runners[resp.RunnerID]
	if runner == nil || runner.Capacity != 1 || len(runner.InUse) != 0 {
		t.Fatalf("unexpected initial runner state: %+v", runner)
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
	runner = registry.runners[resp.RunnerID]
	if runner == nil || runner.Capacity != 2 || len(runner.InUse) != 1 {
		t.Fatalf("unexpected heartbeat runner state: %+v", runner)
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
	runner := registry.runners[resp.RunnerID]
	if runner == nil || runner.Capacity != 1 || len(runner.InUse) != 0 {
		t.Fatalf("unexpected state after create-by-unknown-id heartbeat: %+v", runner)
	}
}

func TestLiveRunnerRegistry_DefaultCapacityWhenUnset(t *testing.T) {
	registry := NewLiveRunnerRegistry()
	req := liveRunnerTestHeartbeat("runner_default_capacity")
	req.Capacity = 0

	resp, err := registry.Heartbeat(req)
	if err != nil {
		t.Fatal(err)
	}
	registry.mu.Lock()
	runner := registry.runners[resp.RunnerID]
	registry.mu.Unlock()
	if runner == nil || runner.Capacity != 1 {
		t.Fatalf("expected default capacity=1, got %+v", runner)
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

func TestLiveRunnerRegistry_ConvertsUSDToWEI(t *testing.T) {
	prevWatcher := core.PriceFeedWatcher
	core.PriceFeedWatcher = stubPriceFeedWatcher{price: eth.PriceData{Price: big.NewRat(2000, 1)}}
	defer func() { core.PriceFeedWatcher = prevWatcher }()

	registry := NewLiveRunnerRegistry()
	req := liveRunnerTestHeartbeat("runner-1")
	req.PriceInfo = LiveRunnerPriceInfo{PricePerUnit: 10, PixelsPerUnit: 1, Unit: "USD"}
	if _, err := registry.Heartbeat(req); err != nil {
		t.Fatal(err)
	}

	runners := registry.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected one runner, got %d", len(runners))
	}
	got := big.NewRat(runners[0].PriceInfo.PricePerUnit, runners[0].PriceInfo.PixelsPerUnit)
	expectedFixed, err := common.PriceToFixed(big.NewRat(5_000_000_000_000_000, 1280*720*30*3600))
	if err != nil {
		t.Fatal(err)
	}
	expected := big.NewRat(expectedFixed, 1)
	if got.Cmp(expected) != 0 {
		t.Fatalf("unexpected converted price: got=%s want=%s", got, expected)
	}
	if runners[0].PriceInfo.Unit != "WEI" {
		t.Fatalf("unexpected converted unit: %s", runners[0].PriceInfo.Unit)
	}
}

func TestLiveRunnerRegistry_SharedEndpointAppKeepsPerRunnerPrices(t *testing.T) {
	registry := NewLiveRunnerRegistry()

	req1 := liveRunnerTestHeartbeat("runner-1")
	req1.PriceInfo = LiveRunnerPriceInfo{PricePerUnit: 10, PixelsPerUnit: 1, Unit: "WEI"}
	req2 := liveRunnerTestHeartbeat("runner-2")
	req2.PriceInfo = LiveRunnerPriceInfo{PricePerUnit: 20, PixelsPerUnit: 1, Unit: "WEI"}
	if _, err := registry.Heartbeat(req1); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Heartbeat(req2); err != nil {
		t.Fatal(err)
	}

	registry.Unregister("runner-1")
	runners := registry.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected one remaining runner, got %d", len(runners))
	}
	if got := runners[0].PriceInfo.PricePerUnit; got != 20 {
		t.Fatalf("expected remaining runner price to stay isolated, got %d", got)
	}
}
