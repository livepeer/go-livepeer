package pm

import (
	"crypto/hmac"
	"crypto/sha256"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRecipientFixtureOrFatal(t *testing.T) (ethcommon.Address, *stubBroker, *stubValidator, *stubGasPriceMonitor, *stubSenderMonitor, *stubTimeManager, TicketParamsConfig, []byte) {
	sender := RandAddress()

	b := newStubBroker()

	v := &stubValidator{}
	v.SetIsValidTicket(true)

	gm := &stubGasPriceMonitor{gasPrice: big.NewInt(100)}
	sm := newStubSenderMonitor()
	sm.maxFloat = big.NewInt(10000000000)
	tm := &stubTimeManager{lastSeenBlock: big.NewInt(1), round: big.NewInt(1), blkHash: RandHash(), preBlkHash: RandHash()}
	cfg := TicketParamsConfig{
		EV:               big.NewInt(5),
		RedeemGas:        10000,
		TxCostMultiplier: 100,
	}

	return sender, b, v, gm, sm, tm, cfg, []byte("foo")
}

func newRecipientOrFatal(t *testing.T, addr ethcommon.Address, b Broker, v Validator, gpm GasPriceMonitor, sm SenderMonitor, tm TimeManager, cfg TicketParamsConfig) Recipient {
	r, err := NewRecipient(addr, b, v, gpm, sm, tm, cfg)
	if err != nil {
		t.Fatal(err)
	}

	return r
}

func ticketParamsOrFatal(t *testing.T, r Recipient, sender ethcommon.Address) *TicketParams {
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	if err != nil {
		t.Fatal(err)
	}

	return params
}

func newTicket(sender ethcommon.Address, params *TicketParams, senderNonce uint32) *Ticket {
	return &Ticket{
		Recipient:              params.Recipient,
		Sender:                 sender,
		FaceValue:              params.FaceValue,
		WinProb:                params.WinProb,
		SenderNonce:            senderNonce,
		RecipientRandHash:      params.RecipientRandHash,
		ParamsExpirationBlock:  params.ExpirationBlock,
		PricePerPixel:          params.PricePerPixel,
		CreationRound:          params.ExpirationParams.CreationRound,
		CreationRoundBlockHash: params.ExpirationParams.CreationRoundBlockHash,
	}
}

func genRecipientRand(sender ethcommon.Address, secret [32]byte, params *TicketParams) *big.Int {
	h := hmac.New(sha256.New, secret[:])
	msg := append(params.Seed.Bytes(), sender.Bytes()...)
	msg = append(msg, params.FaceValue.Bytes()...)
	msg = append(msg, params.WinProb.Bytes()...)
	msg = append(msg, params.ExpirationBlock.Bytes()...)
	msg = append(msg, params.PricePerPixel.Num().Bytes()...)
	msg = append(msg, params.PricePerPixel.Denom().Bytes()...)
	msg = append(msg, params.ExpirationParams.AuxData()...)
	h.Write(msg)
	return new(big.Int).SetBytes(h.Sum(nil))
}

func TestReceiveTicket_InvalidRecipientRand(t *testing.T) {
	assert := assert.New(t)
	sender, b, _, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)

	sv := &stubSigVerifier{}
	sv.SetVerifyResult(true)
	v := NewValidator(sv, tm)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, gm, sm, tm, secret, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	// Test invalid faceValue
	ticket := newTicket(sender, params, 0)
	ticket.FaceValue = big.NewInt(0) // Using invalid FaceValue for generating recipientRand
	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	require.NotEqual(t, params.FaceValue, ticket.FaceValue)
	assert.Equal(sessionID, "")
	assert.Equal(won, false)
	assert.Equal(err.Error(), errInvalidTicketRecipientRand.Error())
	_, ok := err.(*FatalReceiveErr)
	assert.True(ok)

	// Test invalid winProb
	ticket = newTicket(sender, params, 0)
	ticket.WinProb = big.NewInt(0) // Using invalid WinProb for generating recipientRand
	sessionID, won, err = r.ReceiveTicket(ticket, sig, params.Seed)
	require.NotEqual(t, params.WinProb, ticket.WinProb)
	assert.Equal(sessionID, "")
	assert.Equal(won, false)
	assert.Equal(err.Error(), errInvalidTicketRecipientRand.Error())
	_, ok = err.(*FatalReceiveErr)
	assert.True(ok)

	// Test invalid ParamsExpirationBlock
	ticket = newTicket(sender, params, 0)
	ticket.ParamsExpirationBlock = big.NewInt(0) // Using invalid ParamsExpirationBlock for generating recipientRand
	sessionID, won, err = r.ReceiveTicket(ticket, sig, params.Seed)
	require.NotEqual(t, params.ExpirationBlock, ticket.ParamsExpirationBlock)
	assert.Equal(sessionID, "")
	assert.Equal(won, false)
	assert.Equal(err.Error(), errInvalidTicketRecipientRand.Error())
	_, ok = err.(*FatalReceiveErr)
	assert.True(ok)

	// Test invalid PricePerPixel
	ticket = newTicket(sender, params, 0)
	ticket.PricePerPixel = big.NewRat(0, 1) // Using invalid PricePerPixel for generating recipientRand
	sessionID, won, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.Equal(sessionID, "")
	assert.Equal(won, false)
	assert.Equal(err.Error(), errInvalidTicketRecipientRand.Error())
	_, ok = err.(*FatalReceiveErr)
	assert.True(ok)

	// Test invalid creation round
	ticket = newTicket(sender, params, 0)
	ticket.CreationRound = 999
	sessionID, won, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.Equal(sessionID, "")
	assert.Equal(won, false)
	assert.Equal(err.Error(), errInvalidTicketRecipientRand.Error())
	_, ok = err.(*FatalReceiveErr)
	assert.True(ok)

	// Test invalid creation round blockhash
	ticket = newTicket(sender, params, 0)
	ticket.CreationRoundBlockHash = RandHash()
	sessionID, won, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.Equal(sessionID, "")
	assert.Equal(won, false)
	assert.Equal(err.Error(), errInvalidTicketRecipientRand.Error())
	_, ok = err.(*FatalReceiveErr)
	assert.True(ok)
}

func TestReceiveTicket_InvalidSignature(t *testing.T) {
	assert := assert.New(t)
	sender, b, _, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)

	sv := &stubSigVerifier{}
	v := NewValidator(sv, tm)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, gm, sm, tm, secret, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	// Test invalid signature
	ticket := newTicket(sender, params, 0)
	sv.SetVerifyResult(false)
	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	assert.Equal(sessionID, "")
	assert.Equal(won, false)
	assert.Equal(err.Error(), errInvalidTicketSignature.Error())
	_, ok := err.(*FatalReceiveErr)
	assert.False(ok)
}

func TestReceiveTicket_InvalidSender(t *testing.T) {
	assert := assert.New(t)
	sender, b, v, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	sm.validateSenderErr = errors.New("Invalid Sender")
	r := newRecipientOrFatal(t, RandAddress(), b, v, gm, sm, tm, cfg)

	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	// Test valid non-winning ticket
	newSenderNonce := uint32(3)
	ticket := newTicket(sender, params, newSenderNonce)

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.EqualError(err, "Invalid Sender")
}

func TestReceiveTicket_ValidNonWinningTicket(t *testing.T) {
	sender, b, v, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, gm, sm, tm, secret, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	// Test valid non-winning ticket
	newSenderNonce := uint32(3)
	ticket := newTicket(sender, params, newSenderNonce)

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err != nil {
		t.Error(err)
	}
	if won {
		t.Errorf("expected valid non-winning ticket")
	}
	if sessionID != "" {
		t.Errorf("expected empty sessionID for valid non-winning ticket")
	}

	recipientRand := genRecipientRand(sender, secret, params)
	senderNonce := r.(*recipient).senderNonces[recipientRand.String()]

	if _, ok := senderNonce.nonceSeen[newSenderNonce]; !ok {
		t.Errorf("expected senderNonce to exist: %d", newSenderNonce)
	}
}

func TestReceiveTicket_ValidWinningTicket(t *testing.T) {
	sender, b, v, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, gm, sm, tm, secret, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	// Test valid winning ticket
	newSenderNonce := uint32(3)
	ticket := newTicket(sender, params, newSenderNonce)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err != nil {
		t.Error(err)
	}
	if !won {
		t.Errorf("expected valid winning ticket")
	}
	if sessionID != ticket.RecipientRandHash.Hex() {
		t.Errorf("expected sessionID %s, got %s", ticket.RecipientRandHash.Hex(), sessionID)
	}

	recipientRand := genRecipientRand(sender, secret, params)
	senderNonce := r.(*recipient).senderNonces[recipientRand.String()]

	if _, ok := senderNonce.nonceSeen[newSenderNonce]; !ok {
		t.Errorf("expected senderNonce to exist: %d", newSenderNonce)
	}
}

func TestReceiveTicket_InvalidSenderNonce(t *testing.T) {
	sender, b, v, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, gm, sm, tm, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	// Test invalid senderNonce
	// Receive senderNonce = 0
	ticket0 := newTicket(sender, params, 0)

	_, _, err = r.ReceiveTicket(ticket0, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}

	// Receive senderNonce = 1
	ticket1 := newTicket(sender, params, 1)

	_, _, err = r.ReceiveTicket(ticket1, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}

	// Replay senderNonce = 1 (new nonce = highest seen nonce)
	_, _, err = r.ReceiveTicket(ticket1, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid senderNonce (new nonce = highest seen nonce) error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid ticket senderNonce") {
		t.Errorf("expected invalid senderNonce (new nonce = highest seen nonce) error, got %v", err)
	}
	_, ok := err.(*FatalReceiveErr)
	assert.False(t, ok)
	// Replay senderNonce = 0 (new nonce < highest seen nonce)
	_, _, err = r.ReceiveTicket(ticket0, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid senderNonce (new nonce < highest seen nonce) error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid ticket senderNonce") {
		t.Errorf("expected invalid senderNonce (new nonce < highest seen nonce) error, got %v", err)
	}
	_, ok = err.(*FatalReceiveErr)
	assert.False(t, ok)
}

func TestReceiveTicket_ValidNonWinningTicket_Concurrent(t *testing.T) {
	assert := assert.New(t)
	sender, b, v, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, gm, sm, tm, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	var wg sync.WaitGroup
	var errCount uint64

	for i := 0; i < 50; i++ {
		wg.Add(1)

		go func(senderNonce uint32) {
			defer wg.Done()

			ticket := newTicket(sender, params, senderNonce)

			_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
			if err != nil {
				atomic.AddUint64(&errCount, 1)
			}
		}(uint32(i))
	}
	wg.Wait()
	assert.Zero(errCount)
}

func TestReceiveTicket_NonceMapFill(t *testing.T) {
	assert := assert.New(t)
	sender, b, v, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, gm, sm, tm, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)
	// fill nonce map to capacity
	for i := 0; i < maxSenderNonces+1; i++ {
		ticket := newTicket(sender, params, uint32(i))
		_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
		if i < maxSenderNonces {
			assert.NoError(err)
		} else {
			assert.Error(err)
		}
	}
}

func TestReceiveTicket_ValidTicket_Expired(t *testing.T) {
	assert := assert.New(t)
	sender, b, v, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, gm, sm, tm, secret, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	tm.lastSeenBlock = big.NewInt(100)
	require.Nil(t, err)

	// Test valid winning ticket
	newSenderNonce := uint32(3)
	ticket := newTicket(sender, params, newSenderNonce)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	assert.Equal(sessionID, params.RecipientRandHash.Hex())
	assert.True(won)
	assert.EqualError(err, ErrTicketParamsExpired.Error())
	_, ok := err.(*FatalReceiveErr)
	assert.False(ok)
}

func TestRedeemWinningTicket(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	sender, b, v, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, gm, sm, tm, secret, cfg).(*recipient)

	params := ticketParamsOrFatal(t, r, sender)
	ticket := newTicket(sender, params, 1)

	_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
	require.Nil(err)

	recipientRand := genRecipientRand(sender, secret, params)
	err = r.RedeemWinningTicket(ticket, sig, recipientRand)
	assert.Nil(err)

	assert.Equal(sm.queued[0].Ticket, ticket)
}

func TestTicketParams(t *testing.T) {
	sender, b, v, gm, sm, tm, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, gm, sm, tm, secret, cfg)

	require := require.New(t)
	assert := assert.New(t)

	// Test price = 0
	params, err := r.TicketParams(sender, big.NewRat(0, 1))
	assert.Nil(err)
	assert.Equal(big.NewInt(0), params.FaceValue)
	assert.Equal(big.NewInt(0), params.WinProb)

	// Test price < 0
	params, err = r.TicketParams(sender, big.NewRat(-1, 1))
	assert.Nil(err)
	assert.Equal(big.NewInt(0), params.FaceValue)
	assert.Equal(big.NewInt(0), params.WinProb)

	// Test SenderMonitor.MaxFloat() error
	sm.maxFloatErr = errors.New("MaxFloat error")
	_, err = r.TicketParams(sender, big.NewRat(1, 1))
	assert.EqualError(err, sm.maxFloatErr.Error())

	// Test correct params returned when default faceValue < maxFloat
	sm.maxFloatErr = nil
	params1, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(err)

	if params1.Recipient != recipient {
		t.Errorf("expected recipient %x got %x", recipient, params1.Recipient)
	}

	faceValue := big.NewInt(100000000)
	if params1.FaceValue.Cmp(faceValue) != 0 {
		t.Errorf("expected faceValue %d got %d", faceValue, params1.FaceValue)
	}

	winProb, _ := new(big.Int).SetString("5789604461865809771178549250434395392663499233282028201972879200395660", 10)
	if params1.WinProb.Cmp(winProb) != 0 {
		t.Errorf("expected winProb %d got %d", winProb, params1.WinProb)
	}

	// Might be slightly off due to truncation
	expEV := new(big.Int).Div(new(big.Int).Mul(faceValue, winProb), maxWinProb)
	assert.LessOrEqual(new(big.Int).Abs(new(big.Int).Sub(cfg.EV, expEV)).Int64(), int64(1))

	recipientRandHash := crypto.Keccak256Hash(ethcommon.LeftPadBytes(genRecipientRand(sender, secret, params1).Bytes(), uint256Size))

	if params1.RecipientRandHash != recipientRandHash {
		t.Errorf("expected recipientRandHash %x got %x", recipientRandHash, params1.RecipientRandHash)
	}

	assert.Equal(params1.ExpirationParams.CreationRound, tm.round.Int64())
	assert.Equal(params1.ExpirationParams.CreationRoundBlockHash.Bytes(), tm.blkHash[:])

	// Test expiration params from the previous round if the current round L1 block hash is zero
	tm.blkHash = [32]byte{}
	params, err = r.TicketParams(sender, big.NewRat(1, 1))
	assert.Equal(params.ExpirationParams.CreationRound, tm.round.Int64()-1)
	assert.Equal(params.ExpirationParams.CreationRoundBlockHash.Bytes(), tm.preBlkHash[:])

	// Test correct params returned and different seed + recipientRandHash
	params, err = r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(err)

	if params.Recipient != recipient {
		t.Errorf("expected recipient %x got %x", recipient, params.Recipient)
	}

	if params.FaceValue.Cmp(faceValue) != 0 {
		t.Errorf("expected faceValue %d got %d", faceValue, params.FaceValue)
	}

	if params.WinProb.Cmp(winProb) != 0 {
		t.Errorf("expected winProb %d got %d", winProb, params.WinProb)
	}

	if params.RecipientRandHash == params1.RecipientRandHash {
		t.Errorf("expected different recipientRandHash value for different params")
	}

	if params.Seed == params1.Seed {
		t.Errorf("expected different seed value for different params")
	}

	recipientRandHash = crypto.Keccak256Hash(ethcommon.LeftPadBytes(genRecipientRand(sender, secret, params).Bytes(), uint256Size))

	if params.RecipientRandHash != recipientRandHash {
		t.Errorf("expected recipientRandHash %x got %x", recipientRandHash, params.RecipientRandHash)
	}

	// Test correct params returned and different faceValue + winProb
	gm.gasPrice = big.NewInt(777)

	params, err = r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(err)

	faceValue = big.NewInt(777000000)
	assert.Equal(faceValue, params.FaceValue)

	winProb, _ = new(big.Int).SetString("745122839364969082519761808292714979750772102095499125093034646125570", 10)
	assert.Equal(winProb, params.WinProb)

	// Might be slightly off due to truncation
	expEV = new(big.Int).Div(new(big.Int).Mul(faceValue, winProb), maxWinProb)
	assert.LessOrEqual(new(big.Int).Abs(new(big.Int).Sub(cfg.EV, expEV)).Int64(), int64(1))

	// Test correct params returned when default faceValue > maxFloat
	sm.maxFloat = big.NewInt(10001)
	gm.gasPrice = big.NewInt(1)

	params, err = r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(err)

	faceValue = sm.maxFloat
	assert.Equal(faceValue, params.FaceValue)

	winProb, _ = new(big.Int).SetString("57890255593098787833002192285115442382396752657554526567072084795477017120", 10)
	assert.Equal(winProb, params.WinProb)

	// Might be slightly off due to truncation
	expEV = new(big.Int).Div(new(big.Int).Mul(faceValue, winProb), maxWinProb)
	assert.LessOrEqual(new(big.Int).Abs(new(big.Int).Sub(cfg.EV, expEV)).Int64(), int64(1))

	// Test insufficient sender reserve error due to maxFloat < EV
	sm.maxFloat = new(big.Int).Sub(cfg.EV, big.NewInt(1))
	_, err = r.TicketParams(sender, big.NewRat(1, 1))
	assert.EqualError(err, errInsufficientSenderReserve.Error())

	// Test faceValue < txCostWithGasPrice(current gasPrice) and faceValue > txCostWithGasPrice(avg gasPrice)
	// Set current gasPrice higher than avg gasPrice
	gm.gasPrice = new(big.Int).Add(avgGasPrice, big.NewInt(1))
	txCost := new(big.Int).Mul(big.NewInt(int64(cfg.RedeemGas)), gm.gasPrice)
	txCostAvgGasPrice := new(big.Int).Mul(big.NewInt(int64(cfg.RedeemGas)), avgGasPrice)
	sm.maxFloat = new(big.Int).Sub(txCost, big.NewInt(1))
	require.True(sm.maxFloat.Cmp(txCost) < 0)
	require.True(sm.maxFloat.Cmp(txCostAvgGasPrice) > 0)
	_, err = r.TicketParams(sender, big.NewRat(1, 1))
	assert.Nil(err)

	// Test faceValue < txCostWithGasPrice(current gasPrice) and faceValue = txCostWithGasPrice(avg gasPrice)
	sm.maxFloat = txCostAvgGasPrice
	require.True(sm.maxFloat.Cmp(txCost) < 0)
	require.True(sm.maxFloat.Cmp(txCostAvgGasPrice) == 0)
	_, err = r.TicketParams(sender, big.NewRat(1, 1))
	assert.Nil(err)

	// Test faceValue < txCostWithGasPrice(current gasPrice) and faceValue < txCostWithGasPrice(avg gasPrice)
	sm.maxFloat = new(big.Int).Sub(txCostAvgGasPrice, big.NewInt(1))
	require.True(sm.maxFloat.Cmp(txCost) < 0)
	require.True(sm.maxFloat.Cmp(txCostAvgGasPrice) < 0)
	_, err = r.TicketParams(sender, big.NewRat(1, 1))
	assert.EqualError(err, errInsufficientSenderReserve.Error())

	// Test lazy evaluation when faceValue > txCostWithGasPrice(current gasPrice)
	// Set current gasPrice lower than avg gasPrice
	gm.gasPrice = new(big.Int).Sub(avgGasPrice, big.NewInt(1))
	txCost = new(big.Int).Mul(big.NewInt(int64(cfg.RedeemGas)), gm.gasPrice)
	sm.maxFloat = new(big.Int).Add(txCost, big.NewInt(1))
	require.True(sm.maxFloat.Cmp(txCost) > 0)
	require.True(sm.maxFloat.Cmp(txCostAvgGasPrice) < 0)
	_, err = r.TicketParams(sender, big.NewRat(1, 1))
	assert.Nil(err)

	// Test lazy evaluation when faceValue = txCostWithGasPrice(current gasPrice)
	sm.maxFloat = txCost
	require.True(sm.maxFloat.Cmp(txCost) == 0)
	require.True(sm.maxFloat.Cmp(txCostAvgGasPrice) < 0)
	_, err = r.TicketParams(sender, big.NewRat(1, 1))
	assert.Nil(err)

	// Test default faceValue < EV and maxFloat > EV
	// Set gas price = 0 to set default faceValue = 0
	gm.gasPrice = big.NewInt(0)
	sm.maxFloat = maxWinProb // Set maxFloat to some really big number

	params, err = r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(err)

	assert.Equal(new(big.Int).Mul(cfg.EV, evMultiplier), params.FaceValue)
	expWinProb := calcWinProb(params.FaceValue, cfg.EV)
	assert.Equal(expWinProb, params.WinProb)
	// Test default faceValue < EV and maxFloat < EV
	sm.maxFloat = big.NewInt(0) // Set maxFloat to some value less than EV

	_, err = r.TicketParams(sender, big.NewRat(1, 1))
	assert.EqualError(err, errInsufficientSenderReserve.Error())
}

func TestTxCostMultiplier_UsingFaceValue_ReturnsDefaultMultiplier(t *testing.T) {
	sender, b, v, gm, sm, tm, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, gm, sm, tm, secret, cfg)

	mul, err := r.TxCostMultiplier(sender)
	assert.Nil(t, err)
	assert.Equal(t, big.NewRat(int64(cfg.TxCostMultiplier), 1), mul)
}

func TestTxCostMultiplier_UsingMaxFloat_ReturnsScaledMultiplier(t *testing.T) {
	sender, b, v, gm, sm, tm, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, gm, sm, tm, secret, cfg)

	txCost := new(big.Int).Mul(gm.gasPrice, big.NewInt(int64(cfg.RedeemGas)))
	sm.maxFloat = new(big.Int).Add(txCost, big.NewInt(1))
	expMul := new(big.Rat).SetFrac(sm.maxFloat, txCost)

	mul, err := r.TxCostMultiplier(sender)
	assert.Nil(t, err)
	assert.Equal(t, expMul, mul)
}

func TestTxCostMultiplier_MaxFloatError_ReturnsError(t *testing.T) {
	sender, b, v, gm, sm, tm, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, gm, sm, tm, secret, cfg)

	sm.maxFloatErr = errors.New("MaxFloat error")
	mul, err := r.TxCostMultiplier(sender)
	assert.Nil(t, mul)
	assert.EqualError(t, err, sm.maxFloatErr.Error())
}

func TestTxCostMultiplier_InsufficientReserve_ReturnsError(t *testing.T) {
	sender, b, v, gm, sm, tm, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, gm, sm, tm, secret, cfg)

	sm.maxFloat = big.NewInt(0) // Set maxFloat to some value less than EV

	mul, err := r.TxCostMultiplier(sender)
	assert.Nil(t, mul)
	assert.EqualError(t, err, errInsufficientSenderReserve.Error())
}

func TestTxCostMultiplier_ZeroTxCost_Returns_Zero(t *testing.T) {
	sender, b, v, gm, sm, tm, cfg, _ := newRecipientFixtureOrFatal(t)
	gm.gasPrice = big.NewInt(0)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, gm, sm, tm, secret, cfg)

	mul, err := r.TxCostMultiplier(sender)
	assert.Nil(t, err)
	assert.Equal(t, big.NewRat(0, 1), mul)
}

func TestTxCost_NilGasPrice_ReturnsZero(t *testing.T) {
	_, b, v, gm, sm, tm, cfg, _ := newRecipientFixtureOrFatal(t)
	gm.gasPrice = nil
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, gm, sm, tm, secret, cfg)
	txCost := r.(*recipient).txCost()
	assert.Equal(t, big.NewInt(0), txCost)
}

func TestSenderNoncesCleanupLoop(t *testing.T) {
	assert := assert.New(t)

	tm := &stubTimeManager{}

	r := &recipient{
		tm:   tm,
		quit: make(chan struct{}),
		senderNonces: make(map[string]*struct {
			nonceSeen       map[uint32]bool
			expirationBlock *big.Int
		}),
	}

	// add some senderNonces
	rand0 := "blastoise"
	rand1 := "charizard"
	rand2 := "raichu"
	r.senderNonces[rand0] = &struct {
		nonceSeen       map[uint32]bool
		expirationBlock *big.Int
	}{map[uint32]bool{1: true}, big.NewInt(3)}

	r.senderNonces[rand1] = &struct {
		nonceSeen       map[uint32]bool
		expirationBlock *big.Int
	}{map[uint32]bool{1: true}, big.NewInt(2)}

	r.senderNonces[rand2] = &struct {
		nonceSeen       map[uint32]bool
		expirationBlock *big.Int
	}{map[uint32]bool{1: true}, big.NewInt(1)}

	go r.senderNoncesCleanupLoop()
	time.Sleep(20 * time.Millisecond)

	lsb := big.NewInt(1)
	tm.blockNumSink <- lsb
	time.Sleep(20 * time.Millisecond)

	// rand0 should still be present
	_, ok := r.senderNonces[rand0]
	assert.True(ok)
	// rand1 should still be present
	_, ok = r.senderNonces[rand1]
	assert.True(ok)
	// rand2 should be deleted
	_, ok = r.senderNonces[rand2]
	assert.False(ok)

	lsb = big.NewInt(2)
	tm.blockNumSink <- lsb
	time.Sleep(20 * time.Millisecond)

	// rand0 should still be present
	_, ok = r.senderNonces[rand0]
	assert.True(ok)
	// rand1 should be deleted
	_, ok = r.senderNonces[rand1]
	assert.False(ok)
	// rand2 was already deleted

	// test quit
	r.Stop()
	time.Sleep(20 * time.Millisecond)
	assert.True(tm.blockNumSub.(*stubSubscription).unsubscribed)
}

func calcWinProb(faceValue, EV *big.Int) *big.Int {
	// Return 0 if faceValue happens to be 0
	if faceValue.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0)
	}
	// Return maxWinProb if faceValue = EV
	if faceValue.Cmp(EV) == 0 {
		return maxWinProb
	}

	m := new(big.Int)
	x, m := new(big.Int).DivMod(maxWinProb, faceValue, m)
	if m.Int64() != 0 {
		return new(big.Int).Mul(EV, x.Add(x, big.NewInt(1)))
	}
	// Compute winProb as the numerator of a fraction over maxWinProb
	return new(big.Int).Mul(EV, x)
}
