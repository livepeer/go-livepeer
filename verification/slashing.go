package verification

import (
	"bytes"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/ffmpeg"
)

// VerificationParams holds all the data required for a verification check
type VerificationParams struct {
	ManifestID   core.ManifestID
	Profiles     []ffmpeg.VideoProfile
	Orchestrator *net.OrchestratorInfo
	DecodedData  []*net.TranscodedSegmentData
	ReceivedSig  []byte
	Broadcaster  ethcommon.Address
}

// SlashingManager is a specific utility that helps with slashing related activities
type SlashingManager struct {
	storage *common.DB
	dec     map[core.ManifestID]*ReceivedEncoders
}

// SlashingEvidence is the structure that holds the evidence for a slashing claim
type SlashingEvidence struct {
	ManifestID      core.ManifestID
	DoublesignEv    *DoublesignEvidence
	BroadcasterSigs map[ethcommon.Address][]byte
	Orchestrator    *net.OrchestratorInfo
}

// DoublesignEvidence is the specific evidence for a double signing claim
type DoublesignEvidence struct {
	Seg1 *net.TranscodedSegmentData
	Sig1 []byte
	Seg2 *net.TranscodedSegmentData
	Sig2 []byte
}

// ReceivedEncoders helps with tracking of received segments from encoders
type ReceivedEncoders struct {
	Details   map[string]map[ethcommon.Address]*ReceivedSegment // "rendition" -> "encoder" -> data
	Encodings map[string][]*ReceivedSegment                     // "rendition" -> list of segments sorted by claim
	Winners   map[string]*ReceivedSegment                       // "rendition" -> winning segment
}

// ReceivedSegment is a segment received from a specific encoder
type ReceivedSegment struct {
	SegData     *net.TranscodedSegmentData
	Sig         []byte
	Broadcaster ethcommon.Address
}

// checkSlashing is the main entrypoint for the slashing manager.
// It will take a set of parameters and check them against the store
// to see if a slashable offense has been committed.
// It will also update the store with the current set of checks.
func (sm *SlashingManager) checkSlashing(params *VerificationParams) (*SlashingEvidence, error) {
	// First, check for double signing
	doubleSignEvidence, err := sm.CheckDoubleSigning(params)
	if err != nil {
		return nil, err
	}
	if doubleSignEvidence != nil {
		// Slashing opportunity found
		// For now we only care about double signing
		return doubleSignEvidence, nil
	}
	// No slashable event found, so we update the store with the given params
	return nil, sm.UpdateStore(params)
}

// CheckDoubleSigning will check the given verification parameters against the store for a double signing event
func (sm *SlashingManager) CheckDoubleSigning(params *VerificationParams) (*SlashingEvidence, error) {
	if params.DecodedData == nil {
		return nil, nil
	}
	if sm.storage == nil {
		return nil, nil
	}
	mid := params.ManifestID
	if _, ok := sm.dec[mid]; !ok {
		sm.dec[mid] = &ReceivedEncoders{
			Details:   make(map[string]map[ethcommon.Address]*ReceivedSegment),
			Encodings: make(map[string][]*ReceivedSegment),
			Winners:   make(map[string]*ReceivedSegment),
		}
	}
	for i, tr := range params.DecodedData {
		rendition := params.Profiles[i].Name
		if _, ok := sm.dec[mid].Details[rendition]; !ok {
			sm.dec[mid].Details[rendition] = make(map[ethcommon.Address]*ReceivedSegment)
		}
		encoder := params.Orchestrator.Address
		if existing, ok := sm.dec[mid].Details[rendition][encoder]; ok {
			// Compare segment data to check for double signing
			if !bytes.Equal(existing.SegData.ConcatHash(), tr.ConcatHash()) {
				// Double signing detected!
				return &SlashingEvidence{
					ManifestID: mid,
					Orchestrator: params.Orchestrator,
					DoublesignEv: &DoublesignEvidence{
						Seg1: existing.SegData,
						Sig1: existing.Sig,
						Seg2: tr,
						Sig2: params.ReceivedSig,
					},
				}, nil
			}
		}
	}
	return nil, nil
}

// UpdateStore will update the internal store with the given parameters
func (sm *SlashingManager) UpdateStore(params *VerificationParams) error {
	mid := params.ManifestID
	if sm.storage != nil {
		// TODO: Implement actual persistent storage
	}
	if _, ok := sm.dec[mid]; !ok {
		sm.dec[mid] = &ReceivedEncoders{
			Details:   make(map[string]map[ethcommon.Address]*ReceivedSegment),
			Encodings: make(map[string][]*ReceivedSegment),
			Winners:   make(map[string]*ReceivedSegment),
		}
	}
	broadcaster := params.Broadcaster.Address()
	encoder := params.Orchestrator.Address
	for i, tr := range params.DecodedData {
		rendition := params.Profiles[i].Name
		if _, ok := sm.dec[mid].Details[rendition]; !ok {
			sm.dec[mid].Details[rendition] = make(map[ethcommon.Address]*ReceivedSegment)
		}
		// We only store the *first* segment we see from a given encoder for a given rendition
		if _, ok := sm.dec[mid].Details[rendition][encoder]; !ok {
			sm.dec[mid].Details[rendition][encoder] = &ReceivedSegment{
				SegData:     tr,
				Sig:         params.ReceivedSig,
				Broadcaster: broadcaster,
			}
		}
	}
	return nil
}
