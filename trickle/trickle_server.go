package trickle

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// TODO sweep idle streams connections

const CHANGEFEED = "_changes"

type TrickleServerConfig struct {
	BasePath   string
	Mux        *http.ServeMux
	Changefeed bool
}

type Server struct {
	mutex   sync.RWMutex
	streams map[string]*Stream

	// for internal channels
	changefeed  bool
	internalPub *TrickleLocalPublisher
}

type Stream struct {
	mutex       sync.RWMutex
	segments    []*Segment
	latestWrite int
	name        string
	mimeType    string
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
}

func ConfigureServer(config TrickleServerConfig) *Server {
	streamManager := &Server{
		streams:    make(map[string]*Stream),
		changefeed: config.Changefeed,
	}

	// set up changefeed
	if streamManager.changefeed {
		streamManager.internalPub = NewLocalPublisher(streamManager, CHANGEFEED, "application/json")
		streamManager.internalPub.CreateChannel()
	}

	applyDefaults(&config)
	var (
		mux      = config.Mux
		basePath = config.BasePath
	)

	mux.HandleFunc("GET "+basePath+"{streamName}/{idx}", streamManager.handleGet)
	mux.HandleFunc("POST "+basePath+"{streamName}/{idx}", streamManager.handlePost)
	mux.HandleFunc("DELETE "+basePath+"{streamName}", streamManager.handleDelete)
	return streamManager
}

func (sm *Server) getStream(streamName string) (*Stream, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	stream, exists := sm.streams[streamName]
	return stream, exists
}

func (sm *Server) getOrCreateStream(streamName, mimeType string) *Stream {
	sm.mutex.Lock()

	stream, exists := sm.streams[streamName]
	if !exists {
		stream = &Stream{
			segments: make([]*Segment, maxSegmentsPerStream),
			name:     streamName,
			mimeType: mimeType,
		}
		sm.streams[streamName] = stream
		slog.Info("Creating stream", "stream", streamName)
	}
	sm.mutex.Unlock()

	// update changefeed
	if !exists && sm.changefeed {
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
		stream.clear()
	}
	sm.streams = make(map[string]*Stream)
}

func (s *Stream) clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, segment := range s.segments {
		segment.close()
	}
	s.segments = make([]*Segment, maxSegmentsPerStream)
}

func (sm *Server) closeStream(streamName string) error {
	stream, exists := sm.getStream(streamName)
	if !exists {
		return errors.New("Invalid stream")
	}

	// TODO there is a bit of an issue around session reuse

	stream.clear()
	sm.mutex.Lock()
	delete(sm.streams, streamName)
	sm.mutex.Unlock()
	slog.Info("Deleted stream", "streamName", streamName)

	// update changefeed if needed
	if !sm.changefeed {
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

func (sm *Server) handlePost(w http.ResponseWriter, r *http.Request) {
	stream := sm.getOrCreateStream(r.PathValue("streamName"), r.Header.Get("Content-Type"))
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
}

func (tr *timeoutReader) Read(p []byte) (int, error) {
	if tr.firstByteRead {
		// After the first byte is read, proceed normally
		return tr.body.Read(p)
	}

	done := make(chan struct{})
	var n int
	var err error

	go func() {
		n, err = tr.body.Read(p)
		close(done)
	}()

	select {
	case <-done:
		if n > 0 {
			tr.firstByteRead = true
		}
		return n, err
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
		http.Error(w, "Entry already exists for this index", http.StatusBadRequest)
		return
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
		// TODO figure out how to better handle restarts while maintaining ordering
		/* } else if idx > s.latestWrite { */
	} else {
		s.latestWrite = idx
	}
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

func (s *Stream) getForRead(idx int) (*Segment, bool) {
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
	return segment, exists(segment, idx)
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
	segment, exists := s.getForRead(idx)
	if !exists {
		http.Error(w, "Entry not found", http.StatusNotFound)
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
