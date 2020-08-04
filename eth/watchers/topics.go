package watchers

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var eventSignatures = []string{
	"Unbond(address,address,uint256,uint256,uint256)",
	"Rebond(address,address,uint256,uint256)",
	"WithdrawStake(address,uint256,uint256,uint256)",
	"NewRound(uint256,bytes32)",
	"DepositFunded(address,uint256)",
	"ReserveFunded(address,uint256)",
	"Withdrawal(address,uint256,uint256)",
	"WinningTicketTransfer(address,address,uint256)",
	"Unlock(address,uint256,uint256)",
	"UnlockCancelled(address)",
	"TranscoderActivated(address,uint256)",
	"TranscoderDeactivated(address,uint256)",
	"ServiceURIUpdate(address,string)",
}

// FilterTopics returns a list of topics to be used when filtering logs
func FilterTopics() []common.Hash {
	topics := make([]common.Hash, len(eventSignatures))
	for i, sig := range eventSignatures {
		topics[i] = crypto.Keccak256Hash([]byte(sig))
	}

	return topics
}
