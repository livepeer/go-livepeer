package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/api/client"
	"github.com/ethereum/go-ethereum/swarm/storage/feed"
	"github.com/ethereum/go-ethereum/swarm/storage/feed/lookup"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/lpms/ffmpeg"
)

type swarmFeedEntry struct {
	PlData string `json:"plData"`
	SegLoc string `json:"segLoc"`
	SeqNo  uint64 `json:"seqNo"`
}

func main() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flag.Set("logtostderr", "true")
	read := flag.Bool("read", false, "Reading the feed")
	write := flag.Bool("write", false, "Writing the feed")
	flag.Parse()

	//Get Signer of the Address
	// addr := common.HexToAddress("0x100fB8CD838ccc9351c7E7590c00Cc2eB331365C")
	// am, err := eth.NewAccountManager(addr, "/Users/ericxtang/Library/Ethereum/keystore/")
	addr := common.HexToAddress("0x06Dc3950B18431ED836C76c6832937A7bA010E61")
	am, err := eth.NewAccountManager(addr, "/Users/ericxtang/.lpData/rinkeby/keystore/")
	if err != nil {
		fmt.Printf("Cannot create account manager for addr: %v, %v", addr, am)
		return
	}
	signer, err := am.Signer()
	if err != nil {
		fmt.Printf("Cannot get signer: %v", err)
		return
	}
	manifestID := "993af9155677f739f7b733664e153ade366b4f563edb37eecb70496b5f16aba3"

	//Create the feed
	f := new(feed.Feed)
	f.User = addr
	f.Topic, _ = feed.NewTopic("ethnyc", nil)
	query := feed.NewQueryLatest(f, lookup.NoClue)

	c := client.NewClient("http://localhost:8500")
	feedStatus, err := c.GetFeedRequest(query, manifestID)
	if err != nil {
		fmt.Printf("Error retrieving feed status: %s", err.Error())
		return
	}
	fmt.Printf("Request: %v\n", feedStatus)
	if *read {
		readFeed(c, query, manifestID)
	} else if *write {
		writeFeed(c, signer)
	}
}

func readFeed(c *client.Client, query *feed.Query, manifestID string) {
	// read update
	for {
		// reader, err := c.QueryFeed(query, manifestID)
		reader, err := c.QueryFeed(query, "")
		if err != nil {
			fmt.Printf("Cannot query feed: %v", err)
			return
		}

		feed, err := ioutil.ReadAll(reader)
		if err != nil {
			fmt.Printf("Cannot read feed: %v", err)
			return
		}
		fmt.Printf("Feed: %s\n", feed)
	}
}

func writeFeed(c *client.Client, signer *feed.GenericSigner) {
	updates := [][]byte{
		[]byte("Weather looks bright and sunny today, we should merge this PR and go out enjoy"),
		[]byte("hi"),
	}

	// write update
	counter := 0
	f := new(feed.Feed)
	addr := common.HexToAddress("0x06Dc3950B18431ED836C76c6832937A7bA010E61")
	f.User = addr
	f.Topic, _ = feed.NewTopic("ethnyc", nil)
	query := feed.NewQueryLatest(f, lookup.NoClue)

	for count := 0; ; count++ {
		feedStatus, err := c.GetFeedRequest(query, "")
		if err != nil {
			fmt.Printf("Error retrieving feed status: %s", err.Error())
			return
		}
		// update := updates[counter]

		segLoc := fmt.Sprintf("https://os-test-rinkeby.s3.amazonaws.com/b6a99bbe/source/%v.ts", count)
		// feedEntry := fmt.Sprintf("{plData:\"%v\", segLoc:\"%v\", seqNo:\"%v\"}", ffmpeg.P720p30fps16x9.Name, , count)
		// fmt.Println("Inserting: ", feedEntry)
		// feedStatus.SetData([]byte(feedEntry))

		entry := swarmFeedEntry{PlData: ffmpeg.P720p30fps16x9.Name, SegLoc: segLoc, SeqNo: uint64(count)}
		entryB, err := json.Marshal(entry)
		if err != nil {
			glog.Infof("Error marshaling swarm feed entry: %v", err)
		}
		fmt.Printf("inserting entryB: %s\n", entryB)
		feedStatus.SetData(entryB)

		// sign update
		if err := feedStatus.Sign(signer); err != nil {
			utils.Fatalf("Error signing feed update: %s", err.Error())
		}

		// post update
		err = c.UpdateFeed(feedStatus)
		if err != nil {
			utils.Fatalf("Error updating feed: %s", err.Error())
		}

		if counter < len(updates) {
			counter++
		} else {
			counter = 0
		}
	}
}
