package byoc

import (
	"math"
	"math/big"
	"sync"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddressBalances_CompareAndUpdateBalance(t *testing.T) {
	addr := ethcommon.BytesToAddress([]byte("foo"))
	mid := "some_id"
	node := mockJobLivepeerNode()
	node.Balances = core.NewAddressBalances(1 * time.Minute)
	defer node.Balances.StopCleanup()

	assert := assert.New(t)
	bso := &BYOCOrchestratorServer{
		node:         node,
		sharedBalMtx: &sync.Mutex{},
	}
	// Test 1: Balance doesn't exist - should initialize to 1 and then update to expected
	expected := big.NewRat(10, 1)
	minimumBal := big.NewRat(5, 1)
	current, diff, minimumBalCovered, resetToZero := compareAndUpdateBalance(bso, addr, mid, expected, minimumBal)

	assert.Zero(expected.Cmp(current), "Balance should be updated to expected value")
	assert.Zero(big.NewRat(10, 1).Cmp(diff), "Diff should be expected - initial (10 - 1)")
	assert.True(minimumBalCovered, "Minimum balance should be covered when going from 1 to 10")
	assert.False(resetToZero, "Should not be reset to zero")

	// Test 2: Expected > Current (Credit scenario)
	expected = big.NewRat(20, 1)
	minimumBal = big.NewRat(15, 1)
	current, diff, minimumBalCovered, resetToZero = compareAndUpdateBalance(bso, addr, mid, expected, minimumBal)

	assert.Zero(expected.Cmp(current), "Balance should be updated to expected value")
	assert.Zero(big.NewRat(10, 1).Cmp(diff), "Diff should be 20 - 10 = 10")
	assert.True(minimumBalCovered, "Minimum balance should be covered when crossing threshold")
	assert.False(resetToZero, "Should not be reset to zero")

	// Test 3: Expected < Current (Debit scenario)
	expected = big.NewRat(5, 1)
	minimumBal = big.NewRat(3, 1)
	current, diff, minimumBalCovered, resetToZero = compareAndUpdateBalance(bso, addr, mid, expected, minimumBal)

	assert.Zero(expected.Cmp(current), "Balance should be updated to expected value")
	assert.Zero(big.NewRat(-15, 1).Cmp(diff), "Diff should be 5 - 20 = -15")
	assert.True(minimumBalCovered, "Minimum balance should still be covered")
	assert.False(resetToZero, "Should not be reset to zero")

	// Test 4: Expected == Current (No change)
	expected = big.NewRat(5, 1)
	minimumBal = big.NewRat(3, 1)
	current, diff, minimumBalCovered, resetToZero = compareAndUpdateBalance(bso, addr, mid, expected, minimumBal)

	assert.Zero(expected.Cmp(current), "Balance should remain the same")
	assert.Zero(big.NewRat(0, 1).Cmp(diff), "Diff should be 0")
	assert.True(minimumBalCovered, "Minimum balance should still be covered")
	assert.False(resetToZero, "Should not be reset to zero")

	// Test 5: Reset to zero (current > 0, expected = 0)
	bso.node.Balances.Credit(addr, core.ManifestID(mid), big.NewRat(5, 1)) // Set current to 10
	expected = big.NewRat(0, 1)
	minimumBal = big.NewRat(3, 1)
	current, diff, minimumBalCovered, resetToZero = compareAndUpdateBalance(bso, addr, mid, expected, minimumBal)

	assert.Zero(expected.Cmp(current), "Balance should be reset to zero")
	assert.Zero(big.NewRat(-10, 1).Cmp(diff), "Diff should be 0 - 10 = -10")
	assert.False(minimumBalCovered, "Minimum balance should not be covered when resetting to zero")
	assert.True(resetToZero, "Should be marked as reset to zero")

	// Test 6: Minimum balance covered threshold - just below to just above
	expected = big.NewRat(2, 1)
	minimumBal = big.NewRat(5, 1)
	compareAndUpdateBalance(bso, addr, mid, expected, minimumBal) // Set to 2

	expected = big.NewRat(5, 1)
	current, diff, minimumBalCovered, resetToZero = compareAndUpdateBalance(bso, addr, mid, expected, minimumBal)

	assert.Zero(expected.Cmp(current), "Balance should be updated to 5")
	assert.Zero(big.NewRat(3, 1).Cmp(diff), "Diff should be 5 - 2 = 3")
	assert.True(minimumBalCovered, "Minimum balance should be covered when crossing from below to at threshold")
	assert.False(resetToZero, "Should not be reset to zero")

	// Test 7: Minimum balance not covered - already above threshold
	expected = big.NewRat(10, 1)
	minimumBal = big.NewRat(5, 1)
	current, diff, minimumBalCovered, resetToZero = compareAndUpdateBalance(bso, addr, mid, expected, minimumBal)

	assert.Zero(expected.Cmp(current), "Balance should be updated to 10")
	assert.Zero(big.NewRat(5, 1).Cmp(diff), "Diff should be 10 - 5 = 5")
	assert.True(minimumBalCovered, "Minimum balance should still be covered")
	assert.False(resetToZero, "Should not be reset to zero")

	// Test 8: Negative balance handling
	bso.node.Balances.Debit(addr, core.ManifestID(mid), big.NewRat(20, 1)) // Force negative: 10 - 20 = -10
	expected = big.NewRat(5, 1)
	minimumBal = big.NewRat(3, 1)
	current, diff, minimumBalCovered, resetToZero = compareAndUpdateBalance(bso, addr, mid, expected, minimumBal)

	assert.Zero(expected.Cmp(current), "Balance should be updated to expected value")
	assert.Zero(big.NewRat(15, 1).Cmp(diff), "Diff should be 5 - (-10) = 15")
	assert.True(minimumBalCovered, "Minimum balance should be covered when going from negative to positive above minimum")
	assert.False(resetToZero, "Should not be reset to zero")
}

func TestTicketCountForCost(t *testing.T) {
	tests := []struct {
		name           string
		cost           *big.Rat
		ticketEV       *big.Rat
		timeoutSeconds int64
		expectedCount  int64
	}{
		{
			name:           "ticketEV is zero",
			cost:           big.NewRat(100, 1),
			ticketEV:       big.NewRat(0, 1),
			timeoutSeconds: 60,
			expectedCount:  0,
		},
		{
			name:           "cost is zero",
			cost:           big.NewRat(0, 1),
			ticketEV:       big.NewRat(10, 1),
			timeoutSeconds: 60,
			expectedCount:  0,
		},
		{
			name:           "ticketEV less than cost - exact division",
			cost:           big.NewRat(100, 1),
			ticketEV:       big.NewRat(25, 1),
			timeoutSeconds: 60,
			expectedCount:  4,
		},
		{
			name:           "ticketEV less than cost - requires ceiling",
			cost:           big.NewRat(101, 1),
			ticketEV:       big.NewRat(25, 1),
			timeoutSeconds: 60,
			expectedCount:  5,
		},
		{
			name:           "ticketEV greater than cost",
			cost:           big.NewRat(50, 1),
			ticketEV:       big.NewRat(100, 1),
			timeoutSeconds: 60,
			expectedCount:  1,
		},
		{
			name:           "ticketEV much greater than cost",
			cost:           big.NewRat(10, 1),
			ticketEV:       big.NewRat(1000, 1),
			timeoutSeconds: 60,
			expectedCount:  1,
		},
		{
			name:           "ticketEV slightly greater than cost",
			cost:           big.NewRat(99, 1),
			ticketEV:       big.NewRat(100, 1),
			timeoutSeconds: 60,
			expectedCount:  1,
		},
		{
			name:           "exact division with large numbers",
			cost:           big.NewRat(1000000, 1),
			ticketEV:       big.NewRat(10000, 1),
			timeoutSeconds: 60,
			expectedCount:  100,
		},
		{
			name:           "very small ticketEV relative to cost",
			cost:           big.NewRat(100, 1),
			ticketEV:       big.NewRat(1, 100),
			timeoutSeconds: 60,
			expectedCount:  600,
		},
		{
			name:           "Zero returned with 0 ticketEV",
			cost:           big.NewRat(1, 1),
			ticketEV:       big.NewRat(0, 1),
			timeoutSeconds: 300,
			expectedCount:  0,
		},
		{
			name:           "result exceeds max limit of 600",
			cost:           big.NewRat(1000000, 1),
			ticketEV:       big.NewRat(1, 1),
			timeoutSeconds: 60,
			expectedCount:  600,
		},
		{
			name:           "result exactly at max limit of 600",
			cost:           big.NewRat(12000, 1),
			ticketEV:       big.NewRat(20, 1),
			timeoutSeconds: 60,
			expectedCount:  600,
		},
		{
			name:           "fractional result requiring ceiling",
			cost:           big.NewRat(125, 1),
			ticketEV:       big.NewRat(20, 1),
			timeoutSeconds: 60,
			expectedCount:  7,
		},
		{
			name:           "negative values should not occur but test robustness",
			cost:           big.NewRat(-100, 1),
			ticketEV:       big.NewRat(10, 1),
			timeoutSeconds: 60,
			expectedCount:  0,
		},
		{
			name:           "negative ticketEV should not occur but test robustness",
			cost:           big.NewRat(100, 1),
			ticketEV:       big.NewRat(-10, 1),
			timeoutSeconds: 60,
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ticketCountForCost(tt.cost, tt.ticketEV, tt.timeoutSeconds)
			assert.Equal(t, tt.expectedCount, result,
				"ticketCountForCost(cost=%v, ticketEV=%v, timeoutSeconds=%d) = %d, expected %d",
				tt.cost.FloatString(3), tt.ticketEV.FloatString(3), tt.timeoutSeconds, result, tt.expectedCount)
		})
	}
}

func TestTicketCountForCost_EdgeCases(t *testing.T) {
	t.Run("Very large numbers", func(t *testing.T) {
		// Test with extremely large numbers that would cause Float64 to fail
		// Use big.NewInt for large numbers and convert to Rat
		costInt := new(big.Int).SetInt64(math.MaxInt64)
		costInt.Mul(costInt, big.NewInt(2)) // Multiply to exceed int64 range
		cost := new(big.Rat).SetInt(costInt)
		ticketEV := big.NewRat(1, 1)
		timeoutSeconds := int64(100)

		result := ticketCountForCost(cost, ticketEV, timeoutSeconds)
		// Should fallback to timeoutSeconds when float64 conversion fails
		assert.Equal(t, int64(600), result)
	})

	t.Run("Very small numbers requiring high precision", func(t *testing.T) {
		cost := big.NewRat(1, 1000000)     // Very small cost
		ticketEV := big.NewRat(1, 2000000) // Even smaller EV
		timeoutSeconds := int64(60)

		result := ticketCountForCost(cost, ticketEV, timeoutSeconds)
		// Should calculate 2 and ceiling to 1
		assert.Equal(t, int64(2), result)
	})

	t.Run("Equal cost and ticketEV", func(t *testing.T) {
		cost := big.NewRat(100, 1)
		ticketEV := big.NewRat(100, 1)
		timeoutSeconds := int64(60)

		result := ticketCountForCost(cost, ticketEV, timeoutSeconds)
		// Should return 1 since 100/100 = 1.0
		assert.Equal(t, int64(1), result)
	})

	t.Run("Cost much smaller than EV", func(t *testing.T) {
		cost := big.NewRat(1, 1)
		ticketEV := big.NewRat(1000, 1)
		timeoutSeconds := int64(60)

		result := ticketCountForCost(cost, ticketEV, timeoutSeconds)
		// Should return 1 since any positive cost requires at least 1 ticket
		assert.Equal(t, int64(1), result)
	})
}

func TestTicketCountForCost_BoundaryConditions(t *testing.T) {
	t.Run("Exactly 600 tickets needed", func(t *testing.T) {
		cost := big.NewRat(600, 1)
		ticketEV := big.NewRat(1, 1)
		timeoutSeconds := int64(60)

		result := ticketCountForCost(cost, ticketEV, timeoutSeconds)
		assert.Equal(t, int64(600), result)
	})

	t.Run("One ticket over 600 limit", func(t *testing.T) {
		cost := big.NewRat(601, 1)
		ticketEV := big.NewRat(1, 1)
		timeoutSeconds := int64(60)

		result := ticketCountForCost(cost, ticketEV, timeoutSeconds)
		// Should cap at 600 even though actual calculation would be 601
		assert.Equal(t, int64(600), result)
	})

	t.Run("Zero timeoutSeconds", func(t *testing.T) {
		cost := big.NewRat(100, 1)
		ticketEV := big.NewRat(10, 1)
		timeoutSeconds := int64(0)

		result := ticketCountForCost(cost, ticketEV, timeoutSeconds)
		// Should return 10 (cost/ticketEV) since float conversion succeeds
		assert.Equal(t, int64(10), result)
	})
}

func TestTicketCountForCost_RealWorldScenarios(t *testing.T) {
	t.Run("Typical payment scenario - EV less than cost", func(t *testing.T) {
		// Simulate a scenario where expected value is less than total cost
		cost := big.NewRat(500000000000000000, 1)    // 0.5 ETH in wei
		ticketEV := big.NewRat(10000000000000000, 1) // 0.01 ETH in wei per ticket
		timeoutSeconds := int64(300)                 // 5 minutes

		result := ticketCountForCost(cost, ticketEV, timeoutSeconds)
		// Should calculate 50 tickets (0.5 / 0.01 = 50)
		assert.Equal(t, int64(50), result)
	})

	t.Run("High value tickets - EV greater than cost", func(t *testing.T) {
		// Scenario where each ticket has high expected value
		cost := big.NewRat(100000000000000000, 1)     // 0.1 ETH in wei
		ticketEV := big.NewRat(500000000000000000, 1) // 0.5 ETH in wei per ticket
		timeoutSeconds := int64(60)

		result := ticketCountForCost(cost, ticketEV, timeoutSeconds)
		// Should return 1 since cost < ticketEV
		assert.Equal(t, int64(1), result)
	})

	t.Run("Many small tickets needed", func(t *testing.T) {
		// Scenario requiring many tickets but under the 600 limit
		cost := big.NewRat(1000000000000000000, 1)  // 1 ETH in wei
		ticketEV := big.NewRat(2000000000000000, 1) // 0.002 ETH in wei per ticket
		timeoutSeconds := int64(60)

		result := ticketCountForCost(cost, ticketEV, timeoutSeconds)
		// Should calculate 500 tickets (1 / 0.002 = 500)
		assert.Equal(t, int64(500), result)
	})

	t.Run("Exceeds max tickets limit", func(t *testing.T) {
		// Scenario that would require more than 600 tickets
		// Use big.NewInt for large numbers
		costInt := new(big.Int).SetInt64(1000000000000000) // 1e15
		costInt.Mul(costInt, big.NewInt(10000))            // Multiply to get 1e19
		cost := new(big.Rat).SetInt(costInt)
		ticketEV := big.NewRat(1000000000000000, 1) // 0.001 ETH in wei per ticket
		timeoutSeconds := int64(60)

		result := ticketCountForCost(cost, ticketEV, timeoutSeconds)
		// Should cap at 600 (10000 / 0.001 = 10000, but capped at 600)
		assert.Equal(t, int64(600), result)
	})
}

func TestTicketCountForCost_CalculationAccuracy(t *testing.T) {
	require := require.New(t)

	t.Run("Fractional calculations are properly ceilinged", func(t *testing.T) {
		testCases := []struct {
			cost       *big.Rat
			ticketEV   *big.Rat
			timeout    int64
			exactRatio float64
			expected   int64
		}{
			{big.NewRat(10, 1), big.NewRat(3, 1), 60, 3.333, 4},
			{big.NewRat(15, 1), big.NewRat(4, 1), 60, 3.75, 4},
			{big.NewRat(16, 1), big.NewRat(4, 1), 60, 4.0, 4},
			{big.NewRat(17, 1), big.NewRat(4, 1), 60, 4.25, 5},
			{big.NewRat(100, 1), big.NewRat(30, 1), 60, 3.333, 4},
		}

		for _, tc := range testCases {
			result := ticketCountForCost(tc.cost, tc.ticketEV, tc.timeout)
			require.Equal(tc.expected, result,
				"Cost=%v, EV=%v should ratio=%.3f, result=%d, expected=%d",
				tc.cost.FloatString(2), tc.ticketEV.FloatString(2),
				tc.exactRatio, result, tc.expected)
		}
	})

	t.Run("Integer divisions work correctly", func(t *testing.T) {
		testCases := []*big.Rat{
			big.NewRat(100, 1), // 100 / 10 = 10
			big.NewRat(200, 1), // 200 / 20 = 10
			big.NewRat(300, 1), // 300 / 30 = 10
		}

		timeout := int64(60)

		for _, cost := range testCases {
			ticketEv := new(big.Rat).Quo(cost, big.NewRat(10, 1))
			result := ticketCountForCost(cost, ticketEv, timeout)
			require.Equal(int64(10), result)
		}
	})

	t.Run("Very precise fractional calculations", func(t *testing.T) {
		// Test with numbers that create very precise fractional results
		cost := big.NewRat(7, 1)
		ticketEV := big.NewRat(3, 1)
		timeout := int64(60)

		result := ticketCountForCost(cost, ticketEV, timeout)
		// 7/3 = 2.333..., should ceiling to 3
		require.Equal(int64(3), result)
	})
}
