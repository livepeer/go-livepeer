package pm

import ethcommon "github.com/ethereum/go-ethereum/common"

func hexToHash(in string) ethcommon.Hash {
	return ethcommon.BytesToHash(ethcommon.FromHex(in))
}
