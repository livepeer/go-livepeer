package watchers

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var stubBondingManagerAddr = common.HexToAddress("0x511bc4556d823ae99630ae8de28b9b80df90ea2e")

func newStubBaseLog() types.Log {
	return types.Log{
		BlockNumber: uint64(30),
		TxHash:      common.HexToHash("0xd9bb5f9e888ee6f74bedcda811c2461230f247c205849d6f83cb6c3925e54586"),
		TxIndex:     uint(0),
		BlockHash:   common.HexToHash("0x6bbf9b6e836207ab25379c20e517a89090cbbaf8877746f6ed7fb6820770816b"),
		Index:       uint(0),
		Removed:     false,
	}
}

func newStubUnbondLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubBondingManagerAddr
	log.Topics = []common.Hash{
		common.HexToHash("0x2d5d98d189bee5496a08db2a5948cb7e5e786f09d17d0c3f228eb41776c24a06"),
		// delegate = 0x525419FF5707190389bfb5C87c375D710F5fCb0E
		common.HexToHash("0x000000000000000000000000525419ff5707190389bfb5c87c375d710f5fcb0e"),
		// delegator = 0xF75b78571F6563e8Acf1899F682Fb10A9248CCE8
		common.HexToHash("0x000000000000000000000000f75b78571f6563e8acf1899f682fb10a9248cce8"),
	}
	// unbondingLockId = 1
	// amount = 11111000000000000000
	// withdrawRound = 1457
	log.Data = common.Hex2Bytes("0x00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000009a3233a1a35d800000000000000000000000000000000000000000000000000000000000000005b1")

	return log
}
