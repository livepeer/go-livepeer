package ipfs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/golang/glog"
)

func ConfigIpfsPath(ipfsPath string) error {
	_, err := os.Stat(ipfsPath)
	if err != nil {
		glog.Infof("IPFS config file does not exist. Initializing IPFS at %v", ipfsPath)

		cmd := exec.Command("ipfs", "init")
		cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%v", ipfsPath))
		err := cmd.Start()
		if err != nil {
			return err
		}
		if err := cmd.Wait(); err != nil {
			return err
		}
	} else {
		glog.Infof("IPFS config file already exists at %v", ipfsPath)
	}

	return nil
}

func ConfigIpfsGateway(ipfsPath string, gatewayPort int) error {
	cmd := exec.Command("ipfs", "config", "Addresses.Gateway", fmt.Sprintf("/ip4/127.0.0.1/tcp/%v", gatewayPort))
	cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%v", ipfsPath))
	err := cmd.Start()
	if err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}

	glog.Infof("Set IPFS gateway port to %v", gatewayPort)

	return nil
}

func ConfigIpfsApi(ipfsPath string, apiPort int) error {
	cmd := exec.Command("ipfs", "config", "Addresses.API", fmt.Sprintf("/ip4/127.0.0.1/tcp/%v", apiPort))
	cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%v", ipfsPath))
	err := cmd.Start()
	if err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}

	glog.Infof("Set IPFS API port to %v", apiPort)

	return nil
}

func StartIpfsDaemon(ctx context.Context, ipfsPath string) error {
	cmd := exec.Command("ipfs", "daemon")
	var stderr bytes.Buffer
	var out bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &out
	cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%v", ipfsPath))
	err := cmd.Start()
	if err != nil {
		return err
	}

	glog.Infof("Started IPFS daemon")

	if err := cmd.Wait(); err != nil {
		// glog.Infof("%v", stderr)
		glog.Errorf("Error: %v - %v", err, out)
		return err
	}

	select {
	case <-ctx.Done():
		if err := cmd.Process.Kill(); err != nil {
			glog.Errorf("Error killing IPFS daemon: %v", err)
		}

		glog.Infof("Exited IPFS daemon")

		return ctx.Err()
	}
}
