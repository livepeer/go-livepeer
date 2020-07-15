package verification

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sort"

	"github.com/golang/glog"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	lpcrypto "github.com/livepeer/go-livepeer/crypto"
	"github.com/livepeer/go-livepeer/drivers"
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

type Fatal struct {
	Retryable
}

var ErrPixelMismatch = Retryable{errors.New("PixelMismatch")}
var ErrPixelsAbsent = errors.New("PixelsAbsent")
var errPMCheckFailed = errors.New("PM Check Failed")

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

	// Rendition locations; typically when the data is in object storage
	URIs []string

	// Cached data when local object storage is used
	Renditions [][]byte

	// External object storage used by broadcaster (can be defined per-stream)
	OS drivers.OSSession
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

type Policy struct {

	// Verification function to run
	Verifier Verifier

	// Maximum number of retries until the policy chooses a winner
	Retries int

	// How often to invoke the verifier, on a per-segment basis
	SampleRate float64 // XXX for later

	// How many parallel transcodes to support
	Redundancy int // XXX for later
}

type SegmentVerifierResults struct {
	params *Params
	res    *Results
}

type sigVerifyFn func(addr ethcommon.Address, msg, sig []byte) bool

type byResScore []SegmentVerifierResults

func (a byResScore) Len() int           { return len(a) }
func (a byResScore) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byResScore) Less(i, j int) bool { return a[i].res.Score < a[j].res.Score }

type SegmentVerifier struct {
	policy    *Policy
	results   []SegmentVerifierResults
	count     int
	verifySig sigVerifyFn
}

func NewSegmentVerifier(p *Policy) *SegmentVerifier {
	return &SegmentVerifier{policy: p, verifySig: lpcrypto.VerifySig}
}

func (sv *SegmentVerifier) Verify(params *Params) (*Params, error) {
	if sv.policy == nil {
		return nil, nil
	}

	if err := sv.sigVerification(params); err != nil {
		return nil, err
	}

	var err error
	res := &Results{}

	// TODO Use policy sampling rate to determine whether to invoke verifier.
	//      If not, exit early. Seed sample using source data for repeatability!
	if sv.policy.Verifier != nil {
		res, err = sv.policy.Verifier.Verify(params)
	}

	// Check pixel counts
	if (err == nil || (err != ErrAudioMismatch && IsRetryable(err))) && res != nil && params.Results != nil {
		pxls := res.Pixels
		if len(pxls) != len(params.Results.Segments) {
			pxls, err = countPixelParams(params)
		}
		for i := 0; err == nil && i < len(params.Results.Segments) && i < len(pxls); i++ {
			reportedPixels := params.Results.Segments[i].Pixels
			verifiedPixels := pxls[i]
			if reportedPixels != verifiedPixels {
				err = ErrPixelMismatch
			}
		}
	}

	if err == nil {
		// Verification passed successfully, so use this set of params
		return params, nil
	}
	sv.count++

	// Append non-fatal retryable errors to results
	// The caller should terminate processing for non-retryable errors
	if !IsFatal(err) && IsRetryable(err) {
		r := SegmentVerifierResults{params: params, res: res}
		sv.results = append(sv.results, r)
	}

	// Check for max retries
	// If max hit, return best params so far
	if sv.count > sv.policy.Retries {
		if len(sv.results) <= 0 {
			return nil, err
		}
		sort.Sort(byResScore(sv.results))
		return sv.results[len(sv.results)-1].params, err
	}

	return nil, err
}

func IsFatal(err error) bool {
	_, fatal := err.(Fatal)
	return fatal
}

func IsRetryable(err error) bool {
	_, retryable := err.(Retryable)
	return retryable || IsFatal(err)
}

func (sv *SegmentVerifier) sigVerification(params *Params) error {
	if params.Orchestrator == nil || params.Orchestrator.TicketParams == nil {
		return nil
	}

	if len(params.Results.Segments) != len(params.Renditions) {
		return errPMCheckFailed
	}

	segHashes := make([][]byte, len(params.Renditions))
	for i := range params.Renditions {
		segHashes[i] = crypto.Keccak256(params.Renditions[i])
	}

	// Verify the signature against the orchestrator provided address if it exists
	// Otherwise verify the signature against the ticket recipient address
	addr := ethcommon.BytesToAddress(params.Orchestrator.Address)
	if (addr == ethcommon.Address{}) {
		addr = ethcommon.BytesToAddress(params.Orchestrator.TicketParams.Recipient)
	}

	if !sv.verifySig(
		addr,
		crypto.Keccak256(segHashes...),
		params.Results.Sig) {
		glog.Error("Sig check failed")
		return errPMCheckFailed
	}
	return nil
}

func countPixelParams(params *Params) ([]int64, error) {

	if len(params.Results.Segments) != len(params.Renditions) {
		return nil, ErrPixelsAbsent
	}

	pxls := make([]int64, len(params.Results.Segments))

	for i := 0; i < len(params.Results.Segments); i++ {
		count, err := countPixels(params.Renditions[i])
		if err != nil {
			return nil, err
		}
		pxls[i] = count
	}
	return pxls, nil
}

func countPixels(data []byte) (int64, error) {
	// write the data to a temp file
	tempfile, err := ioutil.TempFile("", common.RandName())
	if err != nil {
		return 0, fmt.Errorf("error creating temp file for pixels verification: %w", err)
	}
	defer os.Remove(tempfile.Name())

	if _, err := tempfile.Write(data); err != nil {
		tempfile.Close()
		return 0, fmt.Errorf("error writing temp file for pixels verification: %w", err)
	}

	if err = tempfile.Close(); err != nil {
		return 0, fmt.Errorf("error closing temp file for pixels verification: %w", err)
	}

	p, err := pixels(tempfile.Name())
	if err != nil {
		return 0, err
	}

	return p, nil
}

func pixels(fname string) (int64, error) {
	in := &ffmpeg.TranscodeOptionsIn{Fname: fname}
	res, err := ffmpeg.Transcode3(in, nil)
	if err != nil {
		return 0, err
	}

	return res.Decoded.Pixels, nil
}
