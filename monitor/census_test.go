package monitor

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	lpnet "github.com/livepeer/go-livepeer/net"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAveragerCanBeRemoved(t *testing.T) {
	a1 := newAverager("test")
	if !a1.canBeRemoved() {
		t.Fatal("Should be able to remove empty buffer")
	}
	a1.addEmerged(1)
	time.Sleep(time.Millisecond)
	a1.addEmerged(2)
	rate, has := a1.successRate()
	if rate != 1 {
		t.Fatalf("Rate should be 1, got %v", rate)
	}
	if has {
		t.Fatalf("Rate shouldn't be found at this point")
	}
	if a1.canBeRemoved() {
		t.Fatal("Should not be able to remove buffer with not transcoded segments till timeout passes")
	}
	a1.segments[0].transcoded = 1
	a1.segments[1].failed = true
	if !a1.canBeRemoved() {
		t.Fatal("Should be able to remove buffer with all transcoded segments")
	}
	a2 := newAverager("test")
	a2.addEmerged(1)
	old := timeToWaitForError
	timeToWaitForError = time.Millisecond
	time.Sleep(10 * time.Millisecond)
	if !a2.canBeRemoved() {
		t.Fatal("Should be able to remove buffer with timeouted segments")
	}
	timeToWaitForError = old
}

func TestLastSegmentTimeout(t *testing.T) {
	unitTestMode = true
	defer func() { unitTestMode = false }()
	NodeID = "testid"

	InitCensus("bctr", "testversion")
	// defer func() {
	// 	shutDown <- nil
	// }()
	StreamCreated("h1", 1)
	if len(census.success) != 1 {
		t.Fatal("Should be one stream")
	}
	SegmentEmerged(context.TODO(), 1, 1, 3, 1)
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	SegmentFullyTranscoded(context.Background(), 1, 1, "ps", "", &lpnet.OrchestratorInfo{})
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	SegmentEmerged(context.TODO(), 1, 2, 3, 1)
	SegmentTranscodeFailed(context.TODO(), SegmentTranscodeErrorOrchestratorBusy, 1, 2, fmt.Errorf("some"), true)
	if sr := census.successRate(); sr != 0.5 {
		t.Fatalf("Success rate should be 0.5, not %f", sr)
	}
	SegmentEmerged(context.TODO(), 1, 3, 3, 1)
	SegmentTranscodeFailed(context.TODO(), SegmentTranscodeErrorSessionEnded, 1, 3, fmt.Errorf("some"), true)
	SegmentEmerged(context.TODO(), 1, 4, 3, 1)
	SegmentFullyTranscoded(context.Background(), 1, 4, "ps", "", &lpnet.OrchestratorInfo{})
	if sr := census.successRate(); sr != 0.75 {
		t.Fatalf("Success rate should be 0.75, not %f", sr)
	}
	StreamEnded(context.TODO(), 1)
	if len(census.success) != 0 {
		t.Fatalf("Should be no streams, instead have %d", len(census.success))
	}

	StreamCreated("h1", 2)
	SegmentEmerged(context.TODO(), 2, 1, 3, 1)
	SegmentFullyTranscoded(context.Background(), 2, 1, "ps", "", &lpnet.OrchestratorInfo{})
	SegmentEmerged(context.TODO(), 2, 2, 3, 1)
	StreamEnded(context.TODO(), 2)
	if len(census.success) != 1 {
		t.Fatalf("Should be one stream, instead have %d", len(census.success))
	}
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	old1 := timeToWaitForError
	timeToWaitForError = time.Nanosecond
	if sr := census.successRate(); sr != 0.5 {
		t.Fatalf("Success rate should be 0.5, not %f", sr)
	}
	go census.timeoutWatcher(context.Background())
	time.Sleep(10 * time.Millisecond)
	if len(census.success) != 0 {
		t.Fatalf("Should be streams, instead have %d", len(census.success))
	}
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	timeToWaitForError = old1

	StreamCreated("h3", 3)
	SegmentEmerged(context.TODO(), 3, 1, 3, 1)
	SegmentFullyTranscoded(context.Background(), 3, 1, "ps", "", &lpnet.OrchestratorInfo{})
	SegmentEmerged(context.TODO(), 3, 2, 3, 1)
	StreamEnded(context.TODO(), 3)
	if len(census.success) != 1 {
		t.Fatalf("Should be one stream, instead have %d", len(census.success))
	}
	if sr := census.successRate(); sr != 1 {
		t.Fatalf("Success rate should be 1, not %f", sr)
	}
	SegmentTranscodeFailed(context.TODO(), SegmentTranscodeErrorOrchestratorBusy, 3, 2, fmt.Errorf("some"), true)
	if sr := census.successRate(); sr != 0.5 {
		t.Fatalf("Success rate should be 0.5, not %f", sr)
	}
}

func TestWei2Gwei(t *testing.T) {
	assert := assert.New(t)

	wei := big.NewInt(gweiConversionFactor)
	assert.Equal(1.0, wei2gwei(wei))

	wei = big.NewInt(gweiConversionFactor / 2)
	assert.Equal(0.5, wei2gwei(wei))

	wei = big.NewInt(0)
	assert.Equal(0.0, wei2gwei(wei))

	wei = big.NewInt(gweiConversionFactor * 1.5)
	assert.Equal(1.5, wei2gwei(wei))

	wei = big.NewInt(gweiConversionFactor * 2)
	assert.Equal(2.0, wei2gwei(wei))
}

func TestFracWei2Gwei(t *testing.T) {
	assert := assert.New(t)

	wei := big.NewRat(gweiConversionFactor, 1)
	assert.Equal(1.0, fracwei2gwei(wei))

	wei = big.NewRat(gweiConversionFactor/2, 1)
	assert.Equal(0.5, fracwei2gwei(wei))

	wei = big.NewRat(0, 1)
	assert.Equal(0.0, fracwei2gwei(wei))

	wei = big.NewRat(gweiConversionFactor*1.5, 1)
	assert.Equal(1.5, fracwei2gwei(wei))

	wei = big.NewRat(gweiConversionFactor*2, 1)
	assert.Equal(2.0, fracwei2gwei(wei))

	delta := .000000001

	// Test wei amounts with a fractional part
	wei = big.NewRat(gweiConversionFactor/2, 6)
	assert.InDelta(.083333333, fracwei2gwei(wei), delta)

	wei = big.NewRat(gweiConversionFactor*1.5, 7)
	assert.InDelta(.214285714, fracwei2gwei(wei), delta)

	wei = big.NewRat(gweiConversionFactor*2, 7)
	assert.InDelta(.285714286, fracwei2gwei(wei), delta)
}

func TestLiveRunnerMetrics(t *testing.T) {
	t.Run("requests", func(t *testing.T) {
		cleanup := setupLiveRunnerMetricsTest(t)
		defer cleanup()

		LiveRunnerRequest("runner-a")
		LiveRunnerRequest("runner-a")
		LiveRunnerRequest("")

		rows, err := view.RetrieveData(census.mLiveRunnerRequests.Name())
		require.NoError(t, err)
		assert.Equal(t, int64(2), countValueForTag(t, rows, census.kRunnerID.Name(), "runner-a"))
		assert.Equal(t, int64(1), countValueForTag(t, rows, census.kRunnerID.Name(), "unknown"))
	})

	t.Run("payment_received", func(t *testing.T) {
		cleanup := setupLiveRunnerMetricsTest(t)
		defer cleanup()

		LiveRunnerPaymentRecv("runner-a", big.NewRat(gweiConversionFactor*3, 2))
		LiveRunnerPaymentRecv("runner-a", big.NewRat(0, 1))
		LiveRunnerPaymentRecv("runner-a", nil)
		LiveRunnerPaymentRecv("", big.NewRat(gweiConversionFactor, 1))

		rows, err := view.RetrieveData(census.mLiveRunnerPaymentsRecv.Name())
		require.NoError(t, err)
		assert.InDelta(t, 1.5, sumValueForTag(t, rows, census.kRunnerID.Name(), "runner-a"), 0.000000001)
		assert.InDelta(t, 1.0, sumValueForTag(t, rows, census.kRunnerID.Name(), "unknown"), 0.000000001)
	})

	t.Run("payment_errors", func(t *testing.T) {
		cleanup := setupLiveRunnerMetricsTest(t)
		defer cleanup()

		LiveRunnerPaymentError("runner-b")
		LiveRunnerPaymentError("runner-b")
		LiveRunnerPaymentError("")

		rows, err := view.RetrieveData(census.mLiveRunnerPaymentErrors.Name())
		require.NoError(t, err)
		assert.Equal(t, int64(2), countValueForTag(t, rows, census.kRunnerID.Name(), "runner-b"))
		assert.Equal(t, int64(1), countValueForTag(t, rows, census.kRunnerID.Name(), "unknown"))
	})
}

func setupLiveRunnerMetricsTest(t *testing.T) func() {
	t.Helper()

	origEnabled := Enabled
	Enabled = true

	testName := strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_")
	testName = strings.ReplaceAll(testName, " ", "_")
	suffix := fmt.Sprintf("%s_%d", testName, time.Now().UnixNano())

	census.kRunnerID = tag.MustNewKey("runner_id")
	ctx, err := tag.New(context.Background())
	require.NoError(t, err)
	census.ctx = ctx

	census.mLiveRunnerRequests = stats.Int64("test_live_runner_requests_total_"+suffix, "test", "tot")
	census.mLiveRunnerPaymentsRecv = stats.Float64("test_live_runner_payment_received_"+suffix, "test", "gwei")
	census.mLiveRunnerPaymentErrors = stats.Int64("test_live_runner_payment_account_errors_"+suffix, "test", "tot")

	requestView := &view.View{
		Name:        census.mLiveRunnerRequests.Name(),
		Measure:     census.mLiveRunnerRequests,
		TagKeys:     []tag.Key{census.kRunnerID},
		Aggregation: view.Count(),
	}
	paymentView := &view.View{
		Name:        census.mLiveRunnerPaymentsRecv.Name(),
		Measure:     census.mLiveRunnerPaymentsRecv,
		TagKeys:     []tag.Key{census.kRunnerID},
		Aggregation: view.Sum(),
	}
	errorView := &view.View{
		Name:        census.mLiveRunnerPaymentErrors.Name(),
		Measure:     census.mLiveRunnerPaymentErrors,
		TagKeys:     []tag.Key{census.kRunnerID},
		Aggregation: view.Count(),
	}

	require.NoError(t, view.Register(requestView, paymentView, errorView))

	return func() {
		view.Unregister(requestView, paymentView, errorView)
		Enabled = origEnabled
	}
}

func countValueForTag(t *testing.T, rows []*view.Row, key, value string) int64 {
	t.Helper()

	for _, row := range rows {
		for _, tag := range row.Tags {
			if tag.Key.Name() == key && tag.Value == value {
				countData, ok := row.Data.(*view.CountData)
				require.True(t, ok)
				return countData.Value
			}
		}
	}
	return 0
}

func sumValueForTag(t *testing.T, rows []*view.Row, key, value string) float64 {
	t.Helper()

	for _, row := range rows {
		for _, tag := range row.Tags {
			if tag.Key.Name() == key && tag.Value == value {
				sumData, ok := row.Data.(*view.SumData)
				require.True(t, ok)
				return sumData.Value
			}
		}
	}
	return 0
}
