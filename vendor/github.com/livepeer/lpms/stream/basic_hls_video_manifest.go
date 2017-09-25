package stream

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
)

var ErrVideoManifest = errors.New("ErrVideoManifest")

type BasicHLSVideoManifest struct {
	streamMap     map[string]HLSVideoStream
	variantMap    map[string]*m3u8.Variant
	manifestCache *m3u8.MasterPlaylist
	id            string
}

func NewBasicHLSVideoManifest(id string) *BasicHLSVideoManifest {
	pl := m3u8.NewMasterPlaylist()
	return &BasicHLSVideoManifest{
		streamMap:     make(map[string]HLSVideoStream),
		variantMap:    make(map[string]*m3u8.Variant),
		manifestCache: pl,
		id:            id,
	}
}

func (m *BasicHLSVideoManifest) GetManifestID() string { return m.id }

func (m *BasicHLSVideoManifest) GetVideoFormat() VideoFormat { return HLS }

func (m *BasicHLSVideoManifest) GetManifest() (*m3u8.MasterPlaylist, error) {
	return m.manifestCache, nil
}

func (m *BasicHLSVideoManifest) GetVideoStream(strmID string) (HLSVideoStream, error) {
	strm, ok := m.streamMap[strmID]
	if !ok {
		return nil, ErrNotFound
	}
	return strm, nil
}

func (m *BasicHLSVideoManifest) GetVideoStreams() []HLSVideoStream {
	res := []HLSVideoStream{}
	for _, s := range m.streamMap {
		res = append(res, s)
	}
	return res
}

func (m *BasicHLSVideoManifest) AddVideoStream(strm HLSVideoStream, variant *m3u8.Variant) error {
	_, ok := m.streamMap[strm.GetStreamID()]
	if ok {
		glog.Errorf("Video %v already in manifest stream map")
		return ErrVideoManifest
	}

	//Check if the same Bandwidth & Resolution already exists
	for _, v := range m.variantMap {
		// v := mStrm.GetStreamVariant()
		if v.Bandwidth == variant.Bandwidth && v.Resolution == variant.Resolution {
			// if v.Bandwidth == strm.GetStreamVariant().Bandwidth && v.Resolution == strm.GetStreamVariant().Resolution {
			glog.Errorf("Variant with Bandwidth %v and Resolution %v already exists", v.Bandwidth, v.Resolution)
			return ErrVideoManifest
		}
	}

	//Add to the map
	// m.manifestCache.Append(strm.GetStreamVariant().URI, strm.GetStreamVariant().Chunklist, strm.GetStreamVariant().VariantParams)
	m.manifestCache.Append(variant.URI, variant.Chunklist, variant.VariantParams)
	m.streamMap[strm.GetStreamID()] = strm
	m.variantMap[strm.GetStreamID()] = variant
	return nil
}

func (m *BasicHLSVideoManifest) GetStreamVariant(strmID string) (*m3u8.Variant, error) {
	//Try from the variant map
	v, ok := m.variantMap[strmID]
	if ok {
		return v, nil
	}

	//Try from the playlist itself
	for _, v := range m.manifestCache.Variants {
		vsid := strings.Split(v.URI, ".")[0]
		if vsid == strmID {
			return v, nil
		}
	}
	return nil, ErrNotFound
}

func (m *BasicHLSVideoManifest) DeleteVideoStream(strmID string) error {
	delete(m.streamMap, strmID)
	return nil
}

func (m BasicHLSVideoManifest) String() string {
	return fmt.Sprintf("id:%v, streams:%v", m.id, m.streamMap)
}
