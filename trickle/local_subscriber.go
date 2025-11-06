package trickle

import (
	"errors"
	"io"
	"log/slog"
	"strconv"
	"sync"
)

// local (in-memory) subscriber for trickle protocol

type TrickleData struct {
	Reader   io.Reader
	Metadata map[string]string
}

type TrickleLocalSubscriber struct {
	channelName string
	server      *Server

	mu  *sync.Mutex
	seq int
}

func NewLocalSubscriber(sm *Server, channelName string) *TrickleLocalSubscriber {
	return &TrickleLocalSubscriber{
		channelName: channelName,
		server:      sm,
		mu:          &sync.Mutex{},
		seq:         -1,
	}
}

func (c *TrickleLocalSubscriber) Read() (*TrickleData, error) {
	stream, exists := c.server.getStream(c.channelName)
	if !exists {
		return nil, errors.New("stream not found")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	segment, latestSeq, exists, closed := stream.getForRead(c.seq)
	if !exists {
		if closed {
			return nil, EOS
		}
		return nil, errors.New("seq not found")
	}
	seq := segment.idx
	if seq >= 0 {
		c.seq = seq + 1
	}
	r, w := io.Pipe()
	go func() {
		subscriber := &SegmentSubscriber{
			segment: segment,
		}
		for {
			data, eof := subscriber.readData()
			n, err := w.Write(data)
			if err != nil {
				slog.Info("Error writing", "channel", c.channelName, "seq", segment.idx, "err", err)
				return
			}
			if n != len(data) {
				slog.Info("Did not write enough data to local subscriber", "channel", c.channelName, "seq", segment.idx)
				return
			}
			if eof {
				// trigger eof on the reader
				w.Close()
				return
			}
		}
	}()
	return &TrickleData{
		Reader: r,
		Metadata: map[string]string{
			"Lp-Trickle-Latest": strconv.Itoa(latestSeq),
			"Lp-Trickle-Seq":    strconv.Itoa(segment.idx),
			"Content-Type":      stream.mimeType,
		}, // TODO take more metadata from http headers
	}, nil
}

func (c *TrickleLocalSubscriber) SetSeq(seq int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq = seq
}
