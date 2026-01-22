package server

import (
	"context"
	"net/url"
	"testing"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/stretchr/testify/require"
)

func TestLV2VCapabilities(t *testing.T) {
	caps := lv2vCapabilities("modelA")
	require.NotNil(t, caps)
	require.Equal(t, uint32(1), caps.Capacities[uint32(core.Capability_LiveVideoToVideo)])

	models := caps.Constraints.PerCapability[uint32(core.Capability_LiveVideoToVideo)].Models
	_, ok := models["modelA"]
	require.True(t, ok)
}

func TestRefreshOrchInfoForLV2V(t *testing.T) {
	defer func(prev func(context.Context, common.Broadcaster, *url.URL, GetOrchestratorInfoParams) (*net.OrchestratorInfo, error)) {
		remoteGetOrchInfo = prev
	}(remoteGetOrchInfo)

	expected := &net.OrchestratorInfo{
		PriceInfo: &net.PriceInfo{PricePerUnit: 5, PixelsPerUnit: 10},
		TicketParams: &net.TicketParams{
			Recipient: []byte{0x01},
		},
	}

	remoteGetOrchInfo = func(ctx context.Context, b common.Broadcaster, uri *url.URL, params GetOrchestratorInfoParams) (*net.OrchestratorInfo, error) {
		return expected, nil
	}

	node, _ := core.NewLivepeerNode(nil, "", nil)
	info := refreshOrchInfoForLV2V(context.Background(), node, "https://example.com", "modelA")
	require.Equal(t, expected, info)
}
