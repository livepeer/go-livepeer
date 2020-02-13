package crypto

import (
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestEcrecover(t *testing.T) {
	assert := assert.New(t)

	addr := ethcommon.HexToAddress("3BadDb1eeE2105893136A3F96c8a963E9C6309d6")
	msg := ethcommon.FromHex("b7da355477356fc4c47fcabcf232dc77a6db9b07b7e48b76261cc55cc8fbabb3")
	sig := ethcommon.FromHex("206443228e8f784bc3a122de0d85eb3ebff82d6a79cca26c7eeb907099a6404f6dff57bc6828f28bd6cd073c89d94cf3364204679ed8365fa45b5ee6af19a9841c")

	_, err := ecrecover(msg, sig[:64])
	assert.EqualError(err, "invalid signature length")

	sigHighS := ethcommon.FromHex("e742ff452d41413616a5bf43fe15dd88294e983d3d36206c2712f39083d638bde0a0fc89be718fbc1033e1d30d78be1c68081562ed2e97af876f286f3453231d1b")
	_, err = ecrecover(msg, sigHighS)
	assert.EqualError(err, "signature s value too high")

	sigInvalidV := make([]byte, 65)
	copy(sigInvalidV[:], sigInvalidV[:])
	sigInvalidV[64] -= 27
	_, err = ecrecover(msg, sigInvalidV)
	assert.EqualError(err, "signature v value must be 27 or 28")

	// Check that the correct address is recovered
	recovered, err := ecrecover(msg, sig)
	assert.Nil(err)
	assert.Equal(addr, recovered)

	// Check that the wrong address is recovered for a different message
	recovered, err = ecrecover(ethcommon.FromHex("foo"), sig)
	assert.Nil(err)
	assert.NotEqual(addr, recovered)
}

func TestVerifySig(t *testing.T) {
	assert := assert.New(t)

	addr := ethcommon.HexToAddress("3BadDb1eeE2105893136A3F96c8a963E9C6309d6")
	sig := ethcommon.FromHex("206443228e8f784bc3a122de0d85eb3ebff82d6a79cca26c7eeb907099a6404f6dff57bc6828f28bd6cd073c89d94cf3364204679ed8365fa45b5ee6af19a9841c")
	msg := ethcommon.FromHex("b7da355477356fc4c47fcabcf232dc77a6db9b07b7e48b76261cc55cc8fbabb3")

	// Check that verification passes
	assert.True(VerifySig(addr, msg, sig))
	// Check that verification fails for a different message
	assert.False(VerifySig(addr, ethcommon.FromHex("foo"), sig))
}
