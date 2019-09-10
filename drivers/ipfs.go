package drivers

/*
import (
	"bytes"
	"fmt"

	"github.com/livepeer/go-livepeer/ipfs"
	"github.com/livepeer/go-livepeer/net"
)

type ipfsOS struct {
}

var ipfsAPI ipfs.IpfsApi

func GetSegmentDataIpfs(uri string) ([]byte, error) {
	return nil, fmt.Errorf("NotImplemented")
}

// SetIpfsAPI ...
func SetIpfsAPI(api ipfs.IpfsApi) {
	ipfsAPI = api
}

func newIPFSSession() OSSession {
	od := &ipfsOS{}
	return od
}

func (os *ipfsOS) IsExternal() bool {
	return true
}

// GetInfo
func (os *ipfsOS) GetInfo() *net.OSInfo {
	info := net.OSInfo{
		StorageType: net.OSInfo_IPFS,
	}
	return &info
}

func (os *ipfsOS) EndSession() {

}

func (os *ipfsOS) SaveData(name string, data []byte) (string, error) {

	reader := bytes.NewReader(data)
	url, err := ipfsAPI.Add(reader)
	return url, err
}
*/
