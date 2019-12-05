package verification

import (
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"

	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
)

// Special error type indicating a retryable error
// Such errors typically mean re-trying the transcode might help
// (Non-retryable errors usually indicate unrecoverable system errors)
type Retryable struct {
	error
}

type Params struct {
	// ManifestID should go away once we do direct push of video
	ManifestID core.ManifestID

	// Bytes of the source video segment
	Source *stream.HLSSegment

	// Rendition parameters to be checked
	Profiles []ffmpeg.VideoProfile

	// Information on the orchestrator that performed the transcoding
	Orchestrator *net.OrchestratorInfo

	// Transcoded result metadata
	Results *net.TranscodeData
}

type Results struct {
	// Verifier specific score
	Score float64

	// Number of pixels decoded in this result
	Pixels []int64
}

type Verifier interface {
	Verify(params *Params) (*Results, error)
}
