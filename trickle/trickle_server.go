package trickle

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const CHANGEFEED = "_changes"

type TrickleServerConfig struct {
	// Base HTTP path for the server
	BasePath string

	// HTTP mux to use
	Mux *http.ServeMux

	// Whether to enable the changefeed (default false)
	Changefeed bool

	// Whether to auto-create channels on first publish (default false)
	Autocreate bool

	// Amount of time a channel has no new segments before being swept (default 5 minutes)
	IdleTimeout time.Duration

	// How often to sweep for idle channels (default 1 minute)
	SweepInterval time.Duration
}

type Server struct {
	mutex   sync.RWMutex
	streams map[string]*Stream

	config TrickleServerConfig

	// for internal channels
	internalPub *TrickleLocalPublisher
}

type Stream struct {
	mutex       sync.RWMutex
	segments    []*Segment
	latestWrite int
	name        string
	mimeType    string
	writeTime   time.Time
	closed      bool
}

type Segment struct {
	idx    int
	mutex  *sync.Mutex
	cond   *sync.Cond
	buffer *bytes.Buffer
	closed bool
}

type SegmentSubscriber struct {
	segment *Segment
	readPos int
}

type Changefeed struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

const maxSegmentsPerStream = 5

var FirstByteTimeout = errors.New("pending read timeout")

func applyDefaults(config *TrickleServerConfig) {
	if config.BasePath == "" {
		config.BasePath = "/"
	}
	if config.Mux == nil {
		config.Mux = http.DefaultServeMux
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 5 * time.Minute
	}
	if config.SweepInterval == 0 {
		config.SweepInterval = time.Minute
	}
}

func ConfigureServer(config TrickleServerConfig) *Server {
	streamManager := &Server{
		streams: make(map[string]*Stream),
		config:  config,
	}

	// set up changefeed
	if config.Changefeed {
		streamManager.internalPub = NewLocalPublisher(streamManager, CHANGEFEED, "application/json")
		streamManager.internalPub.CreateChannel()
	}

	applyDefaults(&streamManager.config)
	var (
		mux      = streamManager.config.Mux
		basePath = streamManager.config.BasePath
	)

	mux.HandleFunc("POST "+basePath+"{streamName}", streamManager.handleCreate)
	mux.HandleFunc("GET "+basePath+"{streamName}/{idx}", streamManager.handleGet)
	mux.HandleFunc("POST "+basePath+"{streamName}/{idx}", streamManager.handlePost)
	mux.HandleFunc("DELETE "+basePath+"{streamName}/{idx}", streamManager.closeSeq)
	mux.HandleFunc("DELETE "+basePath+"{streamName}", streamManager.handleDelete)
	return streamManager
}

func (sm *Server) Start() func() {
	ticker := time.NewTicker(sm.config.SweepInterval)
	done := make(chan bool)
	stop := func() {
		ticker.Stop()
		done <- true
	}
	go func() {
		for {
			select {
			case <-ticker.C:
				sm.sweepIdleChannels()
			case <-done:
				sm.clearAllStreams()
				return
			}
		}
	}()
	return stop
}

func (sm *Server) getStream(streamName string) (*Stream, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	stream, exists := sm.streams[streamName]
	return stream, exists
}

func (sm *Server) getOrCreateStream(streamName, mimeType string, isLocal bool) *Stream {
	sm.mutex.Lock()

	stream, exists := sm.streams[streamName]
	if !exists && (isLocal || sm.config.Autocreate) {
		stream = &Stream{
			segments:  make([]*Segment, maxSegmentsPerStream),
			name:      streamName,
			mimeType:  mimeType,
			writeTime: time.Now(),
		}
		sm.streams[streamName] = stream
		slog.Info("Creating stream", "stream", streamName)
	}
	sm.mutex.Unlock()

	// didn't exist and wasn't autocreated
	if stream == nil {
		return nil
	}

	// update changefeed
	if !exists && sm.config.Changefeed {
		jb, _ := json.Marshal(&Changefeed{
			Added: []string{streamName},
		})
		sm.internalPub.Write(bytes.NewReader(jb))
	}
	return stream
}

func (sm *Server) clearAllStreams() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// TODO update changefeed

	for _, stream := range sm.streams {
		stream.close()
	}
	sm.streams = make(map[string]*Stream)
}

func (sm *Server) sweepIdleChannels() {
	sm.mutex.Lock()
	streams := slices.Collect(maps.Values(sm.streams))
	sm.mutex.Unlock()
	now := time.Now()
	for _, s := range streams {
		// skip internal channels for now, eg changefeed
		if strings.HasPrefix(s.name, "_") {
			continue
		}
		s.mutex.Lock()
		writeTime := s.writeTime
		s.mutex.Unlock()
		if now.Sub(writeTime) > sm.config.IdleTimeout {
			if err := sm.closeStream(s.name); err != nil {
				slog.Warn("Could not close idle channel", "channel", s.name, "err", err)
			} else {
				slog.Info("Closed idle channel", "channel", s.name)
			}
		}
	}
}

func (s *Stream) close() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, segment := range s.segments {
		segment.close()
	}
	s.segments = make([]*Segment, maxSegmentsPerStream)
	s.closed = true
}

func (sm *Server) closeStream(streamName string) error {
	stream, exists := sm.getStream(streamName)
	if !exists {
		return errors.New("Invalid stream")
	}

	// TODO there is a bit of an issue around session reuse

	stream.close()
	sm.mutex.Lock()
	delete(sm.streams, streamName)
	sm.mutex.Unlock()
	slog.Info("Deleted stream", "streamName", streamName)

	// update changefeed if needed
	if !sm.config.Changefeed {
		return nil
	}
	jb, err := json.Marshal(&Changefeed{
		Removed: []string{streamName},
	})
	if err != nil {
		return err
	}
	sm.internalPub.Write(bytes.NewReader(jb))
	return nil
}

func (sm *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	streamName := r.PathValue("streamName")
	if err := sm.closeStream(streamName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (sm *Server) closeSeq(w http.ResponseWriter, r *http.Request) {
	s, exists := sm.getStream(r.PathValue("streamName"))
	if !exists {
		http.Error(w, "Stream not found", http.StatusNotFound)
		return
	}
	idx, err := strconv.Atoi(r.PathValue("idx"))
	if err != nil {
		http.Error(w, "Invalid idx", http.StatusBadRequest)
		return
	}
	slog.Info("DELETE closing seq", "channel", s.name, "seq", idx)
	s.mutex.RLock()
	seg := s.segments[idx%maxSegmentsPerStream]
	s.mutex.RUnlock()
	if seg == nil || seg.idx != idx {
		http.Error(w, "Nonexistent segment", http.StatusBadRequest)
		return
	}
	seg.close()
}

func (sm *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	stream := sm.getOrCreateStream(r.PathValue("streamName"), r.Header.Get("Expect-Content"), false)
	if stream == nil {
		http.Error(w, "Stream not found", http.StatusNotFound)
		return
	}
}

func (sm *Server) handlePost(w http.ResponseWriter, r *http.Request) {
	stream := sm.getOrCreateStream(r.PathValue("streamName"), r.Header.Get("Content-Type"), false)
	if stream == nil {
		http.Error(w, "Stream not found", http.StatusNotFound)
		return
	}
	idx, err := strconv.Atoi(r.PathValue("idx"))
	if err != nil {
		http.Error(w, "Invalid idx", http.StatusBadRequest)
		return
	}
	stream.handlePost(w, r, idx)
}

type timeoutReader struct {
	body          io.Reader
	timeout       time.Duration
	firstByteRead bool
	readStarted   bool
	doneCh        chan int
	errCh         chan error
}

func (tr *timeoutReader) startRead(p []byte) {
	go func() {
		n, err := tr.body.Read(p)
		if err != nil {
			tr.errCh <- err
			return
		}
		tr.doneCh <- n
	}()
}

func (tr *timeoutReader) Read(p []byte) (int, error) {
	if tr.firstByteRead {
		// After the first byte is read, proceed normally
		return tr.body.Read(p)
	}

	// we only want to start the reader once
	if !tr.readStarted {
		tr.errCh = make(chan error, 1)
		tr.doneCh = make(chan int, 1)
		tr.readStarted = true
		go tr.startRead(p)
	}

	select {
	case err := <-tr.errCh:
		return 0, err
	case n := <-tr.doneCh:
		if n > 0 {
			tr.firstByteRead = true
		}
		return n, nil
	case <-time.After(tr.timeout):
		return 0, FirstByteTimeout
	}
}

// Handle post requests for a given index
func (s *Stream) handlePost(w http.ResponseWriter, r *http.Request, idx int) {
	segment, exists := s.getForWrite(idx)
	if exists {
		slog.Warn("Overwriting existing entry", "idx", idx)
		// Overwrite anything that exists now. TODO figure out a safer behavior?
		// TODO fix concurrent writes to the same segment; would be very bad
		segment.buffer.Reset()
		segment.closed = false
	}

	// Wrap the request body with the custom timeoutReader so we can send
	// provisional headers (keepalives) until receiving the first byte
	reader := &timeoutReader{
		body: r.Body,
		// This can't be too short for now but ideally it'd be like 1 second
		// https://github.com/golang/go/issues/65035
		timeout: 10 * time.Second,
	}
	defer r.Body.Close()

	buf := make([]byte, 1024*32) // 32kb to begin with
	totalRead := 0
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			segment.writeData(buf[:n])
			if n == len(buf) && n < 1024*1024 { // 1 MB max
				// filled the buffer, so double it for efficiency
				buf = make([]byte, len(buf)*2)
			}
			totalRead += n
		}
		if err != nil {
			if err == FirstByteTimeout {
				// Keepalive via provisional headers
				slog.Info("Sending provisional headers for", "stream", s.name, "idx", idx)
				w.WriteHeader(http.StatusContinue)
				continue
			} else if err == io.EOF {
				break
			}
			slog.Info("Error reading POST body", "stream", s.name, "idx", idx, "bytes written", totalRead, "err", err)
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
	}

	// Mark segment as closed
	segment.close()
}

func (s *Stream) getForWrite(idx int) (*Segment, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if idx == -1 {
		idx = s.latestWrite
	} else {
		s.latestWrite = idx
	}
	s.writeTime = time.Now()
	slog.Info("POST segment", "stream", s.name, "idx", idx, "latest", s.latestWrite)
	segmentPos := idx % maxSegmentsPerStream
	if segment := s.segments[segmentPos]; segment != nil {
		if idx == segment.idx {
			return segment, !segment.isFresh()
		}
		// something exists here but its not the expected segment
		// probably an old segment so overwrite it
		segment.close()
	}
	segment := newSegment(idx)
	s.segments[segmentPos] = segment
	return segment, false
}

func (s *Stream) getForRead(idx int) (*Segment, int, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	exists := func(seg *Segment, i int) bool {
		return seg != nil && seg.idx == i
	}
	if idx == -1 {
		idx = s.latestWrite
	}
	segmentPos := idx % maxSegmentsPerStream
	segment := s.segments[segmentPos]
	if !exists(segment, idx) && (idx == s.latestWrite+1 || (idx == 0 && s.latestWrite == 0)) {
		// read request is just a little bit ahead of write head
		segment = newSegment(idx)
		s.segments[segmentPos] = segment
		slog.Info("GET precreating", "stream", s.name, "idx", idx, "latest", s.latestWrite)
	}
	slog.Info("GET segment", "stream", s.name, "idx", idx, "latest", s.latestWrite, "exists?", exists(segment, idx))
	return segment, s.latestWrite, exists(segment, idx)
}

func (sm *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	stream, exists := sm.getStream(r.PathValue("streamName"))
	if !exists {
		http.Error(w, "Stream not found", http.StatusNotFound)
		return
	}
	idx, err := strconv.Atoi(r.PathValue("idx"))
	if err != nil {
		http.Error(w, "Invalid idx", http.StatusBadRequest)
		return
	}
	stream.handleGet(w, r, idx)
}

func (s *Stream) handleGet(w http.ResponseWriter, r *http.Request, idx int) {
	segment, latestSeq, exists := s.getForRead(idx)
	if !exists {
		// Special status to indicate "stream exists but segment doesn't"
		w.Header().Set("Lp-Trickle-Latest", strconv.Itoa(latestSeq))
		w.Header().Set("Lp-Trickle-Seq", strconv.Itoa(idx))
		w.WriteHeader(470)
		w.Write([]byte("Entry not found"))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	subscriber := &SegmentSubscriber{
		segment: segment,
	}

	// Function to write data to the client
	sendData := func() (int, error) {
		totalWrites := 0
		for {
			// Check if client disconnected
			select {
			case <-r.Context().Done():
				return totalWrites, fmt.Errorf("client disconnected")
			default:
			}

			data, eof := subscriber.readData()
			if len(data) > 0 {
				if totalWrites <= 0 {
					if segment.idx != latestSeq {
						w.Header().Set("Lp-Trickle-Latest", strconv.Itoa(latestSeq))
					}
					w.Header().Set("Lp-Trickle-Seq", strconv.Itoa(segment.idx))
					w.Header().Set("Content-Type", s.mimeType)
				}
				n, err := w.Write(data)
				totalWrites += n
				if err != nil {
					return totalWrites, err
				}
				// TODO error if bytes written != len(data) ?
				flusher.Flush()
			}
			if eof {
				if totalWrites <= 0 {
					// check if the channel was closed; sometimes we drop / skip a segment
					s.mutex.RLock()
					closed := s.closed
					latestSeq := s.latestWrite
					s.mutex.RUnlock()
					w.Header().Set("Lp-Trickle-Seq", strconv.Itoa(segment.idx))
					if closed {
						w.Header().Set("Lp-Trickle-Closed", "terminated")
					} else {
						// usually happens if a publisher cancels a pending segment right before closing the channel
						// other times, the subscriber is slow and the segment falls out of the live window
						// send over latest seq so slow clients can grab leading edge
						w.Header().Set("Lp-Trickle-Latest", strconv.Itoa(latestSeq))
						w.WriteHeader(470)
					}
				}
				return totalWrites, nil
			}
		}
	}

	if n, err := sendData(); err != nil {
		// Handle write error or client disconnect
		slog.Error("Error sending data to client", "stream", s.name, "idx", segment.idx, "sentBytes", n, "err", err)
		return
	}
}

func newSegment(idx int) *Segment {
	mu := &sync.Mutex{}
	return &Segment{
		idx:    idx,
		buffer: new(bytes.Buffer),
		cond:   sync.NewCond(mu),
		mutex:  mu,
	}
}

func (segment *Segment) writeData(data []byte) {
	segment.mutex.Lock()
	defer segment.mutex.Unlock()

	// Write to buffer
	segment.buffer.Write(data)

	// Signal waiting readers
	segment.cond.Broadcast()
}

func (s *Segment) readData(startPos int) ([]byte, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for {
		totalLen := s.buffer.Len()
		if startPos < totalLen {
			data := s.buffer.Bytes()[startPos:totalLen]
			return data, s.closed
		}
		if startPos > totalLen {
			slog.Info("Invalid start pos, invoking eof")
			// This might happen if the buffer was reset
			// eg because of a repeated POST
			return nil, true
		}
		if s.closed {
			return nil, true
		}
		// Wait for new data
		s.cond.Wait()
	}
}

func (s *Segment) close() {
	if s == nil {
		return // sometimes happens, weird
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if !s.closed {
		s.closed = true
		s.cond.Broadcast()
	}
}
func (s *Segment) isFresh() bool {
	// fresh segments have not been written to yet
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return !s.closed && s.buffer.Len() == 0
}

func (ss *SegmentSubscriber) readData() ([]byte, bool) {
	data, eof := ss.segment.readData(ss.readPos)
	ss.readPos += len(data)
	return data, eof
}
