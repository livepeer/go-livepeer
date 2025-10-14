/*
Livepeer is a peer-to-peer global video live streaming network.  The Golp project is a go implementation of the Livepeer protocol.  For more information, visit the project wiki.
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/livepeer/go-livepeer/cmd/livepeer/starter"
	"github.com/livepeer/livepeer-data/pkg/mistconnector"
	"github.com/peterbourgon/ff/v3"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/core"
)

const shutdownTimeout = 10 * time.Second

func main() {
	// Override the default flag set since there are dependencies that
	// incorrectly add their own flags (specifically, due to the 'testing'
	// package being linked)
	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	//We preserve this flag before resetting all the flags.  Not a scalable approach, but it'll do for now.  More discussions here - https://github.com/livepeer/go-livepeer/pull/617
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flag.CommandLine.SetOutput(os.Stdout)

	// Help & Log
	mistJSON := flag.Bool("j", false, "Print application info as json")
	version := flag.Bool("version", false, "Print out the version")
	verbosity := flag.String("v", "3", "Log verbosity.  {4|5|6}")

	cfg := starter.NewLivepeerConfig(flag.CommandLine)

	// Config file
	_ = flag.String("config", "", "Config file in the format 'key value', flags and env vars take precedence over the config file")
	err := ff.Parse(flag.CommandLine, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithEnvVarPrefix("LP"),
		ff.WithConfigFileParser(ff.PlainParser),
	)
	if err != nil {
		glog.Exit("Error parsing config: ", err)
	}

	vFlag.Value.Set(*verbosity)

	if *mistJSON {
		mistconnector.PrintMistConfigJson(
			"livepeer",
			"Official implementation of the Livepeer video processing protocol. Can play all roles in the network.",
			"Livepeer",
			core.LivepeerVersion,
			flag.CommandLine,
		)
		return
	}

	cfg = starter.UpdateNilsForUnsetFlags(cfg)
	cfg.PrintConfig(os.Stdout)

	if *version {
		fmt.Println("Livepeer Node Version: " + core.LivepeerVersion)
		fmt.Printf("Golang runtime version: %s %s\n", runtime.Compiler, runtime.Version())
		fmt.Printf("Architecture: %s\n", runtime.GOARCH)
		fmt.Printf("Operating system: %s\n", runtime.GOOS)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	lc := make(chan struct{})

	go func() {
		defer close(lc)
		starter.StartLivepeer(ctx, cfg)
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-c:
		glog.Infof("Exiting Livepeer: %v", sig)
		cancel()
	case <-lc:
		// fallthrough to normal shutdown below
	}
	select {
	case <-lc:
		glog.Infof("Graceful shutdown complete")
	case <-time.After(shutdownTimeout):
		glog.Infof("Shutdown timed out, forcing exit")
		os.Exit(1)
	}
}
