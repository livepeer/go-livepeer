package verification

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"

	"github.com/livepeer/go-livepeer/common"

	"github.com/livepeer/lpms/ffmpeg"
)

// VerifierPath is the local path to the verifier shared volume.
// Remove as soon as [1] is implemented.
// [1] https://github.com/livepeer/verification-classifier/issues/64
var VerifierPath string

var ErrMissingSource = errors.New("MissingSource")
var ErrVerifierStatus = errors.New("VerifierStatus")
var ErrVideoUnavailable = errors.New("VideoUnavailable")
var ErrAudioMismatch = Fatal{Retryable{errors.New("AudioMismatch")}}
var ErrTampered = Retryable{errors.New("Tampered")}

type epicResolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}
type epicRendition struct {
	URI        string         `json:"uri"`
	Resolution epicResolution `json:"resolution"`
	Framerate  uint           `json:"frame_rate"`
	Pixels     int64          `json:"pixels"`
}
type epicRequest struct {
	Source         string          `json:"source"`
	Renditions     []epicRendition `json:"renditions"`
	OrchestratorID string          `json:"orchestratorID"`
	Model          string          `json:"model"`
}

type epicResultFields struct {
	VideoAvailable bool    `json:"video_available"`
	AudioAvailable bool    `json:"audio_available"`
	AudioDistance  float64 `json:"audio_dist"`
	Pixels         int64   `json:"pixels"`
	Tamper         float64 `json:"tamper"`
}

type epicResults struct {
	Source  string             `json:"source"`
	Results []epicResultFields `json:"results"`
}

type EpicClassifier struct {
	Addr string
}

func epicResultsToVerificationResults(er *epicResults) (*Results, error) {
	// find average of scores and build list of pixels
	var (
		score  float64
		pixels []int64
	)
	var err error
	// If an error is gathered, continue to gather overall pixel counts
	// In case this is a false positive. Only return the first error.
	for _, v := range er.Results {
		// The order of error checking is somewhat arbitrary for now
		// But generally it should check for fatal errors first, then retryable
		if v.AudioAvailable && v.AudioDistance != 0.0 && err == nil {
			err = ErrAudioMismatch
		}
		if v.VideoAvailable && v.Tamper <= 0 && err == nil {
			err = ErrTampered
		}
		if !v.VideoAvailable && err == nil {
			err = ErrVideoUnavailable
		}
		score += v.Tamper / float64(len(er.Results))
		pixels = append(pixels, v.Pixels)
	}
	return &Results{Score: score, Pixels: pixels}, err
}

func (e *EpicClassifier) Verify(params *Params) (*Results, error) {
	mid, source, profiles := params.ManifestID, params.Source, params.Profiles
	orch, res := params.Orchestrator, params.Results
	glog.V(common.DEBUG).Infof("Verifying segment manifestID=%s seqNo=%d\n",
		mid, source.SeqNo)

	// Write segments to Docker shared volume
	dir, err := ioutil.TempDir(VerifierPath, "")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	sourcePath, renditionPaths, err := writeSegments(params, dir)

	if err != nil {
		glog.Error("Bad! Error! Writing segments! ", err)
		return nil, err
	}

	// Build the request object
	renditions := []epicRendition{}
	for i, v := range res.Segments {
		p := profiles[i]
		w, h, err := ffmpeg.VideoProfileResolution(p)
		if err != nil {
			return nil, err
		}
		r := epicRendition{
			URI:        renditionPaths[i],
			Resolution: epicResolution{Width: w, Height: h},
			Framerate:  p.Framerate,
			Pixels:     v.Pixels,
		}
		renditions = append(renditions, r)
	}

	oid := orch.Transcoder
	if orch.TicketParams != nil {
		oid = hex.EncodeToString(orch.TicketParams.Recipient)
	}
	req := epicRequest{
		Source:         sourcePath,
		Renditions:     renditions,
		OrchestratorID: oid,
		Model:          "https://storage.googleapis.com/verification-models/verification.tar.xz",
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		glog.Error("Could not marshal JSON for verifier! ", err)
		return nil, err
	}
	glog.V(common.DEBUG).Info("Request Body: ", string(reqData))

	// Submit request and process results
	startTime := time.Now()
	resp, err := http.Post(e.Addr, "application/json", bytes.NewBuffer(reqData))
	if err != nil {
		glog.Error("Could not submit request ", err)
		return nil, err
	}
	defer resp.Body.Close()
	var deferErr error // short variable re-declaration of `err` bites us with defer
	body, err := ioutil.ReadAll(resp.Body)
	endTime := time.Now()
	// `defer` param evaluation semantics force us into an anonymous function
	defer func() {
		glog.Infof("Verification complete manifestID=%s seqNo=%d err=%v dur=%v",
			mid, source.SeqNo, deferErr, endTime.Sub(startTime))
	}()
	if deferErr = err; err != nil {
		return nil, err
	}
	glog.V(common.DEBUG).Info("Response Body: ", string(body))
	if resp.StatusCode >= 400 {
		deferErr = err
		return nil, ErrVerifierStatus
	}
	var er epicResults
	err = json.Unmarshal(body, &er)
	if deferErr = err; err != nil {
		return nil, err
	}
	vr, err := epicResultsToVerificationResults(&er)
	deferErr = err
	return vr, err
}

func writeSegments(params *Params, dir string) (string, []string, error) {
	baseDir := filepath.Base(dir)

	if params.Source == nil {
		return "", nil, ErrMissingSource
	}

	// Write out source
	var srcPath string
	if params.OS != nil && params.OS.IsExternal() && params.OS.IsOwn(params.Source.Name) {
		// We're using a non-local store, so use the URL of that
		srcPath = params.Source.Name
	} else {
		// Using a local store, so write the source to disk.
		// Remove this part after implementing [1]
		// [1] https://github.com/livepeer/verification-classifier/issues/64
		sharedPath := filepath.Join(dir, "source")
		err := ioutil.WriteFile(sharedPath, params.Source.Data, 0644)
		srcPath = "/stream/" + baseDir + "/source"
		if err != nil {
			return "", nil, err
		}
	}

	// Write out renditions
	renditionPaths := make([]string, len(params.URIs))
	for i, fname := range params.URIs {
		// If the broadcaster is using its own external storage, use that
		if params.OS != nil && params.OS.IsExternal() && params.OS.IsOwn(fname) {
			renditionPaths[i] = fname
			continue
		}

		// Write renditions to local disk.
		// Remove this part after implementing [1]
		// [1] https://github.com/livepeer/verification-classifier/issues/64
		profileName := params.Profiles[i].Name
		sharedPath := filepath.Join(dir, profileName)
		data := params.Renditions[i]
		if err := ioutil.WriteFile(sharedPath, data, 0644); err != nil {
			return "", nil, err
		}
		renditionPaths[i] = "/stream/" + baseDir + "/" + profileName
	}
	return srcPath, renditionPaths, nil
}
