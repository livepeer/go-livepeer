package common

import (
	"encoding/hex"
)

const VideoProfileIDSize = 8
const VideoProfileIDBytes = 4

type VideoProfileByteMap map[[VideoProfileIDBytes]byte]string

var VideoProfileNameLookup = map[string]string{
	"a7ac137a": "P720p60fps16x9",
	"49d54ea9": "P720p30fps16x9",
	"e4a64019": "P720p25fps16x9",
	"79332fe7": "P720p30fps4x3",
	"5ecf4b52": "P576p30fps16x9",
	"8b1843d6": "P576p25fps16x9",
	"93c717e7": "P360p30fps16x9",
	"7cd40fc7": "P360p25fps16x9",
	"b60382a0": "P360p30fps4x3",
	"c0a6517a": "P240p30fps16x9",
	"1301a7d0": "P240p25fps16x9",
	"d435c53a": "P240p30fps4x3",
	"fca40bf9": "P144p30fps16x9",
	"03f01d1f": "P144p25fps16x9",
}

func makeVideoProfileByteMap() VideoProfileByteMap {
	var ret = make(VideoProfileByteMap, 0)
	for k, v := range VideoProfileNameLookup {
		b, err := hex.DecodeString(k)
		if err != nil || len(b) != VideoProfileIDBytes {
			continue
		}
		var key [VideoProfileIDBytes]byte
		copy(key[:], b)
		ret[key] = v
	}
	return ret
}

var VideoProfileByteLookup = makeVideoProfileByteMap()
