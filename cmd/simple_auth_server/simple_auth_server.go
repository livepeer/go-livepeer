package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/livepeer/go-livepeer/core"
)

type authWebhookReq struct {
	Url string `json:"url"`
}

type profile struct {
	Name    string `json:"name"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Bitrate int    `json:"bitrate"`
	FPS     uint   `json:"fps"`
	FPSDen  uint   `json:"fpsDen"`
	Profile string `json:"profile"`
	GOP     string `json:"gop"`
}

type authWebhookResponse struct {
	ManifestID string    `json:"manifestID"`
	Profiles   []profile `json:"profiles"`
}

func main() {
	http.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var req authWebhookReq
		err := decoder.Decode(&req)
		if err != nil {
			fmt.Printf("Error parsing URL: %v\n", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		var mid core.ManifestID
		u, err := url.Parse(req.Url)
		mid = parseStreamID(u.String()).ManifestID

		if mid == "" {
			mid = core.RandomManifestID()
			fmt.Printf("Generated random manifestID: %v\n", mid)
		} else if mid == "fizz" {
			mid = "buzz"
			fmt.Printf("Detected \"fizz\" as manifestID. Crazy! Renaming to \"buzz\".\n")
		}
		fmt.Printf("Stream started with manifestID: %v\n", mid)

		resp := authWebhookResponse{
			ManifestID: string(mid),
			Profiles: []profile{{
				Name:    "240p",
				Width:   426,
				Height:  240,
				Bitrate: 250000,
				FPS:     0,
			}},
		}
		byteSlice, _ := json.Marshal(resp)
		w.Write(byteSlice)
	})

	fmt.Println("Listening on localhost:8000/auth\nTry something crazy - stream with \"fizz\" as the manifestID.")
	err := http.ListenAndServe(":8000", nil) // set listen port
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

var StreamPrefix = regexp.MustCompile(`.*[ /](stream/)?`)

func cleanStreamPrefix(reqPath string) string {
	return StreamPrefix.ReplaceAllString(reqPath, "")
}

func parseStreamID(reqPath string) core.StreamID {
	// remove extension and create streamid
	p := strings.TrimSuffix(reqPath, path.Ext(reqPath))
	return core.SplitStreamIDString(cleanStreamPrefix(p))
}
