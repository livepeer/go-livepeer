package core

import (
	"testing"

	ffmpeg "github.com/livepeer/lpms/ffmpeg"

	"bytes"

	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

func TestStreamID(t *testing.T) {
	vid := RandomVideoID()
	_, err := MakeStreamID(NodeID("nid"), vid, ffmpeg.P144p30fps16x9.Name)
	if err == nil {
		t.Errorf("Expecting error because NodeID is too short")
	}

	_, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	pid, err := peer.IDFromPublicKey(pub)
	id, err := MakeStreamID(NodeID(peer.IDHexEncode(pid)), vid, ffmpeg.P144p30fps16x9.Name)
	if err != nil {
		t.Errorf("Error creating Node ID: %v", err)
	}

	nid := id.GetNodeID()
	if nid != NodeID(peer.IDHexEncode(pid)) {
		t.Errorf("Expecting: %v, got %v", NodeID(peer.IDHexEncode(pid)), nid)
	}

	if bytes.Compare(vid, id.GetVideoID()) != 0 {
		t.Errorf("Expecting: %v, got %v", vid, id.GetVideoID())
	}
}
