// package shell implements a remote API interface for a running ipfs daemon
package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	gohttp "net/http"
	"os"
	"path"
	"strings"
	"time"

	homedir "github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	files "github.com/whyrusleeping/go-multipart-files"
	tar "github.com/whyrusleeping/tar-utils"
)

const (
	DefaultPathName = ".ipfs"
	DefaultPathRoot = "~/" + DefaultPathName
	DefaultApiFile  = "api"
	EnvDir          = "IPFS_PATH"
)

type Shell struct {
	url     string
	httpcli *gohttp.Client
}

func NewLocalShell() *Shell {
	baseDir := os.Getenv(EnvDir)
	if baseDir == "" {
		baseDir = DefaultPathRoot
	}

	baseDir, err := homedir.Expand(baseDir)
	if err != nil {
		return nil
	}

	apiFile := path.Join(baseDir, DefaultApiFile)

	if _, err := os.Stat(apiFile); err != nil {
		return nil
	}

	api, err := ioutil.ReadFile(apiFile)
	if err != nil {
		return nil
	}

	return NewShell(strings.TrimSpace(string(api)))
}

func NewShell(url string) *Shell {
	c := &gohttp.Client{
		Transport: &gohttp.Transport{
			DisableKeepAlives: true,
		},
	}

	return NewShellWithClient(url, c)
}

func NewShellWithClient(url string, c *gohttp.Client) *Shell {
	if a, err := ma.NewMultiaddr(url); err == nil {
		_, host, err := manet.DialArgs(a)
		if err == nil {
			url = host
		}
	}

	return &Shell{
		url:     url,
		httpcli: c,
	}
}

func (s *Shell) SetTimeout(d time.Duration) {
	s.httpcli.Timeout = d
}

func (s *Shell) newRequest(ctx context.Context, command string, args ...string) *Request {
	return NewRequest(ctx, s.url, command, args...)
}

type IdOutput struct {
	ID              string
	PublicKey       string
	Addresses       []string
	AgentVersion    string
	ProtocolVersion string
}

// ID gets information about a given peer.  Arguments:
//
// peer: peer.ID of the node to look up.  If no peer is specified,
//   return information about the local peer.
func (s *Shell) ID(peer ...string) (*IdOutput, error) {
	if len(peer) > 1 {
		return nil, fmt.Errorf("Too many peer arguments")
	}

	resp, err := NewRequest(context.Background(), s.url, "id", peer...).Send(s.httpcli)
	if err != nil {
		return nil, err
	}

	defer resp.Close()
	if resp.Error != nil {
		return nil, resp.Error
	}

	decoder := json.NewDecoder(resp.Output)
	out := new(IdOutput)
	err = decoder.Decode(out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// Cat the content at the given path. Callers need to drain and close the returned reader after usage.
func (s *Shell) Cat(path string) (io.ReadCloser, error) {
	resp, err := NewRequest(context.Background(), s.url, "cat", path).Send(s.httpcli)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}

	return resp.Output, nil
}

type object struct {
	Hash string
}

// Add a file to ipfs from the given reader, returns the hash of the added file
func (s *Shell) Add(r io.Reader) (string, error) {
	return s.addWithOpts(r, true)
}

// AddNoPin a file to ipfs from the given reader, returns the hash of the added file without pinning the file
func (s *Shell) AddNoPin(r io.Reader) (string, error) {
	return s.addWithOpts(r, false)
}

func (s *Shell) addWithOpts(r io.Reader, pin bool) (string, error) {
	var rc io.ReadCloser
	if rclose, ok := r.(io.ReadCloser); ok {
		rc = rclose
	} else {
		rc = ioutil.NopCloser(r)
	}

	// handler expects an array of files
	fr := files.NewReaderFile("", "", rc, nil)
	slf := files.NewSliceFile("", "", []files.File{fr})
	fileReader := files.NewMultiFileReader(slf, true)

	req := NewRequest(context.Background(), s.url, "add")
	req.Body = fileReader
	req.Opts["progress"] = "false"
	if !pin {
		req.Opts["pin"] = "false"
	}

	resp, err := req.Send(s.httpcli)
	if err != nil {
		return "", err
	}
	defer resp.Close()
	if resp.Error != nil {
		return "", resp.Error
	}

	var out object
	err = json.NewDecoder(resp.Output).Decode(&out)
	if err != nil {
		return "", err
	}

	return out.Hash, nil
}

func (s *Shell) AddLink(target string) (string, error) {
	link := files.NewLinkFile("", "", target, nil)
	slf := files.NewSliceFile("", "", []files.File{link})
	reader := files.NewMultiFileReader(slf, true)

	req := s.newRequest(context.Background(), "add")
	req.Body = reader

	resp, err := req.Send(s.httpcli)
	if err != nil {
		return "", err
	}
	defer resp.Close()
	if resp.Error != nil {
		return "", resp.Error
	}

	var out object
	err = json.NewDecoder(resp.Output).Decode(&out)
	if err != nil {
		return "", err
	}

	return out.Hash, nil
}

// AddDir adds a directory recursively with all of the files under it
func (s *Shell) AddDir(dir string) (string, error) {
	stat, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}

	sf, err := files.NewSerialFile("", dir, stat)
	if err != nil {
		return "", err
	}
	slf := files.NewSliceFile("", dir, []files.File{sf})
	reader := files.NewMultiFileReader(slf, true)

	req := NewRequest(context.Background(), s.url, "add")
	req.Opts["r"] = "true"
	req.Body = reader

	resp, err := req.Send(s.httpcli)
	if err != nil {
		return "", err
	}
	defer resp.Close()
	if resp.Error != nil {
		return "", resp.Error
	}

	dec := json.NewDecoder(resp.Output)
	var final string
	for {
		var out object
		err = dec.Decode(&out)
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		final = out.Hash
	}

	if final == "" {
		return "", errors.New("no results received")
	}

	return final, nil
}

const (
	TRaw = iota
	TDirectory
	TFile
	TMetadata
	TSymlink
)

// List entries at the given path
func (s *Shell) List(path string) ([]*LsLink, error) {
	resp, err := NewRequest(context.Background(), s.url, "ls", path).Send(s.httpcli)
	if err != nil {
		return nil, err
	}
	defer resp.Close()

	if resp.Error != nil {
		return nil, resp.Error
	}

	var out struct{ Objects []LsObject }
	err = json.NewDecoder(resp.Output).Decode(&out)
	if err != nil {
		return nil, err
	}

	return out.Objects[0].Links, nil
}

type LsLink struct {
	Hash string
	Name string
	Size uint64
	Type int
}

type LsObject struct {
	Links []*LsLink
	LsLink
}

// Pin the given path
func (s *Shell) Pin(path string) error {
	req := NewRequest(context.Background(), s.url, "pin/add", path)
	req.Opts["r"] = "true"

	resp, err := req.Send(s.httpcli)
	if err != nil {
		return err
	}
	defer resp.Close()
	if resp.Error != nil {
		return resp.Error
	}

	return nil
}

// Unpin the given path
func (s *Shell) Unpin(path string) error {
	req := NewRequest(context.Background(), s.url, "pin/rm", path)
	req.Opts["r"] = "true"

	resp, err := req.Send(s.httpcli)
	if err != nil {
		return err
	}
	defer resp.Close()
	if resp.Error != nil {
		return resp.Error
	}

	return nil
}

const (
	DirectPin    = "direct"
	RecursivePin = "recursive"
	IndirectPin  = "indirect"
)

type PinInfo struct {
	Type string
}

// Pins returns a map of the pin hashes to their info (currently just the
// pin type, one of DirectPin, RecursivePin, or IndirectPin. A map is returned
// instead of a slice because it is easier to do existence lookup by map key
// than unordered array searching. The map is likely to be more useful to a
// client than a flat list.
func (s *Shell) Pins() (map[string]PinInfo, error) {
	resp, err := s.newRequest(context.Background(), "pin/ls").Send(s.httpcli)
	if err != nil {
		return nil, err
	}
	defer resp.Close()

	if resp.Error != nil {
		return nil, resp.Error
	}

	raw := struct{ Keys map[string]PinInfo }{}
	err = json.NewDecoder(resp.Output).Decode(&raw)
	if err != nil {
		return nil, err
	}

	return raw.Keys, nil
}

type PeerInfo struct {
	Addrs []string
	ID    string
}

func (s *Shell) FindPeer(peer string) (*PeerInfo, error) {
	resp, err := s.newRequest(context.Background(), "dht/findpeer", peer).Send(s.httpcli)
	if err != nil {
		return nil, err
	}
	defer resp.Close()

	if resp.Error != nil {
		return nil, resp.Error
	}

	str := struct{ Responses []PeerInfo }{}
	err = json.NewDecoder(resp.Output).Decode(&str)
	if err != nil {
		return nil, err
	}

	if len(str.Responses) == 0 {
		return nil, errors.New("peer not found")
	}

	return &str.Responses[0], nil
}

func (s *Shell) Refs(hash string, recursive bool) (<-chan string, error) {
	req := s.newRequest(context.Background(), "refs", hash)
	if recursive {
		req.Opts["r"] = "true"
	}

	resp, err := req.Send(s.httpcli)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	out := make(chan string)
	go func() {
		var ref struct {
			Ref string
		}
		defer resp.Close()
		defer close(out)
		dec := json.NewDecoder(resp.Output)
		for {
			err := dec.Decode(&ref)
			if err != nil {
				return
			}
			if len(ref.Ref) > 0 {
				out <- ref.Ref
			}
		}
	}()

	return out, nil
}

func (s *Shell) Patch(root, action string, args ...string) (string, error) {
	cmdargs := append([]string{root}, args...)
	resp, err := s.newRequest(context.Background(), "object/patch/"+action, cmdargs...).Send(s.httpcli)
	if err != nil {
		return "", err
	}
	defer resp.Close()

	if resp.Error != nil {
		return "", resp.Error
	}

	dec := json.NewDecoder(resp.Output)
	var out object
	err = dec.Decode(&out)
	if err != nil {
		return "", err
	}

	return out.Hash, nil
}

func (s *Shell) PatchData(root string, set bool, data interface{}) (string, error) {
	var read io.Reader
	switch d := data.(type) {
	case io.Reader:
		read = d
	case []byte:
		read = bytes.NewReader(d)
	case string:
		read = strings.NewReader(d)
	default:
		return "", fmt.Errorf("unrecognized type: %#v", data)
	}

	cmd := "append-data"
	if set {
		cmd = "set-data"
	}

	fr := files.NewReaderFile("", "", ioutil.NopCloser(read), nil)
	slf := files.NewSliceFile("", "", []files.File{fr})
	fileReader := files.NewMultiFileReader(slf, true)

	req := s.newRequest(context.Background(), "object/patch/"+cmd, root)
	req.Body = fileReader

	resp, err := req.Send(s.httpcli)
	if err != nil {
		return "", err
	}
	defer resp.Close()

	if resp.Error != nil {
		return "", resp.Error
	}

	dec := json.NewDecoder(resp.Output)
	var out object
	err = dec.Decode(&out)
	if err != nil {
		return "", err
	}

	return out.Hash, nil
}

func (s *Shell) PatchLink(root, path, childhash string, create bool) (string, error) {
	cmdargs := []string{root, path, childhash}

	req := s.newRequest(context.Background(), "object/patch/add-link", cmdargs...)
	if create {
		req.Opts["create"] = "true"
	}

	resp, err := req.Send(s.httpcli)
	if err != nil {
		return "", err
	}
	defer resp.Close()

	if resp.Error != nil {
		return "", resp.Error
	}

	var out object
	err = json.NewDecoder(resp.Output).Decode(&out)
	if err != nil {
		return "", err
	}

	return out.Hash, nil
}

func (s *Shell) Get(hash, outdir string) error {
	resp, err := s.newRequest(context.Background(), "get", hash).Send(s.httpcli)
	if err != nil {
		return err
	}
	defer resp.Close()

	if resp.Error != nil {
		return resp.Error
	}

	extractor := &tar.Extractor{Path: outdir}
	return extractor.Extract(resp.Output)
}

func (s *Shell) NewObject(template string) (string, error) {
	args := []string{}
	if template != "" {
		args = []string{template}
	}

	resp, err := s.newRequest(context.Background(), "object/new", args...).Send(s.httpcli)
	if err != nil {
		return "", err
	}
	defer resp.Close()

	if resp.Error != nil {
		return "", resp.Error
	}

	var out object
	err = json.NewDecoder(resp.Output).Decode(&out)
	if err != nil {
		return "", err
	}

	return out.Hash, nil
}

func (s *Shell) ResolvePath(path string) (string, error) {
	resp, err := s.newRequest(context.Background(), "object/stat", path).Send(s.httpcli)
	if err != nil {
		return "", err
	}
	defer resp.Close()

	if resp.Error != nil {
		return "", resp.Error
	}

	var out object
	err = json.NewDecoder(resp.Output).Decode(&out)
	if err != nil {
		return "", err
	}

	return out.Hash, nil
}

// returns ipfs version and commit sha
func (s *Shell) Version() (string, string, error) {
	resp, err := s.newRequest(context.Background(), "version").Send(s.httpcli)
	if err != nil {
		return "", "", err
	}

	defer resp.Close()
	if resp.Error != nil {
		return "", "", resp.Error
	}

	ver := struct {
		Version string
		Commit  string
	}{}

	err = json.NewDecoder(resp.Output).Decode(&ver)
	if err != nil {
		return "", "", err
	}

	return ver.Version, ver.Commit, nil
}

func (s *Shell) IsUp() bool {
	_, _, err := s.Version()
	return err == nil
}

func (s *Shell) BlockStat(path string) (string, int, error) {
	resp, err := s.newRequest(context.Background(), "block/stat", path).Send(s.httpcli)
	if err != nil {
		return "", 0, err
	}
	defer resp.Close()

	if resp.Error != nil {
		return "", 0, resp.Error
	}

	var inf struct {
		Key  string
		Size int
	}

	err = json.NewDecoder(resp.Output).Decode(&inf)
	if err != nil {
		return "", 0, err
	}

	return inf.Key, inf.Size, nil
}

func (s *Shell) BlockGet(path string) ([]byte, error) {
	resp, err := s.newRequest(context.Background(), "block/get", path).Send(s.httpcli)
	if err != nil {
		return nil, err
	}
	defer resp.Close()

	if resp.Error != nil {
		return nil, resp.Error
	}

	return ioutil.ReadAll(resp.Output)
}

func (s *Shell) BlockPut(block []byte) (string, error) {
	data := bytes.NewReader(block)
	rc := ioutil.NopCloser(data)
	fr := files.NewReaderFile("", "", rc, nil)
	slf := files.NewSliceFile("", "", []files.File{fr})
	fileReader := files.NewMultiFileReader(slf, true)

	req := s.newRequest(context.Background(), "block/put")
	req.Body = fileReader
	resp, err := req.Send(s.httpcli)
	if err != nil {
		return "", err
	}
	defer resp.Close()

	if resp.Error != nil {
		return "", resp.Error
	}

	var out struct {
		Key string
	}
	err = json.NewDecoder(resp.Output).Decode(&out)
	if err != nil {
		return "", err
	}

	return out.Key, nil
}

type IpfsObject struct {
	Links []ObjectLink
	Data  string
}

type ObjectLink struct {
	Name, Hash string
	Size       uint64
}

func (s *Shell) ObjectGet(path string) (*IpfsObject, error) {
	resp, err := s.newRequest(context.Background(), "object/get", path).Send(s.httpcli)
	if err != nil {
		return nil, err
	}
	defer resp.Close()

	if resp.Error != nil {
		return nil, resp.Error
	}

	var obj IpfsObject
	err = json.NewDecoder(resp.Output).Decode(&obj)
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

func (s *Shell) ObjectPut(obj *IpfsObject) (string, error) {
	data := new(bytes.Buffer)
	err := json.NewEncoder(data).Encode(obj)
	if err != nil {
		return "", err
	}

	rc := ioutil.NopCloser(data)

	fr := files.NewReaderFile("", "", rc, nil)
	slf := files.NewSliceFile("", "", []files.File{fr})
	fileReader := files.NewMultiFileReader(slf, true)

	req := s.newRequest(context.Background(), "object/put")
	req.Body = fileReader
	resp, err := req.Send(s.httpcli)
	if err != nil {
		return "", err
	}
	defer resp.Close()

	if resp.Error != nil {
		return "", resp.Error
	}

	var out object
	err = json.NewDecoder(resp.Output).Decode(&out)
	if err != nil {
		return "", err
	}

	return out.Hash, nil
}

func (s *Shell) PubSubSubscribe(topic string) (*PubSubSubscription, error) {
	// connect
	req := s.newRequest(context.Background(), "pubsub/sub", topic)

	resp, err := req.Send(s.httpcli)
	if err != nil {
		return nil, err
	}

	return newPubSubSubscription(resp), nil
}

func (s *Shell) PubSubPublish(topic, data string) (err error) {
	resp, err := s.newRequest(context.Background(), "pubsub/pub", topic, data).Send(s.httpcli)
	if err != nil {
		return
	}
	defer func() {
		err1 := resp.Close()
		if err == nil {
			err = err1
		}
	}()

	return nil
}

type ObjectStats struct {
	Hash           string
	BlockSize      int
	CumulativeSize int
	DataSize       int
	LinksSize      int
	NumLinks       int
}

// ObjectStat gets stats for the DAG object named by key. It returns
// the stats of the requested Object or an error.
func (s *Shell) ObjectStat(key string) (*ObjectStats, error) {
	resp, err := s.newRequest(context.Background(), "object/stat", key).Send(s.httpcli)
	if err != nil {
		return nil, err
	}
	defer resp.Close()

	if resp.Error != nil {
		return nil, resp.Error
	}

	stat := &ObjectStats{}

	err = json.NewDecoder(resp.Output).Decode(stat)
	if err != nil {
		return nil, err
	}

	return stat, nil
}

func (s *Shell) DiagNet(format string) ([]byte, error) {
	var result = new(bytes.Buffer)

	req := s.newRequest(context.Background(), "diag/net")
	req.Opts["vis"] = format

	resp, err := req.Send(s.httpcli)
	if err != nil {
		return []byte{}, err
	}

	defer resp.Close()
	if resp.Error != nil {
		return []byte{}, resp.Error
	}

	result.ReadFrom(resp.Output)

	return result.Bytes(), nil
}
