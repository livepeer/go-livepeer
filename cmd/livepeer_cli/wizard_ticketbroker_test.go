package main

import (
	"math/big"
	"testing"

	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
)

func createSender(deposit *big.Int, reserve *big.Int, withdrawBlock *big.Int) (sender pm.SenderInfo) {
	sender.Deposit = deposit
	sender.Reserve = reserve
	sender.WithdrawBlock = withdrawBlock

	return
}

func TestSenderStatus(t *testing.T) {
	assert := assert.New(t)

	// Test Empty
	s := createSender(big.NewInt(0), big.NewInt(0), big.NewInt(0))
	ss := senderStatus(s, big.NewInt(0))
	assert.Equal(Empty, ss)

	// Test Empty, but withdrawBlock > 0
	s = createSender(big.NewInt(0), big.NewInt(0), big.NewInt(5))
	ss = senderStatus(s, big.NewInt(0))
	assert.Equal(Empty, ss)

	// Test Unlocked when withdrawBlock = currentBlock
	s = createSender(big.NewInt(7), big.NewInt(0), big.NewInt(5))
	ss = senderStatus(s, big.NewInt(5))
	assert.Equal(Unlocked, ss)

	// Test Unlocked when withdrawBlock < currentBlock
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
