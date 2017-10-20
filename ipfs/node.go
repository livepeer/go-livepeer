package ipfs

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreunix"
	"github.com/ipfs/go-ipfs/importer/balanced"
	"github.com/ipfs/go-ipfs/importer/chunk"
	ihelper "github.com/ipfs/go-ipfs/importer/helpers"
	"github.com/ipfs/go-ipfs/importer/trickle"
	"github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	lockfile "github.com/ipfs/go-ipfs/repo/fsrepo/lock"
	node "gx/ipfs/QmPN7cwmpcc4DWXb4KTB9dNAJgjuPY69h3npsMfhRrQL9c/go-ipld-format"
)

type IpfsApi interface {
	Add(r io.Reader) (string, error)
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
		Repo:   repo,
		Online: true,
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
			}
		}
	}()

	return (*IpfsCoreApi)(node), nil
}

func closeIpfs(node *core.IpfsNode, repoPath string) {
	repoLockFile := filepath.Join(repoPath, lockfile.LockFile)
	os.Remove(repoLockFile)
	node.Close()
}

func (ipfs *IpfsCoreApi) Add(r io.Reader) (string, error) {
	node := ipfs.node()
	return addAndPin(node.Context(), node, r)
}

func addAndPin(ctx context.Context, n *core.IpfsNode, r io.Reader) (string, error) {
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

	var node node.Node
	if fileAdder.Trickle {
		node, err = trickle.TrickleLayout(params.New(chnk))
		if err != nil {
			return "", err
		}
	} else {
		node, err = balanced.BalancedLayout(params.New(chnk))
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
