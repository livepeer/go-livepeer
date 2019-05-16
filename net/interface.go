package net

import (
	"net/url"

	"github.com/ericxtang/m3u8"
)

type OrchestratorPool interface {
	GetURLs() []*url.URL
	GetOrchestrators(int) ([]*OrchestratorInfo, error)
	Size() int
}

type RemoteTranscoderInfo struct {
	Address  string
	Capacity int
}

type NodeStatus struct {
	Manifests                   map[string]*m3u8.MasterPlaylist
	OrchestratorPool            []string
	Version                     string
	RegisteredTranscodersNumber int
	RegisteredTranscoders       []RemoteTranscoderInfo
	LocalTranscoding            bool // Indicates orchestrator that is also transcoder
	// xxx add transcoder's version here
}
