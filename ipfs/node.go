package ipfs

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreunix"
	"github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	lockfile "github.com/ipfs/go-ipfs/repo/fsrepo/lock"
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
	return coreunix.Add(node, r)
}

func (ipfs *IpfsCoreApi) node() *core.IpfsNode {
	return (*core.IpfsNode)(ipfs)
}
