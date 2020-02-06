package verification

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"sort"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
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

var ErrPixelMismatch = Retryable{errors.New("PixelMismatch")}
var ErrPixelsAbsent = errors.New("PixelsAbsent")

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

type byResScore []SegmentVerifierResults

func (a byResScore) Len() int           { return len(a) }
func (a byResScore) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byResScore) Less(i, j int) bool { return a[i].res.Score < a[j].res.Score }

type SegmentVerifier struct {
	policy  *Policy
	results []SegmentVerifierResults
	count   int
}

func NewSegmentVerifier(p *Policy) *SegmentVerifier {
	return &SegmentVerifier{policy: p}
}

func (sv *SegmentVerifier) Verify(params *Params) (*Params, error) {

	if sv.policy == nil {
		return nil, nil
	}

	// TODO sig checking; extract from broadcast.go

	if sv.policy.Verifier == nil {
		if err := verifyPixelParams(params); err != ErrPixelsAbsent {
			return nil, err
		}
		return nil, nil
	}
	// TODO Use policy sampling rate to determine whether to invoke verifier.
	//      If not, exit early. Seed sample using source data for repeatability!
	res, err := sv.policy.Verifier.Verify(params)

	// Check pixel counts
	if (err == nil || IsRetryable(err)) && res != nil && params.Results != nil {
		if len(res.Pixels) != len(params.Results.Segments) {
			err = verifyPixelParams(params)
		}
		for i := 0; err == nil && i < len(params.Results.Segments) && i < len(res.Pixels); i++ {
			reportedPixels := params.Results.Segments[i].Pixels
			verifiedPixels := res.Pixels[i]
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

	// Append retryable errors to results
	// The caller should terminate processing for non-retryable errors
	if IsRetryable(err) {
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

func IsRetryable(err error) bool {
	_, retryable := err.(Retryable)
	return retryable
}

func verifyPixelParams(params *Params) error {
	// check if the node is on-chain mode
	if params != nil &&
		params.Orchestrator != nil &&
		params.Orchestrator.TicketParams != nil {

		var err error
		for i := 0; err == nil && i < len(params.Results.Segments); i++ {
			if err := verifyPixels(
				params.Results.Segments[i].Url,
				flatten(params.Renditions),
				params.Results.Segments[i].Pixels); err != nil {

				glog.Error(err)
				return ErrPixelMismatch
			}
		}
		return nil

	}
	return ErrPixelsAbsent
}

func flatten(s [][]byte) (r []byte) {
	for _, e := range s {
		for _, f := range e {
			r = append(r, f)
		}
	}
	return
}

func verifyPixels(fname string, data []byte, reportedPixels int64) error {
	uri, err := url.ParseRequestURI(fname)
	// If the filename is a relative URI and the broadcaster is using local memory storage
	// write the data to a temp file
	if err == nil && !uri.IsAbs() && data != nil {
		tempfile, err := ioutil.TempFile("", common.RandName())
		if err != nil {
			return fmt.Errorf("error creating temp file for pixels verification: %v", err)
		}
		defer os.Remove(tempfile.Name())

		if _, err := tempfile.Write(data); err != nil {
			tempfile.Close()
			return fmt.Errorf("error writing temp file for pixels verification: %v", err)
		}

		if err = tempfile.Close(); err != nil {
			return fmt.Errorf("error closing temp file for pixels verification: %v", err)
		}

		fname = tempfile.Name()
	}

	p, err := Pixels(fname)
	if err != nil {
		return err
	}

	if p != reportedPixels {
		return ErrPixelMismatch
	}

	return nil
}

func Pixels(fname string) (int64, error) {
	in := &ffmpeg.TranscodeOptionsIn{Fname: fname}
	res, err := ffmpeg.Transcode3(in, nil)
	if err != nil {
		return 0, err
	}

	return res.Decoded.Pixels, nil
}
