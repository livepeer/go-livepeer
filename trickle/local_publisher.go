package trickle

import (
	"errors"
	"io"
	"log/slog"
	"sync"
)

// local (in-memory) publisher for trickle protocol

type TrickleLocalPublisher struct {
	channelName string
	mimeType    string
	server      *Server

	mu  *sync.Mutex
	seq int
}

func NewLocalPublisher(sm *Server, channelName string, mimeType string) *TrickleLocalPublisher {
	return &TrickleLocalPublisher{
		channelName: channelName,
		server:      sm,
		mu:          &sync.Mutex{},
		mimeType:    mimeType,
	}
}

func (c *TrickleLocalPublisher) CreateChannel() {
	c.server.getOrCreateStream(c.channelName, c.mimeType, true)
}

func (c *TrickleLocalPublisher) Write(data io.Reader) error {
	stream := c.server.getOrCreateStream(c.channelName, c.mimeType, true)
	c.mu.Lock()
	seq := c.seq
	segment, exists, closed := stream.getForWrite(seq)
	if closed {
		c.mu.Unlock()
		return errors.New("stream closed")
	}
	if exists {
		c.mu.Unlock()
		return errors.New("Entry already exists for this sequence")
	}

	// before we begin - let's pre-create the next segment
	nextSeq := c.seq + 1
	if _, exists, closed = stream.getForWrite(nextSeq); exists || closed {
		c.mu.Unlock()
		if closed {
			return errors.New("Stream closed")
		}
		return errors.New("Next entry already exists in this sequence")
	}
	c.seq = nextSeq
	c.mu.Unlock()

	// now continue with the show
	buf := make([]byte, 1024*32) // 32kb to begin with
	totalRead := 0
	for {
		n, err := data.Read(buf)
		if n > 0 {
			segment.writeData(buf[:n])
			totalRead += n
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			slog.Info("Error reading published data", "channel", c.channelName, "seq", seq, "bytes written", totalRead, "err", err)
		}
	}
	segment.close()
	return nil
}

func (c *TrickleLocalPublisher) Close() error {
	return c.server.closeStream(c.channelName)
}
