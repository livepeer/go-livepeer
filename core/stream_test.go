package core

import (
	"testing"

	"bytes"

	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

func TestStreamID(t *testing.T) {
	vid := RandomVideoID()
	_, err := MakeStreamID(NodeID("nid"), vid, "")
	if err == nil {
		t.Errorf("Expecting error because NodeID is too short")
	}

	_, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	pid, err := peer.IDFromPublicKey(pub)
	id, err := MakeStreamID(NodeID(peer.IDHexEncode(pid)), vid, "")
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
