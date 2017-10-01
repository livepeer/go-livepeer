package ipfs

import (
	"io"
	"time"

	ipfsApi "github.com/ipfs/go-ipfs-api"
)

type IPFSShell interface {
	Add(r io.Reader) (string, error)
	AddOnlyHash(r io.Reader) (string, error)
	AddDir(dir string) (string, error)
	AddLink(target string) (string, error)
	AddNoPin(r io.Reader) (string, error)
	BlockGet(path string) ([]byte, error)
	BlockPut(block []byte) (string, error)
	BlockStat(path string) (string, int, error)
	Cat(path string) (io.ReadCloser, error)
	DagGet(ref string, out interface{}) error
	DagPut(data interface{}, ienc, kind string) (string, error)
	DiagNet(format string) ([]byte, error)
	FileList(path string) (*ipfsApi.UnixLsObject, error)
	FindPeer(peer string) (*ipfsApi.PeerInfo, error)
	Get(hash, outdir string) error
	ID(peer ...string) (*ipfsApi.IdOutput, error)
	IsUp() bool
	List(path string) ([]*ipfsApi.LsLink, error)
	NewObject(template string) (string, error)
	ObjectGet(path string) (*ipfsApi.IpfsObject, error)
	ObjectPut(obj *ipfsApi.IpfsObject) (string, error)
	ObjectStat(key string) (*ipfsApi.ObjectStats, error)
	Patch(root, action string, args ...string) (string, error)
	PatchData(root string, set bool, data interface{}) (string, error)
	PatchLink(root, path, childhash string, create bool) (string, error)
	Pin(path string) error
	Pins() (map[string]ipfsApi.PinInfo, error)
	PubSubPublish(topic, data string) (err error)
	PubSubSubscribe(topic string) (*ipfsApi.PubSubSubscription, error)
	Publish(node string, value string) error
	Refs(hash string, recursive bool) (<-chan string, error)
	Resolve(id string) (string, error)
	ResolvePath(path string) (string, error)
	SetTimeout(d time.Duration)
	Unpin(path string) error
	Version() (string, string, error)
}
