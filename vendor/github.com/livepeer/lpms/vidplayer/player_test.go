package vidplayer

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"net/url"

	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/m3u8"
	joy4rtmp "github.com/nareix/joy4/format/rtmp"
)

func TestRTMP(t *testing.T) {
	server := &joy4rtmp.Server{Addr: ":1936"}
	player := &VidPlayer{RtmpServer: server}

	//Handler should get called
	handler1Called := false
	player.rtmpPlayHandler = func(url *url.URL) (stream.RTMPVideoStream, error) {
		handler1Called = true
		return nil, fmt.Errorf("error")
	}
	handler := player.rtmpServerHandlePlay()
	handler(&joy4rtmp.Conn{})
	if !handler1Called {
		t.Errorf("Handler not called")
	}

	//Re-assign handler, it should still get called
	handler2Called := false
	player.rtmpPlayHandler = func(url *url.URL) (stream.RTMPVideoStream, error) {
		handler2Called = true
		return nil, fmt.Errorf("error")
	}
	handler(&joy4rtmp.Conn{})
	if !handler1Called {
		t.Errorf("Handler1 not called")
	}

	if !handler2Called {
		t.Errorf("Handler2 not called")
	}

	//TODO: Should add a test for writing to the stream.
}

func stubGetMasterPL(url *url.URL) (*m3u8.MasterPlaylist, error) {
	// glog.Infof("%v", url.Path)
	if url.Path == "/master.m3u8" {
		mpl := m3u8.NewMasterPlaylist()
		mpl.Append("test.m3u8", nil, m3u8.VariantParams{Bandwidth: 10})
		return mpl, nil
	}
	return nil, nil
}

func stubGetMediaPL(url *url.URL) (*m3u8.MediaPlaylist, error) {
	if url.Path == "/media.m3u8" {
		pl, _ := m3u8.NewMediaPlaylist(10, 10)
		pl.Append("seg1.ts", 10, "seg1.ts")
		return pl, nil
	}
	return nil, nil
}

func stubGetSeg(url *url.URL) ([]byte, error) {
	return []byte("testseg"), nil
}

func stubGetSegNotFound(url *url.URL) ([]byte, error) {
	return nil, ErrNotFound
}

func TestHLS(t *testing.T) {
	rec := httptest.NewRecorder()
	handleLive(rec, httptest.NewRequest("GET", "/badpath", strings.NewReader("")), stubGetMasterPL, stubGetMediaPL, stubGetSeg)
	if rec.Result().StatusCode != 500 {
		t.Errorf("Expecting 500 because of bad path, but got: %v", rec.Result().StatusCode)
	}

	//Test getting master playlist
	rec = httptest.NewRecorder()
	handleLive(rec, httptest.NewRequest("GET", "/master.m3u8", strings.NewReader("")), stubGetMasterPL, stubGetMediaPL, stubGetSeg)
	if rec.Result().StatusCode != 200 {
		t.Errorf("Expecting 200, but got %v", rec.Result().StatusCode)
	}
	mpl := m3u8.NewMasterPlaylist()
	mpl.DecodeFrom(rec.Result().Body, true)
	if mpl.Variants[0].URI != "test.m3u8" {
		t.Errorf("Expecting test.m3u8, got %v", mpl.Variants[0].URI)
	}

	//Test getting media playlist
	rec = httptest.NewRecorder()
	handleLive(rec, httptest.NewRequest("GET", "/media.m3u8", strings.NewReader("")), stubGetMasterPL, stubGetMediaPL, stubGetSeg)
	if rec.Result().StatusCode != 200 {
		t.Errorf("Expecting 200, but got %v", rec.Result().StatusCode)
	}
	pl, _, err := m3u8.DecodeFrom(rec.Result().Body, true)
	if err != nil {
		t.Errorf("Error decoding from result: %v", err)
	}
	me, ok := pl.(*m3u8.MediaPlaylist)
	if !ok {
		t.Errorf("Expecting a media playlist, got %v", me)
	}
	if me.Segments[0].URI != "seg1.ts" || me.Segments[0].Duration != 10 {
		t.Errorf("Expecting seg1.ts with duration 10, got: %v", me.Segments[0])
	}

	//Test getting segment
	rec = httptest.NewRecorder()
	handleLive(rec, httptest.NewRequest("GET", "/seg.ts", strings.NewReader("")), stubGetMasterPL, stubGetMediaPL, stubGetSeg)
	if rec.Result().StatusCode != 200 {
		t.Errorf("Expecting 200, but got %v", rec.Result().StatusCode)
	}
	res, err := ioutil.ReadAll(rec.Result().Body)
	if err != nil {
		t.Errorf("Error reading result: %v", err)
	}
	if string(res) != "testseg" {
		t.Errorf("Expecting testseg, got %v", string(res))
	}

	//Test not found segment
	rec = httptest.NewRecorder()
	handleLive(rec, httptest.NewRequest("GET", "/seg.ts", strings.NewReader("")), stubGetMasterPL, stubGetMediaPL, stubGetSegNotFound)
	if rec.Result().StatusCode != 404 {
		t.Errorf("Expecting 404, but got %v", rec.Result().StatusCode)
	}
}

type TestRWriter struct {
	bytes  []byte
	header map[string][]string
}

func (t *TestRWriter) Header() http.Header { return t.header }
func (t *TestRWriter) Write(b []byte) (int, error) {
	t.bytes = b
	return 0, nil
}
func (*TestRWriter) WriteHeader(int) {}
