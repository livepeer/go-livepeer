package trickle

import (
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
)

// TrickleSubscriber represents a trickle streaming reader that always fetches from index -1
type TrickleSubscriber struct {
	url        string
	mu         sync.Mutex     // Mutex to manage concurrent access
	pendingGet *http.Response // Pre-initialized GET request
	idx        int            // Segment index to request

	// Number of errors from preconnect
	preconnectErrorCount int
}

// NewTrickleSubscriber creates a new trickle stream reader for GET requests
func NewTrickleSubscriber(url string) *TrickleSubscriber {
	// No preconnect needed here; it will be handled by the first Read call.
	return &TrickleSubscriber{
		url: url,
		idx: -1, // shortcut for 'latest'
	}
}

func GetIndex(resp *http.Response) int {
	if resp == nil {
		return -1 // TODO hmm
	}
	v := resp.Header.Get("Lp-Trickle-Seq")
	i, err := strconv.Atoi(v)
	if err != nil {
		// Fetch the latest index
		// TODO think through whether this is desirable
		return -1
	}
	return i
}

// preconnect pre-initializes the next GET request for fetching the next segment (always index -1)
func (c *TrickleSubscriber) preconnect() (*http.Response, error) {
	url := fmt.Sprintf("%s/%d", c.url, c.idx)
	slog.Info("preconnecting", "url", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Failed to create request for segment: %v\n", err)
		return nil, err
	}

	// Execute the GET request
	resp, err := (&http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to complete GET for next segment: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close() // Ensure we close the body to avoid leaking connections
		return nil, fmt.Errorf("failed GET segment, status code: %d, msg: %s", resp.StatusCode, string(body))
	}

	// Return the pre-initialized GET request
	return resp, nil
}

// Read retrieves data from the current segment and sets up the next segment concurrently.
// It returns the reader for the current segment's data.
func (c *TrickleSubscriber) Read() (*http.Response, error) {
	// Acquire lock to manage access to pendingGet
	c.mu.Lock()

	// TODO clean up this preconnect error handling!
	hitMaxPreconnects := c.preconnectErrorCount > 5
	if hitMaxPreconnects {
		slog.Error("Hit max preconnect error", "url", c.url, "idx", c.idx)
		c.mu.Unlock()
		return nil, fmt.Errorf("Hit max preconnects")
	}

	// Get the reader to use for the current segment
	conn := c.pendingGet
	if conn == nil {
		// Preconnect if we don't have a pending GET
		slog.Info("No preconnect, connecting", "url", c.url, "idx", c.idx)
		p, err := c.preconnect()
		if err != nil {
			c.preconnectErrorCount++
			c.mu.Unlock()
			return nil, err
		}
		conn = p
		// reset preconnect error
		c.preconnectErrorCount = 0
	}

	// Set to use the next index for the next (pre-)connection
	idx := GetIndex(conn)
	if idx != -1 {
		c.idx = idx + 1
	}

	// Set up the next connection
	go func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		nextConn, err := c.preconnect()
		if err != nil {
			slog.Error("failed to preconnect next segment", "idx", c.idx, "err", err)
			c.preconnectErrorCount++
			return
		}

		c.pendingGet = nextConn
		idx := GetIndex(conn)
		if idx != -1 {
			c.idx = idx + 1
		}
		// reset preconnect error
		c.preconnectErrorCount = 0
	}()

	// Now unlock since the next segment is set up and we have the reader for the current one
	c.mu.Unlock()

	// Return the reader for the current segment
	return conn, nil
}
