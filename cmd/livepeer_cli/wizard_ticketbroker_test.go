package main

import (
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
)

func createSender(deposit *big.Int, reserve *big.Int, withdrawRound *big.Int) (sender pm.SenderInfo) {
	sender.Deposit = deposit
	sender.WithdrawRound = withdrawRound
	sender.Reserve = &pm.ReserveInfo{
		FundsRemaining:        reserve,
		ClaimedInCurrentRound: big.NewInt(0),
	}

	return
}

func TestSenderStatus(t *testing.T) {
	assert := assert.New(t)

	// Test Empty
	s := createSender(big.NewInt(0), big.NewInt(0), big.NewInt(0))
	ss := senderStatus(s, big.NewInt(0))
	assert.Equal(Empty, ss)

	// Test Empty, but WithdrawRound > 0
	s = createSender(big.NewInt(0), big.NewInt(0), big.NewInt(5))
	ss = senderStatus(s, big.NewInt(0))
	assert.Equal(Empty, ss)

	// Test Unlocked when WithdrawRound = currentRound
	s = createSender(big.NewInt(7), big.NewInt(0), big.NewInt(5))
	ss = senderStatus(s, big.NewInt(5))
	assert.Equal(Unlocked, ss)

	// Test Unlocked when WithdrawRound < currentRound
	s = createSender(big.NewInt(7), big.NewInt(0), big.NewInt(5))
	ss = senderStatus(s, big.NewInt(6))
	assert.Equal(Unlocked, ss)

	// Test Unlocking
	s = createSender(big.NewInt(7), big.NewInt(0), big.NewInt(5))
	ss = senderStatus(s, big.NewInt(3))
	assert.Equal(Unlocking, ss)

	// Test Locked
	s = createSender(big.NewInt(7), big.NewInt(0), big.NewInt(0))
	ss = senderStatus(s, big.NewInt(3))
	assert.Equal(Locked, ss)
}

func TestHTTPPostWithStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantOK     bool
	}{
		{name: "success with empty body", statusCode: http.StatusNoContent, wantOK: true},
		{name: "success with body", statusCode: http.StatusOK, body: "accepted", wantOK: true},
		{name: "server error", statusCode: http.StatusInternalServerError, body: "failed", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			body, ok := httpPostWithStatus(server.URL)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.body, body)
		})
	}
}
