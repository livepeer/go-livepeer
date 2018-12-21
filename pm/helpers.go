package pm

import ethcommon "github.com/ethereum/go-ethereum/common"

func hexToHash(in string) ethcommon.Hash {
	return ethcommon.BytesToHash(ethcommon.FromHex(in))
}

func hashToHex(hash ethcommon.Hash) string {
	return ethcommon.ToHex(hash.Bytes())
}
