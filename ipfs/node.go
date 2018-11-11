package ipfs

import (
	"context"
	chunk "gx/ipfs/QmWo8jYc19ppG7YoTsrr2kEtLRbARTJho5oNXFTR6B7Peq/go-ipfs-chunker"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	files "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit/files"

	mfs "github.com/ipfs/go-ipfs/mfs"
	unixfs "github.com/ipfs/go-ipfs/unixfs"

	"github.com/golang/glog"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreunix"
	"github.com/ipfs/go-ipfs/importer/balanced"
	ihelper "github.com/ipfs/go-ipfs/importer/helpers"
	"github.com/ipfs/go-ipfs/importer/trickle"
	"github.com/ipfs/go-ipfs/path"
	resolver "github.com/ipfs/go-ipfs/path/resolver"
	"github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
)

type IpfsApi interface {
	Add(r io.Reader) (string, error)
	// Adds file to existing dir or to new one
	// returns hash of the new dir
	AddToDir(dir, fileName string, r io.Reader) (string, error)
}

type IpfsCoreApi core.IpfsNode

const (
	nBitsForKeypairDefault = 2048
)

func StartIpfs(ctx context.Context, repoPath string) (*IpfsCoreApi, error) {
	if !fsrepo.IsInitialized(repoPath) {
		conf, err := config.Init(os.Stdout, nBitsForKeypairDefault)
		if err != nil {
			return nil, err
		}
		if err := fsrepo.Init(repoPath, conf); err != nil {
			return nil, err
		}
	}

	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, err
	}

	ncfg := &core.BuildCfg{
		Repo:      repo,
		Online:    true,
		Permanent: true,
		Routing:   core.DHTOption,
	}

	node, err := core.NewNode(ctx, ncfg)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				glog.Infof("Closing IPFS...")
				closeIpfs(node, repoPath)
				return
			}
		}
	}()

	return (*IpfsCoreApi)(node), nil
}

func closeIpfs(node *core.IpfsNode, repoPath string) {
	repoLockFile := filepath.Join(repoPath, fsrepo.LockFile)
	os.Remove(repoLockFile)
	node.Close()
}

func (ipfs *IpfsCoreApi) Add(r io.Reader) (string, error) {
	return addAndPin(ipfs.node(), r)
}

func (ipfs *IpfsCoreApi) AddToDir(dirHash, fileName string, r io.Reader) (string, error) {
	n := ipfs.node()

	file := files.NewReaderFile(fileName, fileName, ioutil.NopCloser(r), nil)
	fileAdder, err := coreunix.NewAdder(n.Context(), n.Pinning, n.Blockstore, n.DAG)
	if err != nil {
		return "", err
	}
	fileAdder.Wrap = true
	fileAdder.Pin = true
	rnode := unixfs.EmptyDirNode()
	mr, err := mfs.NewRoot(n.Context(), n.DAG, rnode, nil)
	if err != nil {
		return "", err
	}
	if dirHash != "" {
		p, err := path.ParsePath(dirHash)
		if err != nil {
			return "", err
		}

		r := &resolver.Resolver{
			DAG:         n.DAG,
			ResolveOnce: uio.ResolveUnixfsOnce,
		}

		dagnode, err := core.Resolve(n.Context(), n.Namesys, r, p)
		if err != nil {
			return "", err
		}
		dir, err := uio.NewDirectoryFromNode(n.DAG, dagnode)
		if err != nil && err != uio.ErrNotADir {
			return "", err
		}

		var links []*ipld.Link
		if dir == nil {
			links = dagnode.Links()
		} else {
			links, err = dir.Links(n.Context())
			if err != nil {
				return "", err
			}
		}
		for _, link := range links {
			lnode, err := link.GetNode(n.Context(), n.DAG)
			if err != nil {
				return "", err
			}
			if err := mfs.PutNode(mr, link.Name, lnode); err != nil {
				return "", err
			}
		}
		fileAdder.SetMfsRoot(mr)
	}
	defer n.Blockstore.PinLock().Unlock()

	err = fileAdder.AddFile(file)
	if err != nil {
		return "", err
	}

	dagnode, err := fileAdder.Finalize()
	if err != nil {
		return "", err
	}

	err = fileAdder.PinRoot()
	if err != nil {
		return "", err
	}

	c := dagnode.Cid()
	return c.String(), nil
}

func addAndPin(n *core.IpfsNode, r io.Reader) (string, error) {
	defer n.Blockstore.PinLock().Unlock()

	fileAdder, err := coreunix.NewAdder(n.Context(), n.Pinning, n.Blockstore, n.DAG)
	if err != nil {
		return "", err
	}

	chnk, err := chunk.FromString(r, fileAdder.Chunker)
	if err != nil {
		return "", err
	}

	params := ihelper.DagBuilderParams{
		Dagserv:   n.DAG,
		RawLeaves: fileAdder.RawLeaves,
		Maxlinks:  ihelper.DefaultLinksPerBlock,
		NoCopy:    fileAdder.NoCopy,
		Prefix:    fileAdder.Prefix,
	}

	var node ipld.Node
	if fileAdder.Trickle {
		node, err = trickle.Layout(params.New(chnk))
		if err != nil {
			return "", err
		}
	} else {
		node, err = balanced.Layout(params.New(chnk))
		if err != nil {
			return "", err
		}
	}

	err = fileAdder.PinRoot()
	if err != nil {
		return "", err
	}

	return node.Cid().String(), nil
}

func (ipfs *IpfsCoreApi) node() *core.IpfsNode {
	return (*core.IpfsNode)(ipfs)
}
