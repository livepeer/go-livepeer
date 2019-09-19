package verification

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/golang/glog"

	"github.com/livepeer/go-livepeer/common"

	"github.com/livepeer/lpms/ffmpeg"
)

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

type EpicClassifier struct {
	Addr string
}

func (e *EpicClassifier) Verify(params *Params) error {
	mid, source, profiles := params.ManifestID, params.Source, params.Profiles
	orch, res := params.Orchestrator, params.Results
	glog.V(common.DEBUG).Infof("Verifying segment manifestID=%s seqNo=%d\n",
		mid, source.SeqNo)
	src := fmt.Sprintf("http://127.0.0.1:8935/stream/%s/source/%d.ts", mid, source.SeqNo)
	renditions := []epicRendition{}
	for i, v := range res.Segments {
		p := profiles[i]
		w, h, _ := ffmpeg.VideoProfileResolution(p) // XXX check err
		uri := fmt.Sprintf("http://127.0.0.1:8935/stream/%s/%s/%d.ts",
			mid, p.Name, source.SeqNo)
		r := epicRendition{
			URI:        uri,
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
		Source:         src,
		Renditions:     renditions,
		OrchestratorID: oid,
		Model:          "https://storage.googleapis.com/verification-models/verification.tar.xz",
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		glog.Error("Could not marshal JSON for verifier! ", err)
		return err
	}
	glog.V(common.DEBUG).Info("Request Body: ", string(reqData))
	startTime := time.Now()
	resp, err := http.Post(e.Addr, "application/json", bytes.NewBuffer(reqData))
	if err != nil {
		glog.Error("Could not submit request ", err)
		return err
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
		return err
	}
	glog.V(common.DEBUG).Info("Response Body: ", string(body))
	return nil
}
