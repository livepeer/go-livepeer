package drivers

import (
	"bytes"
	"fmt"

	"github.com/livepeer/go-livepeer/ipfs"
	"github.com/livepeer/go-livepeer/net"
)

type ipfsOS struct {
}

type ipfsOSSession struct {
	jobID      int64
	manifestID string
}

var ipfsAPI ipfs.IpfsApi

func GetSegmentDataIpfs(typedURI *net.TypedURI) ([]byte, error) {
	return nil, fmt.Errorf("NotImplemented")
}

// SetIpfsAPI ...
func SetIpfsAPI(api ipfs.IpfsApi) {
	ipfsAPI = api
}

func newIPSFDriver() OSDriver {
	od := &ipfsOS{}
	return od
}

func (os *ipfsOS) IsExternal() bool {
	return true
}

func (os *ipfsOS) StartSession(jobID int64, manifestID string, nonce uint64) OSSession {
	s := ipfsOSSession{
		jobID:      jobID,
		manifestID: manifestID,
	}
	return &s
}

func (session *ipfsOSSession) IsOwnStorage(turi *net.TypedURI) bool {
	return turi.Storage == "ipfs"
}

func (session *ipfsOSSession) EndSession() {

}

func (session *ipfsOSSession) GetInfo() net.OSInfo {
	info := net.OSInfo{}
	return info
}

func (session *ipfsOSSession) SaveData(streamID, name string, data []byte) (*net.TypedURI, string, error) {
	reader := bytes.NewReader(data)
	url, err := ipfsAPI.Add(reader)
	abs := "ipfs://" + url
	turl := &net.TypedURI{
		Storage:       "ipfs",
		Uri:           url,
		StreamID:      streamID,
		UriInManifest: abs,
	}
	return turl, abs, err
}
