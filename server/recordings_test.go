package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/core"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/stretchr/testify/assert"
)

func TestRecordingHandler(t *testing.T) {
	drivers.Testing = true
	lpmon.NodeID = "testNode"
	assert := assert.New(t)
	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	whts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, _ := ioutil.ReadAll(r.Body)
		var req authWebhookReq
		err := json.Unmarshal(out, &req)
		if err != nil {
			glog.Error("Error parsing URL: ", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Write([]byte(`{"manifestID":"playback01", "recordObjectStore": "memory://recstore5",
		"recordObjectStoreUrl":"https://pub.test/",
		"previousSessions":["sess1","sess2"]}`))
	}))
	defer whts.Close()
	oldURL := AuthWebhookURL
	defer func() { AuthWebhookURL = oldURL }()
	AuthWebhookURL = mustParseUrl(t, whts.URL)

	makeReq := func(method, uri string) *http.Response {
		writer := httptest.NewRecorder()
		req := httptest.NewRequest(method, uri, nil)
		s.HandleRecordings(writer, req)
		resp := writer.Result()
		return resp
	}

	resp := makeReq("GET", "/live/notMatter/1.ts")
	resp.Body.Close()
	assert.Equal(404, resp.StatusCode)

	mos := drivers.TestMemoryStorages["recstore5"]
	msess1 := mos.NewSession("sess1")
	msess2 := mos.NewSession("sess2")
	msess3 := mos.NewSession("sess3")

	jpl := core.NewJSONPlaylist()
	profile := ffmpeg.P144p25fps16x9
	jpl.InsertHLSSegment(&profile, 1, "sess1/testNode/P144p25fps16x9/1.ts", 2100)
	bjpl, _ := json.Marshal(jpl)
	msess1.SaveData(context.TODO(), "testNode/playlist_1.json", bytes.NewReader(bjpl), nil, 0)
	jpl = core.NewJSONPlaylist()
	jpl.InsertHLSSegment(&profile, 2, "sess2/testNode/P144p25fps16x9/2.ts", 2100)
	bjpl, _ = json.Marshal(jpl)
	msess2.SaveData(context.TODO(), "testNode/playlist_2.json", bytes.NewReader(bjpl), nil, 0)
	jpl = core.NewJSONPlaylist()
	jpl.InsertHLSSegment(&profile, 1, "sess3/testNode/P144p25fps16x9/3.ts", 2100)
	bjpl, _ = json.Marshal(jpl)
	msess3.SaveData(context.TODO(), "testNode/playlist_3.json", bytes.NewReader(bjpl), nil, 0)

	resp = makeReq("GET", "/recordings/sess3/P144p25fps16x9.m3u8")
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(200, resp.StatusCode)
	assert.Equal("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-TARGETDURATION:2100\n#EXTINF:2100.000,\nhttps://pub.test/sess1/testNode/P144p25fps16x9/1.ts\n#EXT-X-DISCONTINUITY\n#EXTINF:2100.000,\nhttps://pub.test/sess2/testNode/P144p25fps16x9/2.ts\n#EXT-X-DISCONTINUITY\n#EXTINF:2100.000,\nhttps://pub.test/sess3/testNode/P144p25fps16x9/3.ts\n#EXT-X-ENDLIST\n", string(body))
	fir, err := msess3.ReadData(context.Background(), "sess3/P144p25fps16x9.m3u8")
	assert.Nil(err)
	assert.NotNil(fir)
	body, _ = ioutil.ReadAll(fir.Body)
	fir.Body.Close()
	assert.Equal("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-TARGETDURATION:2100\n#EXTINF:2100.000,\nhttps://pub.test/sess1/testNode/P144p25fps16x9/1.ts\n#EXT-X-DISCONTINUITY\n#EXTINF:2100.000,\nhttps://pub.test/sess2/testNode/P144p25fps16x9/2.ts\n#EXT-X-DISCONTINUITY\n#EXTINF:2100.000,\nhttps://pub.test/sess3/testNode/P144p25fps16x9/3.ts\n#EXT-X-ENDLIST\n", string(body))
}

func TestRecording(t *testing.T) {
	drivers.Testing = true
	lpmon.NodeID = "testNode"
	assert := assert.New(t)
	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()

	whts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, _ := ioutil.ReadAll(r.Body)
		var req authWebhookReq
		err := json.Unmarshal(out, &req)
		if err != nil {
			glog.Error("Error parsing URL: ", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Write([]byte(`{"manifestID":"rectest01", "recordObjectStore": "memory://recstore4"}`))
	}))

	defer whts.Close()
	oldURL := AuthWebhookURL
	defer func() { AuthWebhookURL = oldURL }()
	AuthWebhookURL = mustParseUrl(t, whts.URL)
	makeReq := func(method, uri string) *http.Response {
		writer := httptest.NewRecorder()
		req := httptest.NewRequest(method, uri, nil)
		s.HandleRecordings(writer, req)
		resp := writer.Result()
		return resp
	}

	resp := makeReq("GET", "/live/sess1/1.ts")
	resp.Body.Close()
	assert.Equal(404, resp.StatusCode)

	mos := drivers.TestMemoryStorages["recstore4"]
	msess := mos.NewSession("sess1")
	msess.SaveData(context.TODO(), "testNode/source/1.ts", strings.NewReader("segmentdata"), nil, 0)

	resp = makeReq("GET", "/live/sess1/testNode/source/1.ts")
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(200, resp.StatusCode)
	assert.Equal("segmentdata", string(body))

	jpl := core.NewJSONPlaylist()
	profile := ffmpeg.P144p25fps16x9
	jpl.InsertHLSSegment(&profile, 1, "testNode/P144p25fps16x9/1.ts", 2100)
	bjpl, err := json.Marshal(jpl)
	assert.Nil(err)
	msess.SaveData(context.TODO(), "testNode/playlist_1.json", bytes.NewReader(bjpl), nil, 0)
	jpl = core.NewJSONPlaylist()
	jpl.InsertHLSSegment(&profile, 2, "testNode/P144p25fps16x9/2.ts", 2100)
	bjpl, err = json.Marshal(jpl)
	assert.Nil(err)
	msess.SaveData(context.TODO(), "testNode/playlist_2.json", bytes.NewReader(bjpl), nil, 0)

	resp = makeReq("GET", "/live/sess1/index.m3u8")
	body, _ = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(200, resp.StatusCode)
	assert.Equal("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-STREAM-INF:PROGRAM-ID=0,BANDWIDTH=400000,RESOLUTION=256x144\nP144p25fps16x9.m3u8\n", string(body))
	fir, err := msess.ReadData(context.Background(), "sess1/index.m3u8")
	assert.Nil(err)
	assert.NotNil(fir)
	body, _ = ioutil.ReadAll(fir.Body)
	fir.Body.Close()
	assert.Equal("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-STREAM-INF:PROGRAM-ID=0,BANDWIDTH=400000,RESOLUTION=256x144\nP144p25fps16x9.m3u8\n", string(body))

	resp = makeReq("GET", "/live/sess1/P144p25fps16x9.m3u8")
	body, _ = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(200, resp.StatusCode)
	assert.Equal("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-TARGETDURATION:2100\n#EXTINF:2100.000,\ntestNode/P144p25fps16x9/1.ts\n#EXTINF:2100.000,\ntestNode/P144p25fps16x9/2.ts\n#EXT-X-ENDLIST\n", string(body))
	fir, err = msess.ReadData(context.Background(), "sess1/P144p25fps16x9.m3u8")
	assert.Nil(err)
	assert.NotNil(fir)
	body, _ = ioutil.ReadAll(fir.Body)
	fir.Body.Close()
	assert.Equal("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-TARGETDURATION:2100\n#EXTINF:2100.000,\ntestNode/P144p25fps16x9/1.ts\n#EXTINF:2100.000,\ntestNode/P144p25fps16x9/2.ts\n#EXT-X-ENDLIST\n", string(body))

	msess = mos.NewSession("sess2")
	jpl = core.NewJSONPlaylist()
	jpl.InsertHLSSegment(&profile, 3, "testNode/P144p25fps16x9/3.ts", 2100)
	bjpl, err = json.Marshal(jpl)
	assert.Nil(err)
	msess.SaveData(context.TODO(), "testNode/playlist_1.json", bytes.NewReader(bjpl), nil, 0)
	jpl = core.NewJSONPlaylist()
	jpl.InsertHLSSegment(&profile, 4, "testNode/P144p25fps16x9/4.ts", 2450)
	bjpl, err = json.Marshal(jpl)
	assert.Nil(err)
	msess.SaveData(context.TODO(), "testNode/playlist_2.json", bytes.NewReader(bjpl), nil, 0)
	resp = makeReq("GET", "/live/sess2/P144p25fps16x9.m3u8?finalize=false")
	body, _ = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(200, resp.StatusCode)
	assert.Equal("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-TARGETDURATION:2450\n#EXTINF:2100.000,\ntestNode/P144p25fps16x9/3.ts\n#EXTINF:2450.000,\ntestNode/P144p25fps16x9/4.ts\n#EXT-X-ENDLIST\n", string(body))
	fir, err = msess.ReadData(context.Background(), "sess2/P144p25fps16x9.m3u8")
	assert.NotNil(err)
	assert.Nil(fir)
}
