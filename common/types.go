package common

import (
	"context"
	"encoding/json"
	"math/big"
	"net/url"
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/m3u8"
)

type RemoteTranscoderInfo struct {
	Address  string
	Capacity int
}

type StreamInfo struct {
	SourceBytes     uint64
	TranscodedBytes uint64
}

type NodeStatus struct {
	Manifests map[string]*m3u8.MasterPlaylist
	// maps external manifest (provided in HTTP push URL to the internal one
	// (returned from webhook))
	InternalManifests           map[string]string
	StreamInfo                  map[string]StreamInfo
	OrchestratorPool            []string
	OrchestratorPoolInfos       []OrchestratorLocalInfo
	Version                     string
	GolangRuntimeVersion        string
	GOArch                      string
	GOOS                        string
	RegisteredTranscodersNumber int
	RegisteredTranscoders       []RemoteTranscoderInfo
	LocalTranscoding            bool // Indicates orchestrator that is also transcoder
	BroadcasterPrices           map[string]*big.Rat
	// xxx add transcoder's version here
}

type Broadcaster interface {
	Address() ethcommon.Address
	Sign([]byte) ([]byte, error)
}

type CapabilityComparator interface {
	CompatibleWith(*net.Capabilities) bool
	LegacyOnly() bool
}

const (
	Score_Untrusted = 0.0
	Score_Trusted   = 1.0
)

type OrchestratorLocalInfo struct {
	URL   *url.URL `json:"Url"`
	Score float32
}

// combines B's local metadata about O with info received from this O
type OrchestratorDescriptor struct {
	LocalInfo  *OrchestratorLocalInfo
	RemoteInfo *net.OrchestratorInfo
}

type OrchestratorDescriptors []OrchestratorDescriptor

func (ds OrchestratorDescriptors) GetRemoteInfos() []*net.OrchestratorInfo {
	var ois []*net.OrchestratorInfo
	for _, d := range ds {
		ois = append(ois, d.RemoteInfo)
	}
	return ois
}

func FromRemoteInfos(infos []*net.OrchestratorInfo) OrchestratorDescriptors {
	var ods OrchestratorDescriptors
	for _, oi := range infos {
		ods = append(ods, OrchestratorDescriptor{nil, oi})
	}
	return ods
}

func (u *OrchestratorLocalInfo) MarshalJSON() ([]byte, error) {
	type Alias OrchestratorLocalInfo
	return json.Marshal(&struct {
		URL string `json:"Url"`
		*Alias
	}{
		URL:   u.URL.String(),
		Alias: (*Alias)(u),
	})
}

type ScorePred = func(float32) bool
type OrchestratorPool interface {
	GetInfos() []OrchestratorLocalInfo
	GetOrchestrators(context.Context, int, Suspender, CapabilityComparator, ScorePred) (OrchestratorDescriptors, error)
	Size() int
	SizeWith(ScorePred) int
}

type SelectionAlgorithm interface {
	Select(ctx context.Context, addrs []ethcommon.Address, stakes map[ethcommon.Address]int64, maxPrice *big.Rat, prices map[ethcommon.Address]*big.Rat, perfScores map[ethcommon.Address]float64) ethcommon.Address
}

type PerfScore struct {
	Mu     sync.Mutex
	Scores map[ethcommon.Address]float64
}

func ScoreAtLeast(minScore float32) ScorePred {
	return func(score float32) bool {
		return score >= minScore
	}
}

func ScoreEqualTo(neededScore float32) ScorePred {
	return func(score float32) bool {
		return score == neededScore
	}
}

type Suspender interface {
	Suspended(orch string) int
}

type OrchestratorStore interface {
	OrchCount(filter *DBOrchFilter) (int, error)
	SelectOrchs(filter *DBOrchFilter) ([]*DBOrch, error)
	UpdateOrch(orch *DBOrch) error
}

type RoundsManager interface {
	LastInitializedRound() *big.Int
}
