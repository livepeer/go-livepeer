package ipfs

import (
	"context"
	chunk "gx/ipfs/QmWo8jYc19ppG7YoTsrr2kEtLRbARTJho5oNXFTR6B7Peq/go-ipfs-chunker"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
	"io"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreunix"
	"github.com/ipfs/go-ipfs/importer/balanced"
	ihelper "github.com/ipfs/go-ipfs/importer/helpers"
	"github.com/ipfs/go-ipfs/importer/trickle"
	"github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
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
