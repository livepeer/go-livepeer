package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/livepeer/go-livepeer/server"
	"github.com/stretchr/testify/require"
)

func TestTranscodeServer(t *testing.T) {
	// Start the server
	transcoder := NewTranscodingServer(8000)

	// Load media data
	mediaData, err := ioutil.ReadFile("../../samples/sample_1_360_15000.ts")
	require.NoError(t, err, "Unable to read sample media file")
	jsonData := compact(jobSpec)

	// Construct request
	req, _ := http.NewRequest("POST", "http://127.0.0.1:8000/live/testStreamName/0.ts", bytes.NewBuffer(mediaData))
	req.Header.Set("Content-Type", "video/mp2t")
	req.Header.Set("Accept", "multipart/mixed")
	req.Header.Set(server.LIVERPEER_TRANSCODE_CONFIG_HEADER, jsonData)

	// execute HTTP request
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "http.DefaultClient.Do")

	contentType := res.Header.Get("Content-Type")
	mediatype, params, err := mime.ParseMediaType(contentType)
	if mediatype == "text/plain" {
		// Got error
		body, _ := ioutil.ReadAll(res.Body)
		require.FailNow(t, string(body))
	}
	require.NoError(t, err, "mediatype=%s params=%v error=%v", mediatype, params, err)

	// Read multipart response
	mr := multipart.NewReader(res.Body, params["boundary"])
	var part *multipart.Part
	for part, err = mr.NextPart(); err == nil; part, err = mr.NextPart() {
		rendition, err := ioutil.ReadAll(part)
		require.NoError(t, err)
		require.Greater(t, len(rendition), 4000)
		fmt.Printf("Unpacked rendition %d headers=%v\n", len(rendition), part.Header)
		renditionName := part.Header.Get("Rendition-Name")
		// Store into a file
		err = ioutil.WriteFile(fmt.Sprintf("../../samples/%s.ts", renditionName), rendition, 0666)
		require.NoError(t, err)
	}

	transcoder.Stop()
}

// 3 renditions
var jobSpec string = `{
	"manifestID": "stream",
	"streamID": "unknown",
	"verificationFreq": 1,
	"profiles": [
		{
			"name": "outPAL",
			"width": 1024,
			"height": 576,
			"bitrate": 700000,
			"fps": 24,
			"colorDepth": 0,
			"chromaFormat": 0
		},
		{
			"name": "outWVGA",
			"width": 854,
			"height": 480,
			"bitrate": 500000,
			"fps": 24,
			"colorDepth": 0,
			"chromaFormat": 0
		},
		{
			"name": "outCGA",
			"width": 320,
			"height": 190,
			"bitrate": 300000,
			"fps": 24,
			"colorDepth": 0,
			"chromaFormat": 0
		}
	]
}`

// Used to prepare json for storing into header value
func compact(txt string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(txt, "\n", ""), "\r", ""), "\t", ""), " ", "")
}
