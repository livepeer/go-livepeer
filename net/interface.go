package net

import (
	"fmt"
	"strings"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
)

type NodeStatus struct {
	Manifests map[string]*m3u8.MasterPlaylist
}

func (n NodeStatus) String() string {
	mstrs := make([]string, 0)
	for mid, m := range n.Manifests {
		mstrs = append(mstrs, fmt.Sprintf("%v[]%v", mid, m.String()))
	}
	return strings.Join(mstrs, "|")
}

func (n *NodeStatus) FromString(str string) error {
	arr := strings.Split(str, "|")

	manifests := make(map[string]*m3u8.MasterPlaylist, 0)
	for _, mstr := range arr {
		//Decode the playlist from a string
		mstrArr := strings.Split(mstr, "[]")
		if len(mstrArr) == 2 {
			m := m3u8.NewMasterPlaylist()
			if err := m.DecodeFrom(strings.NewReader(mstrArr[1]), true); err != nil {
				glog.Errorf("Error decoding playlist: %v", err)
			} else {
				manifests[mstrArr[0]] = m
			}
		}
	}
	n.Manifests = manifests

	return nil
}
