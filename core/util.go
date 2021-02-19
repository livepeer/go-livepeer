package core

import (
	"io"
	"net/http"
	"os"
)

var hostName = "undefinde"

func init() {
	hostName, _ = os.Hostname()
}

// NewPostRequest creates POST HTTP request with User Agent header set to
// Livepeer version
func NewPostRequest(url, contentType string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Add("User-Agent", "Livepeer/"+LivepeerVersion)
	req.Header.Add("Livepeer-Host", hostName)
	return req, nil
}
