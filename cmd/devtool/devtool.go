package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/cmd/devtool/devtool"
)

var (
	serviceHost = "127.0.0.1"
	cliPort     = 7935
	mediaPort   = 8935
	rtmpPort    = 1935
)

func main() {
	flag.Set("logtostderr", "true")
	baseDataDir := flag.String("datadir", ".lpdev2", "default data directory")
	endpointAddr := flag.String("endpoint", "", "Geth endpoint to connect to")
	miningAccountFlag := flag.String("miningaccount", "", "Override geth mining account (usually not needed)")
	ethControllerFlag := flag.String("controller", "", "Override controller address (usually not needed)")
	svcHost := flag.String("svchost", "127.0.0.1", "default service host")

	flag.Parse()

	cfg := devtool.NewDevtoolConfig()
	if *endpointAddr != "" {
		cfg.Endpoint = *endpointAddr
	}
	if *miningAccountFlag != "" {
		cfg.GethMiningAccount = *miningAccountFlag
		cfg.GethMiningAccountOverride = true
	}
	if *ethControllerFlag != "" {
		cfg.EthController = *ethControllerFlag
		cfg.EthControllerOverride = true
	}
	if *svcHost != "" {
		serviceHost = *svcHost
		cfg.ServiceURI = fmt.Sprintf("https://%s:", serviceHost)
	}
	args := flag.Args()
	goodToGo := false
	isBroadcaster := true
	if len(args) > 1 && args[0] == "setup" {
		switch args[1] {
		case "broadcaster":
			goodToGo = true
		case "transcoder":
			isBroadcaster = false
			goodToGo = true
		}
	}
	if !goodToGo {
		fmt.Println(`
    Usage: go run cmd/devtool/devtool.go setup broadcaster|transcoder [nodeIndex]
        It will create initialize eth account (on private testnet) to be used for broadcaster or transcoder
        and will create shell script (run_broadcaster_ETHACC.sh or run_transcoder_ETHACC.sh) to run it.
        Node index indicates how much to offset node's port. Orchestrator node's index by default is 1.
        For example:
        "devtool setup broadcaster" will create broadcaster with cli port 7935 and media port 8935
        "devtool setup broadcaster 2" will create broadcaster with cli port 7937 and media port 8937
        "devtool setup transcoder 3" will create transcoder with cli port 7938 and media port 8938`)
		return
	}
	nodeIndex := 0
	if args[1] == "transcoder" {
		nodeIndex = 1
	}
	if len(args) > 2 {
		if i, err := strconv.ParseInt(args[2], 10, 64); err == nil {
			nodeIndex = int(i)
		}
	}
	cfg.ServiceURI += strconv.Itoa(mediaPort + nodeIndex)
	mediaPort += nodeIndex
	cliPort += nodeIndex * 2 // Because we need different CLI ports for the Orchestrator and Transcoder scripts
	rtmpPort += nodeIndex

	t := getNodeType(isBroadcaster)

	tmp, err := ioutil.TempDir("", "livepeer")
	if err != nil {
		glog.Exitf("Can't create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmp)

	tempKeystoreDir := filepath.Join(tmp, "keystore")
	acc := devtool.CreateKey(tempKeystoreDir)
	cfg.Account = acc
	glog.Infof("Using account %s", acc)
	glog.Infof("Using svchost %s", serviceHost)
	dataDir := filepath.Join(*baseDataDir, t+"_"+acc)
	err = os.MkdirAll(dataDir, 0755)
	if err != nil {
		glog.Exitf("Can't create directory %v", err)
	}

	keystoreDir := filepath.Join(dataDir, "keystore")
	err = moveDir(tempKeystoreDir, keystoreDir)
	if err != nil {
		glog.Exit(err)
	}
	cfg.KeystoreDir = keystoreDir
	cfg.IsBroadcaster = isBroadcaster
	devtool, err := devtool.Init(cfg)
	if err != nil {
		glog.Error(err)
	}
	defer devtool.Close()

	if cfg.IsBroadcaster {
		err = devtool.FundBroadcaster()
	} else {
		err = devtool.InitializeOrchestrator(cfg)
	}
	if err != nil {
		glog.Error(err)
	}
	createRunScript(devtool.EthController, dataDir, serviceHost, cfg)
	if !isBroadcaster {
		cliPort++
		tDataDir := filepath.Join(*baseDataDir, "transcoder_"+acc)
		err = os.MkdirAll(tDataDir, 0755)
		if err != nil {
			glog.Exitf("Can't create directory %v", err)
		}
		createTranscoderRunScript(acc, tDataDir, serviceHost)
	}
	glog.Info("Finished")
}

func getNodeType(isBroadcaster bool) string {
	t := "broadcaster"
	if !isBroadcaster {
		t = "orchestrator"
	}
	return t
}

func createTranscoderRunScript(ethAcctAddr, dataDir, serviceHost string) {
	args := []string{
		"./livepeer",
		"-v 99",
		fmt.Sprintf("-dataDir ./%s", dataDir),
		"-orchSecret secre",
		fmt.Sprintf("-orchAddr %s:%d", serviceHost, mediaPort),
		fmt.Sprintf("-cliAddr %s:%d", serviceHost, cliPort),
		"-transcoder",
	}
	fName := fmt.Sprintf("run_transcoder_%s.sh", ethAcctAddr)
	writeScript(fName, args...)
}

func createRunScript(ethController string, dataDir, serviceHost string, cfg devtool.DevtoolConfig) {
	args := []string{
		"./livepeer",
		"-v 99",
		"-ethController " + ethController,
		"-dataDir ./" + dataDir,
		"-ethAcctAddr " + cfg.Account,
		"-ethUrl " + cfg.Endpoint,
		"-ethPassword \"\"",
		"-network=devenv",
		"-blockPollingInterval 1",
		"-monitor=false",
		"-currentManifest=true",
		fmt.Sprintf("-cliAddr %s:%d", serviceHost, cliPort),
		fmt.Sprintf("-httpAddr %s:%d", serviceHost, mediaPort),
	}

	if !cfg.IsBroadcaster {
		args = append(
			args,
			"-initializeRound=true",
			fmt.Sprintf("-serviceAddr=%s:%d", serviceHost, mediaPort),
			"-orchestrator=true",
			"-orchSecret secre",
			"-pricePerUnit 1",
		)

		fName := fmt.Sprintf("run_%s_standalone_%s.sh", getNodeType(cfg.IsBroadcaster), cfg.Account)
		writeScript(fName, args...)

		args = append(args, "-transcoder=true")
		fName = fmt.Sprintf("run_%s_with_transcoder_%s.sh", getNodeType(cfg.IsBroadcaster), cfg.Account)
		writeScript(fName, args...)
	} else {
		args = append(
			args,
			"-gateway=true",
			fmt.Sprintf("-rtmpAddr %s:%d", serviceHost, rtmpPort),
		)

		fName := fmt.Sprintf("run_%s_%s.sh", getNodeType(cfg.IsBroadcaster), cfg.Account)
		writeScript(fName, args...)
	}
}

func writeScript(fName string, args ...string) {
	script := "#!/bin/bash\n"

	script += strings.Join(args, " \\\n\t")
	script += "\n"

	glog.Info(script)
	err := ioutil.WriteFile(fName, []byte(script), 0755)
	if err != nil {
		glog.Warningf("Error writing run script %q: %v", fName, err)
	}
}

func moveDir(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	originalMode := info.Mode()

	if err := os.MkdirAll(dst, os.FileMode(0755)); err != nil {
		return err
	}
	defer os.Chmod(dst, originalMode)

	contents, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, content := range contents {
		cs, cd := filepath.Join(src, content.Name()), filepath.Join(dst, content.Name())
		if err := moveFile(cs, cd, content); err != nil {
			return err
		}
	}

	err = os.Remove(src)
	if err != nil {
		return err
	}

	return nil
}

func moveFile(src, dst string, info os.FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
		return err
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	if err = os.Chmod(f.Name(), info.Mode()); err != nil {
		return err
	}

	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	_, err = io.Copy(f, s)
	if err != nil {
		return err
	}

	err = os.Remove(src)
	if err != nil {
		return err
	}

	return nil
}
