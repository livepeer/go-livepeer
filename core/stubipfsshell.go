package core

import (
	"fmt"
	"io"
	"io/ioutil"
	"time"

	ipfsApi "github.com/ipfs/go-ipfs-api"
)

type StubShell struct{}

func (s *StubShell) Add(r io.Reader) (string, error) { return "", nil }
func (s *StubShell) AddOnlyHash(r io.Reader) (string, error) {
	//Add HASH to the end of the string
	d, _ := ioutil.ReadAll(r)
	return fmt.Sprintf("%sHash", d), nil
}
func (s *StubShell) AddDir(dir string) (string, error)                                 { return "", nil }
func (s *StubShell) AddLink(target string) (string, error)                             { return "", nil }
func (s *StubShell) AddNoPin(r io.Reader) (string, error)                              { return "", nil }
func (s *StubShell) BlockGet(path string) ([]byte, error)                              { return []byte(""), nil }
func (s *StubShell) BlockPut(block []byte) (string, error)                             { return "", nil }
func (s *StubShell) BlockStat(path string) (string, int, error)                        { return "", 0, nil }
func (s *StubShell) Cat(path string) (io.ReadCloser, error)                            { return nil, nil }
func (s *StubShell) DagGet(ref string, out interface{}) error                          { return nil }
func (s *StubShell) DagPut(data interface{}, ienc, kind string) (string, error)        { return "", nil }
func (s *StubShell) DiagNet(format string) ([]byte, error)                             { return []byte(""), nil }
func (s *StubShell) FileList(path string) (*ipfsApi.UnixLsObject, error)               { return nil, nil }
func (s *StubShell) FindPeer(peer string) (*ipfsApi.PeerInfo, error)                   { return nil, nil }
func (s *StubShell) Get(hash, outdir string) error                                     { return nil }
func (s *StubShell) ID(peer ...string) (*ipfsApi.IdOutput, error)                      { return nil, nil }
func (s *StubShell) IsUp() bool                                                        { return true }
func (s *StubShell) List(path string) ([]*ipfsApi.LsLink, error)                       { return nil, nil }
func (s *StubShell) NewObject(template string) (string, error)                         { return "", nil }
func (s *StubShell) ObjectGet(path string) (*ipfsApi.IpfsObject, error)                { return nil, nil }
func (s *StubShell) ObjectPut(obj *ipfsApi.IpfsObject) (string, error)                 { return "", nil }
func (s *StubShell) ObjectStat(key string) (*ipfsApi.ObjectStats, error)               { return nil, nil }
func (s *StubShell) Patch(root, action string, args ...string) (string, error)         { return "", nil }
func (s *StubShell) PatchData(root string, set bool, data interface{}) (string, error) { return "", nil }
func (s *StubShell) PatchLink(root, path, childhash string, create bool) (string, error) {
	return "", nil
}
func (s *StubShell) Pin(path string) error                        { return nil }
func (s *StubShell) Pins() (map[string]ipfsApi.PinInfo, error)    { return nil, nil }
func (s *StubShell) PubSubPublish(topic, data string) (err error) { return nil }
func (s *StubShell) PubSubSubscribe(topic string) (*ipfsApi.PubSubSubscription, error) {
	return nil, nil
}
func (s *StubShell) Publish(node string, value string) error                 { return nil }
func (s *StubShell) Refs(hash string, recursive bool) (<-chan string, error) { return nil, nil }
func (s *StubShell) Resolve(id string) (string, error)                       { return "", nil }
func (s *StubShell) ResolvePath(path string) (string, error)                 { return "", nil }
func (s *StubShell) SetTimeout(d time.Duration)                              {}
func (s *StubShell) Unpin(path string) error                                 { return nil }
func (s *StubShell) Version() (string, string, error)                        { return "", "", nil }
