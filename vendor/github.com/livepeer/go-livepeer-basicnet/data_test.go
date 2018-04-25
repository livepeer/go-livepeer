package basicnet

import (
	"bytes"
	"encoding/gob"
	"testing"

	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

func compareTs(a *TranscodeSubMsg, b *TranscodeSubMsg) bool {
	// not great because we don't know precisely which cases failed.
	// return error instead?
	if peer.IDB58Encode(a.NodeID) != peer.IDB58Encode(b.NodeID) {
		return false
	}
	if a.StrmID != b.StrmID {
		return false
	}
	if 0 != bytes.Compare(a.Sig, b.Sig) {
		return false
	}
	if len(a.MultiAddrs) != len(b.MultiAddrs) {
		return false
	}
	for i, v := range a.MultiAddrs {
		if !b.MultiAddrs[i].Equal(v) {
			return false
		}
	}
	return true
}

func TestTranscodeSubGob(t *testing.T) {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	dec := gob.NewDecoder(&b)
	var tsd TranscodeSubMsg

	// empty transcodesub
	ts := TranscodeSubMsg{}
	err := enc.Encode(ts)
	if err != nil {
		t.Errorf("Unable to encode empty transcodesub")
		return
	}
	err = dec.Decode(&tsd)
	if err != nil || !compareTs(&ts, &tsd) {
		t.Error("Did not decode empty transcodesub correctly", err)
	}

	// populated transcodesub
	b.Reset()
	tsd = TranscodeSubMsg{}
	m1, _ := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234")
	_, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	nodeID, _ := peer.IDFromPublicKey(pub)
	ts = TranscodeSubMsg{
		NodeID:     nodeID,
		StrmID:     "test",
		Sig:        []byte(""),
		MultiAddrs: []ma.Multiaddr{m1},
	}
	err = enc.Encode(ts)
	if err != nil {
		t.Errorf("Unable to encode populated transcodesub")
	}
	err = dec.Decode(&tsd)
	if err != nil || !compareTs(&ts, &tsd) {
		t.Errorf("Did not decode populated transcodesub correctly")
	}

	// with several multiaddrs
	b.Reset()
	tsd = TranscodeSubMsg{}
	m3, _ := ma.NewMultiaddr("/unix/stdio")
	m2, _ := ma.NewMultiaddr("/ip6/::")
	ts.MultiAddrs = []ma.Multiaddr{m1, m2, m3}
	err = enc.Encode(ts)
	if err != nil {
		t.Errorf("Unable to encode several multiaddrs")
	}
	err = dec.Decode(&tsd)
	if err != nil || !compareTs(&ts, &tsd) {
		t.Errorf("Did not decode several multiaddrs correctly")
	}

	// fail to decode an invalid multiaddr
	b.Reset()
	tsd = TranscodeSubMsg{}
	err = enc.Encode(ts)
	if err != nil {
		t.Errorf("Unable to encode")
	}
	rawbuf := b.Bytes()
	idx := bytes.Index(rawbuf, m1.Bytes()) // find m1 within encoded gob
	if idx == -1 {
		t.Errorf("Unable to find m1 in encoded gob")
	}
	rawbuf[idx] = 0x0      // perturb data
	err = dec.Decode(&tsd) // should return error
	if err == nil || err.Error() != "no protocol with code 0" {
		t.Errorf("Did not decode invalid mutiaddr : %v", err)
	}

	// fail to decode an invalid nodeid
	b.Reset()
	tsd = TranscodeSubMsg{}
	enc.Encode(ts)
	rawbuf = b.Bytes()
	idx = bytes.Index(rawbuf, []byte(nodeID))
	if idx == -1 {
		t.Error("Unable to find nodeid encoded gob")
	}
	rawbuf[idx] = 0xff // perturb data
	err = dec.Decode(&tsd)
	if err == nil ||
		!(err.Error()[0:29] == "multihash length inconsistent" ||
			err.Error()[0:15] == "digest too long") {
		t.Error(err)
	}
}
