package eth

import (
	"fmt"
	"math/big"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/eth/contracts"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromPerc_DefaultDenominator(t *testing.T) {
	assert.Equal(t, big.NewInt(1000000), FromPerc(100.0))

	assert.Equal(t, big.NewInt(500000), FromPerc(50.0))

	assert.Equal(t, big.NewInt(0), FromPerc(0.0))
}

func TestFromPercOfUint256_Given100Percent_ResultWithinEpsilon(t *testing.T) {
	actual := FromPercOfUint256(100.0)

	diff := new(big.Int).Sub(maxUint256, actual)
	assert.True(t, diff.Int64() < 100)
}
func TestFromPercOfUint256_Given50Percent_ResultWithinEpsilon(t *testing.T) {
	half := new(big.Int).Div(maxUint256, big.NewInt(2))

	actual := FromPercOfUint256(50.0)

	diff := new(big.Int).Sub(half, actual)
	assert.True(t, diff.Int64() < 100)
}

func TestFromPercOfUint256_Given0Percent_ReturnsZero(t *testing.T) {
	assert.Equal(t, int64(0), FromPercOfUint256(0.0).Int64())
}

func TestToBaseAmount(t *testing.T) {
	exp := "100000000000000000" // 0.1 eth
	wei, err := ToBaseAmount("0.1")
	assert.Nil(t, err)
	assert.Equal(t, wei.String(), exp)

	exp = "20000000000000000" // 0.02 eth
	wei, err = ToBaseAmount("0.02")
	assert.Nil(t, err)
	assert.Equal(t, wei.String(), exp)

	exp = "1250000000000000000" // 1.25 eth
	wei, err = ToBaseAmount("1.25")
	assert.Nil(t, err)
	assert.Equal(t, wei.String(), exp)

	exp = "3333333333333333" // 0.003333333333333333 eth
	wei, err = ToBaseAmount("0.003333333333333333")
	assert.Nil(t, err)
	assert.Equal(t, wei.String(), exp)

	// return an error if value has more than 18 decimals
	wei, err = ToBaseAmount("0.111111111111111111111111111111111")
	assert.Nil(t, wei)
	assert.EqualError(t, err, "submitted value has more than 18 decimals")
}

func TestFromBaseAmount(t *testing.T) {
	wei, _ := new(big.Int).SetString("100000000000000000", 10)
	ethVal := FromBaseAmount(wei) // 0.1 eth
	assert.Equal(t, ethVal, "0.1")

	wei, _ = new(big.Int).SetString("20000000000000000", 10)
	ethVal = FromBaseAmount(wei) // 0.02 eth
	assert.Equal(t, ethVal, "0.02")

	wei, _ = new(big.Int).SetString("1250000000000000000", 10)
	ethVal = FromBaseAmount(wei) // 1.25 eth
	assert.Equal(t, ethVal, "1.25")

	wei, _ = new(big.Int).SetString("3333333333333333", 10) // 0.003333333333333333 eth
	ethVal = FromBaseAmount(wei)
	assert.Equal(t, ethVal, "0.003333333333333333")

	// test that no decimals return no trailing "."
	wei, _ = new(big.Int).SetString("1000000000000000000", 10)
	ethVal = FromBaseAmount(wei) // 1 eth
	assert.Equal(t, ethVal, "1")
}

func TestDecodeTxParams(t *testing.T) {
	assert := assert.New(t)
	// TicketBroker.fundDepositAndReserve(_depositAmount: 10000000000000000, _reserveAmount: 10000000000000000)
	data := ethcommon.Hex2Bytes("511f4073000000000000000000000000000000000000000000000000002386f26fc10000000000000000000000000000000000000000000000000000002386f26fc10000")
	txParams := make(map[string]interface{})
	depositAmount := "10000000000000000"
	reserveAmount := "10000000000000000"
	ticketBroker, err := parseABI(contracts.TicketBrokerABI)
	require.NoError(t, err)

	err = decodeTxParams(&ticketBroker, txParams, data)
	assert.Nil(err)
	assert.Equal(txParams["_depositAmount"], depositAmount)
	assert.Equal(txParams["_reserveAmount"], reserveAmount)

	// Redeem winning ticket (tuple test)
	data = ethcommon.Hex2Bytes("ec8b3cb6000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000001a046641fdc00b0da1c09807862f3a56fb300b94592be66ca46e1248a895eeaa2be000000000000000000000000a5e37e0ba14655e92deff29f32adbc7d09b8a2cf0000000000000000000000003ee860a4aba830af84ebbce2b381fc11db8493e2000000000000000000000000000000000000000000000000002386f26fc1000000068db8bac710cb295e9e1b089a027525460aa64c2f837b4a233a4c6f8cf000000000000000000000000000000000000000000000000000000000000000000e5309662eee5608da45aaefe60433adec052acab8683dd12d78aad8a0192b3bec00000000000000000000000000000000000000000000000000000000000000e000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000653f3b1d2925fd1325a8a6ba8923fbe6de31dd93d9e85540eaf5a306f21177c3b8c000000000000000000000000000000000000000000000000000000000000004190cb8a9280386cc77e9f3473bb87530cf9a1dbe5625013a9ed16675ad7607c800bf4e32ee4cc6b9c785e1c786c9965831a8216c408b251f98d6c18bf1bf4d1621c00000000000000000000000000000000000000000000000000000000000000")
	txParams = make(map[string]interface{})
	err = decodeTxParams(&ticketBroker, txParams, data)

	assert.Nil(err)
	ticket := "{ Recipient: 0xa5E37e0BA14655e92DEff29F32adbc7D09b8a2cF  Sender: 0x3eE860A4abA830AF84EbBCE2b381fc11DB8493e2  FaceValue: 10000000000000000  WinProb: 11579208923731619542357098500868790785326998466564056403945759000000000000  SenderNonce: 14  RecipientRandHash: 0x5309662eee5608da45aaefe60433adec052acab8683dd12d78aad8a0192b3bec  AuxData: 0x0000000000000000000000000000000000000000000000000000000000000653f3b1d2925fd1325a8a6ba8923fbe6de31dd93d9e85540eaf5a306f21177c3b8c }"
	sig := "0x90cb8a9280386cc77e9f3473bb87530cf9a1dbe5625013a9ed16675ad7607c800bf4e32ee4cc6b9c785e1c786c9965831a8216c408b251f98d6c18bf1bf4d1621c"
	recipientRand := "31838803992704255588716800748284438047369588248489535212488229909862636233406"
	assert.Equal(txParams["_ticket"], ticket)
	assert.Equal(txParams["_sig"], sig)
	assert.Equal(txParams["_recipientRand"], recipientRand)

	// Batch redeem winning ticket (slice of tuples test)
	ticketA := contracts.Struct1{
		Recipient:         pm.RandAddress(),
		Sender:            pm.RandAddress(),
		FaceValue:         big.NewInt(5000000000),
		WinProb:           big.NewInt(50000000),
		SenderNonce:       big.NewInt(99999),
		RecipientRandHash: pm.RandHash(),
		AuxData:           pm.RandBytes(64),
	}

	ticketB := contracts.Struct1{
		Recipient:         pm.RandAddress(),
		Sender:            pm.RandAddress(),
		FaceValue:         big.NewInt(5000000000),
		WinProb:           big.NewInt(50000000),
		SenderNonce:       big.NewInt(99999),
		RecipientRandHash: pm.RandHash(),
		AuxData:           pm.RandBytes(64),
	}

	sigs := [][]byte{pm.RandBytes(64), pm.RandBytes(64)}
	recipientRands := []*big.Int{big.NewInt(111111), big.NewInt(222222)}

	data, _ = ticketBroker.Pack("batchRedeemWinningTickets", []contracts.Struct1{ticketA, ticketB}, sigs, recipientRands)
	err = decodeTxParams(&ticketBroker, txParams, data)
	assert.Nil(err)
	ticketAS := fmt.Sprintf("{ Recipient: %v  Sender: %v  FaceValue: %v  WinProb: %v  SenderNonce: %v  RecipientRandHash: 0x%x  AuxData: 0x%v }", ticketA.Recipient.Hex(), ticketA.Sender.Hex(), ticketA.FaceValue.String(), ticketA.WinProb.String(), ticketA.SenderNonce.String(), ticketA.RecipientRandHash, ethcommon.Bytes2Hex(ticketA.AuxData))
	ticketBS := fmt.Sprintf("{ Recipient: %v  Sender: %v  FaceValue: %v  WinProb: %v  SenderNonce: %v  RecipientRandHash: 0x%x  AuxData: 0x%v }", ticketB.Recipient.Hex(), ticketB.Sender.Hex(), ticketB.FaceValue.String(), ticketB.WinProb.String(), ticketB.SenderNonce.String(), ticketB.RecipientRandHash, ethcommon.Bytes2Hex(ticketB.AuxData))

	assert.Equal(txParams["_tickets"], fmt.Sprintf("[ %v  %v ]", ticketAS, ticketBS))
	assert.Equal(txParams["_sigs"], fmt.Sprintf("[ 0x%v  0x%v ]", ethcommon.Bytes2Hex(sigs[0]), ethcommon.Bytes2Hex(sigs[1])))
	assert.Equal(txParams["_recipientRands"], fmt.Sprintf("[ %v  %v ]", recipientRands[0].String(), recipientRands[1].String()))
}
