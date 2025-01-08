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

	// needed to help with reconnects
	written bool
	client  *TricklePublisher
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
			DisableKeepAlives: true,
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
		client: c,
	}, nil
}

func (c *TricklePublisher) Close() error {
	req, err := http.NewRequest("DELETE", c.baseURL, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Transport: &http.Transport{
		DisableKeepAlives: true,
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

func (c *TricklePublisher) Next() (*pendingPost, error) {
	// Acquire lock to manage access to pendingPost and index
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	// Get the writer to use
	pp := c.pendingPost
	if pp == nil {
		p, err := c.preconnect()
		if err != nil {
			c.writeLock.Unlock()
			return nil, err
		}
		pp = p
	}

	// Set up the next connection
	nextPost, err := c.preconnect()
	if err != nil {
		c.writeLock.Unlock()
		return nil, err
	}
	c.pendingPost = nextPost

	return pp, nil
}

func (p *pendingPost) reconnect() (*pendingPost, error) {
	// This is a little gnarly but works for now:
	// Set the publisher's sequence sequence to the intended reconnect
	// Call publisher's preconnect (which increments its sequence)
	// then reset publisher's sequence back to the original
	//slog.Info("Re-connecting", "url", p.client.baseURL, "seq", p.client.index)
	p.client.writeLock.Lock()
	defer p.client.writeLock.Unlock()
	currentSeq := p.client.index
	p.client.index = p.index
	pp, err := p.client.preconnect()
	p.client.index = currentSeq
	return pp, err
}

func (p *pendingPost) Write(data io.Reader) (int64, error) {

	// If writing multiple times, reconnect
	if p.written {
		pp, err := p.reconnect()
		if err != nil {
			return 0, err
		}
		p = pp
	}

	var (
		writer = p.writer
		index  = p.index
		errCh  = p.errCh
	)

	// Mark as written
	p.written = true

	// before writing, check for error from preconnects
	select {
	case err := <-errCh:
		return 0, err
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
		return n, err
	}

	if ioError != nil {
		return n, fmt.Errorf("error streaming data to segment %d: %w", index, ioError)
	}

	if closeErr != nil {
		return n, fmt.Errorf("error closing writer for segment %d: %w", index, closeErr)
	}

	return n, nil
}

/*
Close a segment. This is a polite action to notify any
subscribers that might be waiting for this segment.

Only needed if the segment is dropped or otherwise errored;
not required if the segment is written normally.

Note that subscribers still work fine even without this call;
it would just take longer for them to stop waiting when
the current segment drops out of the window of active segments.
*/
func (p *pendingPost) Close() error {
	p.writer.Close()
	url := fmt.Sprintf("%s/%d", p.client.baseURL, p.index)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Transport: &http.Transport{
		DisableKeepAlives: true,
		// ignore orch certs for now
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &HTTPError{Code: resp.StatusCode, Body: string(body)}
	}
	return nil
}

// Write sends data to the current segment, sets up the next segment concurrently, and blocks until completion
func (c *TricklePublisher) Write(data io.Reader) error {
	pp, err := c.Next()
	if err != nil {
		return err
	}
	_, err = pp.Write(data)
	return err
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
