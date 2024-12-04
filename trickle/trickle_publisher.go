package trickle

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
)

var StreamNotFoundErr = errors.New("stream not found")

// TricklePublisher represents a trickle streaming client
type TricklePublisher struct {
	baseURL     string
	index       int          // Current index for segments
	writeLock   sync.Mutex   // Mutex to manage concurrent access
	pendingPost *pendingPost // Pre-initialized POST request
	contentType string
}

// HTTPError gets returned with a >=400 status code (non-400)
type HTTPError struct {
	Code int
	Body string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("Status code %d - %s", e.Code, e.Body)
}

// pendingPost represents a pre-initialized POST request waiting for data
type pendingPost struct {
	index  int
	writer *io.PipeWriter
	errCh  chan error
}

// NewTricklePublisher creates a new trickle stream client
func NewTricklePublisher(url string) (*TricklePublisher, error) {
	c := &TricklePublisher{
		baseURL:     url,
		contentType: "video/MP2T",
	}
	p, err := c.preconnect()
	if err != nil {
		return nil, err
	}
	c.pendingPost = p

	return c, nil
}

// Acquire lock to manage access to pendingPost and index
// NB expects to have the lock already since we mutate the index
func (c *TricklePublisher) preconnect() (*pendingPost, error) {

	index := c.index
	url := fmt.Sprintf("%s/%d", c.baseURL, index)

	slog.Debug("Preconnecting", "url", url)

	errCh := make(chan error, 1)
	pr, pw := io.Pipe()
	req, err := http.NewRequest("POST", url, pr)
	if err != nil {
		slog.Error("Failed to create request for segment", "url", url, "err", err)
		return nil, err
	}
	req.Header.Set("Content-Type", c.contentType)

	// Start the POST request in a background goroutine
	go func() {
		// Createa new client to prevent connection reuse
		client := http.Client{Transport: &http.Transport{
			// ignore orch certs for now
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}}
		resp, err := client.Do(req)
		if err != nil {
			slog.Error("Failed to complete POST for segment", "url", url, "err", err)
			errCh <- err
			return
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading body", "url", url, "err", err)
			errCh <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			slog.Error("Failed POST segment", "url", url, "status_code", resp.StatusCode, "msg", string(body))
			if resp.StatusCode == http.StatusNotFound {
				errCh <- StreamNotFoundErr
				return
			}
			if resp.StatusCode >= 400 {
				errCh <- &HTTPError{Code: resp.StatusCode, Body: string(body)}
				return
			}
		} else {
			slog.Debug("Uploaded segment", "url", url)
		}
		errCh <- nil
	}()

	c.index += 1
	return &pendingPost{
		writer: pw,
		index:  index,
		errCh:  errCh,
	}, nil
}

func (c *TricklePublisher) Close() error {
	req, err := http.NewRequest("DELETE", c.baseURL, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Transport: &http.Transport{
		// ignore orch certs for now
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Failed to delete stream: %v - %s", resp.Status, string(body))
	}
	return nil
}

// Write sends data to the current segment, sets up the next segment concurrently, and blocks until completion
func (c *TricklePublisher) Write(data io.Reader) error {

	// Acquire lock to manage access to pendingPost and index
	c.writeLock.Lock()

	// Get the writer to use
	pp := c.pendingPost
	if pp == nil {
		p, err := c.preconnect()
		if err != nil {
			c.writeLock.Unlock()
			return err
		}
		pp = p
	}
	writer := pp.writer
	index := pp.index
	errCh := pp.errCh

	// Set up the next connection
	nextPost, err := c.preconnect()
	if err != nil {
		c.writeLock.Unlock()
		return err
	}
	c.pendingPost = nextPost

	// Now unlock so the copy does not block
	c.writeLock.Unlock()

	// before writing, check for error from preconnects
	select {
	case err := <-errCh:
		return err
	default:
		// no error, continue
	}

	// Start streaming data to the current POST request
	n, ioError := io.Copy(writer, data)

	// if no io errors, close the writer
	var closeErr error
	if ioError == nil {
		slog.Debug("Completed writing", "idx", index, "totalBytes", humanBytes(n))

		// Close the pipe writer to signal end of data for the current POST request
		closeErr = writer.Close()
	}

	// check for errors after write, eg >=400 status codes
	// these typically do not result in io errors eg, with io.Copy
	// also prioritize errors over this channel compared to io errors
	// such as "read/write on closed pipe"
	if err := <-errCh; err != nil {
		return err
	}

	if ioError != nil {
		return fmt.Errorf("error streaming data to segment %d: %w", index, err)
	}

	if closeErr != nil {
		return fmt.Errorf("error closing writer for segment %d: %w", index, err)
	}

	return nil
}

func humanBytes(bytes int64) string {
	var unit int64 = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
