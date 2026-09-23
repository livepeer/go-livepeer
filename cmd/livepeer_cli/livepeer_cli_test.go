package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Entries are synthetic so adding a real menu item cannot break this test.
func TestFilterOptions(t *testing.T) {
	options := []wizardOpt{
		{desc: "always"},
		{desc: "orchestrator only", orchestrator: true},
		{desc: "reward", rewardCaller: true},
		{desc: "gateway only", notOrchestrator: true},
		{desc: "testnet only", testnet: true},
		// No real entry sets two gates; this pins which one wins.
		{desc: "both gates", orchestrator: true, notOrchestrator: true},
	}

	tests := []struct {
		name     string
		w        wizard
		expected []string
	}{
		{
			name:     "orchestrator",
			w:        wizard{orchestrator: true},
			expected: []string{"always", "orchestrator only", "reward", "both gates"},
		},
		{
			name:     "orchestrator on testnet",
			w:        wizard{orchestrator: true, testnet: true},
			expected: []string{"always", "orchestrator only", "reward", "testnet only", "both gates"},
		},
		{
			name:     "redeemer",
			w:        wizard{redeemer: true},
			expected: []string{"always", "orchestrator only", "reward", "both gates"},
		},
		{
			name:     "reward caller",
			w:        wizard{rewardCaller: true},
			expected: []string{"always", "reward"},
		},
		{
			name:     "gateway",
			w:        wizard{},
			expected: []string{"always", "gateway only"},
		},
		{
			name:     "testnet gateway",
			w:        wizard{testnet: true},
			expected: []string{"always", "gateway only", "testnet only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, opt := range tt.w.filterOptions(options) {
				got = append(got, opt.desc)
			}
			assert.Equal(t, tt.expected, got)
		})
	}
}
