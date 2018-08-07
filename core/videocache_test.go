package core

import (
	"testing"
	"time"

	"github.com/ericxtang/m3u8"
)

func TestGetMasterPlaylist(t *testing.T) {
	stubnet := &StubVideoNetwork{
		mplMap: make(map[string]*m3u8.MasterPlaylist),
	}
	c := NewBasicVideoCache(stubnet)
	pl := m3u8.NewMasterPlaylist()
	pl.Append("test1.m3u8", nil, m3u8.VariantParams{Bandwidth: 100})
	stubnet.mplMap["122011e494a06b20bf7a80f40e80d538675cc0b168c21912d33e0179617d5d4fe4e0Test"] = pl

	testpl := c.GetHLSMasterPlaylist("122011e494a06b20bf7a80f40e80d538675cc0b168c21912d33e0179617d5d4fe4e0Test")
	if testpl.String() != pl.String() {
		t.Errorf("Expecting %v, got %v", pl.String(), testpl.String())
	}
}

func TestEvictMasterPlaylist(t *testing.T) {
	stubnet := &StubVideoNetwork{
		mplMap: make(map[string]*m3u8.MasterPlaylist),
	}
	c := NewBasicVideoCache(stubnet)
	pl := m3u8.NewMasterPlaylist()
	pl.Append("test1.m3u8", nil, m3u8.VariantParams{Bandwidth: 100})
	stubnet.mplMap["122011e494a06b20bf7a80f40e80d538675cc0b168c21912d33e0179617d5d4fe4e0Test"] = pl

	testpl := c.GetHLSMasterPlaylist("122011e494a06b20bf7a80f40e80d538675cc0b168c21912d33e0179617d5d4fe4e0Test")
	if testpl.String() != pl.String() {
		t.Errorf("Expecting %v, got %v", pl.String(), testpl.String())
	}

	c.EvictHLSMasterPlaylist("122011e494a06b20bf7a80f40e80d538675cc0b168c21912d33e0179617d5d4fe4e0Test")
	testpl = c.GetHLSMasterPlaylist("122011e494a06b20bf7a80f40e80d538675cc0b168c21912d33e0179617d5d4fe4e0Test")
	if testpl != nil {
		t.Errorf("Expecting nil, got %v", testpl.String())
	}
}

func TestGetAndEvictHLSMediaPlaylist(t *testing.T) {
	stubnet := &StubVideoNetwork{
		subscribers: make(map[string]*StubSubscriber),
	}
	c := NewBasicVideoCache(stubnet)
	strmID := "122011e494a06b20bf7a80f40e80d538675cc0b168c21912d33e0179617d5d4fe4e0Test"

	GetMediaPlaylistWaitTime = time.Millisecond * 500
	pl := c.GetHLSMediaPlaylist(StreamID(strmID))
	if pl != nil {
		t.Errorf("Expecting nil as pl")
	}

	stubnet.subscribers[strmID] = &StubSubscriber{}
	pl = c.GetHLSMediaPlaylist(StreamID(strmID))
	if pl == nil {
		t.Errorf("Expecting pl, got nil")
	}
	s := pl.Segments[0]
	if s.URI != "test.ts" {
		t.Errorf("Expecting test.ts, got %v", s.URI)
	}

	//Test evict (first need to simulate that stream has stopped)
	delete(stubnet.subscribers, strmID)
	c.EvictHLSStream(StreamID(strmID))
	pl = c.GetHLSMediaPlaylist(StreamID(strmID))
	if pl != nil {
		t.Errorf("Expecting no pl, got %v", pl)
	}
}
