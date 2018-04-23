package basicnet

import (
	"context"
	"fmt"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	peerstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"io/ioutil"
	"strings"
	"time"

	"github.com/golang/glog"
)

type PeerCache struct {
	Peerstore peerstore.Peerstore
	Filename  string
}

func NewPeerCache(peerStore peerstore.Peerstore, filename string) *PeerCache {
	return &PeerCache{Peerstore: peerStore, Filename: filename}
}

//LoadPeers Load peer info from a file and try to connect to them
func (pc *PeerCache) LoadPeers() []peerstore.PeerInfo {
	bytes, err := ioutil.ReadFile(pc.Filename)
	peers := make([]peerstore.PeerInfo, 0)
	if err == nil {
		for _, line := range strings.Split(string(bytes), "\n") {
			larr := strings.Split(line, "|")
			if len(larr) == 2 {
				pid, err := peer.IDHexDecode(larr[0])
				if err != nil {
					continue
				}

				addrs := strings.Split(larr[1], ",")
				maAddrs := make([]ma.Multiaddr, 0)
				for _, addr := range addrs {
					maAddr, err := ma.NewMultiaddr(addr)
					if err != nil {
						continue
					}
					maAddrs = append(maAddrs, maAddr)
				}

				peers = append(peers, peerstore.PeerInfo{ID: pid, Addrs: maAddrs})
			}
		}
	}
	return peers
}

//Record Periodically write peers to a file
func (pc *PeerCache) Record(ctx context.Context) {
	ticker := time.NewTicker(ConnFileWriteFreq)
	for {
		select {
		case <-ticker.C:
			peers := pc.Peerstore.Peers()
			if len(peers) == 0 {
				continue
			}

			str := ""
			for _, p := range peers {
				pInfo := pc.Peerstore.PeerInfo(p)
				if len(pInfo.Addrs) > 0 {
					addrsStr := make([]string, 0)
					for _, addr := range pInfo.Addrs {
						addrsStr = append(addrsStr, addr.String())
					}
					str = fmt.Sprintf("%v\n%v|%v", str, peer.IDHexEncode(pInfo.ID), strings.Join(addrsStr, ","))
				}
			}
			if len(str) > 0 {
				if err := ioutil.WriteFile(pc.Filename, []byte(str), 0644); err != nil {
					glog.Errorf("Error writing connection to file system: %v", err)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
