package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/verification"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
)

type PushClient struct {
	url        string
	streamName string
}

func NewPushClient(url, streamName string) *PushClient {
	return &PushClient{url: url, streamName: streamName}
}

func UsePushClient(client *PushClient) {
	transcodeSegFunc = client.TranscodeSegment
}

func (c *PushClient) TranscodeSegment(cxn *rtmpConnection, seg *stream.HLSSegment, name string, verifier *verification.SegmentVerifier) ([]string, error) {
	uploadURL := fmt.Sprintf("%v/live/%v/%v.%s", c.url, c.streamName, seg.SeqNo, filepath.Ext(name))
	req, err := http.NewRequest("POST", uploadURL, bytes.NewBuffer(seg.Data))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Accept", "multipart/mixed")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(mediaType, "multipart/") {
		return nil, errors.New("unexpected media type")
	}

	mr := multipart.NewReader(resp.Body, params["boundary"])
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}

		data, err := ioutil.ReadAll(p)
		if err != nil {
			return nil, err
		}

		renditionName := p.Header["Rendition-Name"][0]
		profile, err := ffmpegProfileForName(renditionName, cxn.params.Profiles)
		if err != nil {
			return nil, err
		}

		ext, err := common.ProfileFormatExtension(profile.Format)
		if err != nil {
			return nil, err
		}

		segName := fmt.Sprintf("%s/%d%s", profile.Name, seg.SeqNo, ext)
		uri, err := cxn.pl.GetOSSession().SaveData(segName, data)
		if err != nil {
			return nil, err
		}

		if err := cxn.pl.InsertHLSSegment(&profile, seg.SeqNo, uri, seg.Duration); err != nil {
			return nil, err
		}

		if cxn.recorder != nil {
			cxn.recorder.RecordSegment(int32(seg.SeqNo))
		}
	}
}

func ffmpegProfileForName(name string, profiles []ffmpeg.VideoProfile) (ffmpeg.VideoProfile, error) {
	for _, p := range profiles {
		if p.Name == name {
			return p, nil
		}
	}
	return ffmpeg.VideoProfile{}, errors.New("no ffmpeg profile for name")
}
