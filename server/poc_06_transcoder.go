package server

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/livepeer/lpms/ffmpeg"
)

// Operates on single Transcoder card.
// Transcoder node should spawn multiple StreamingTranscoders to cover all installed cards.
// To distribute load unevenly accross cards configure `connectionLimit`(workers) appropriately.
//   For ex. double `connectionLimit` for cards you wish to send more work.
// Constructed with address of O and number of workers to use.
// On start spawns configured number of workers(TranscoderConnection). When any connection to O
//   is broken we replace it with new one.
// TODO: discuss GPU load/capacity strategy. When GPU load approaches max we can close idle connections
//   in turn stopping new job assignment to this GPU. O chooses from all available connections randomly.
//   If there is less of our connections there are less chanses for a new job to land here.
type StreamingTranscoder struct {
	connectionLimit int
	hwDeviceIndex   int
	orchestratorUrl url.URL
	loginPassword   string

	mutex       sync.Mutex
	connections []*TranscoderConnection
}

// Spawn required number of workers and exit. We count on connection broken events to respawn workers.
func (t *StreamingTranscoder) run() {
	t.connections = make([]*TranscoderConnection, 0)
	// spawn connections up to given limit
	fmt.Printf("T spawn %d workers on %s\n", t.connectionLimit, t.orchestratorUrl.String())
	for i := 0; i < t.connectionLimit; i++ {
		t.spawnNewConnection()
	}
}

// Decide whether to spawn new connections
func (t *StreamingTranscoder) reconnect() {
	t.mutex.Lock()
	missing := t.connectionLimit - len(t.connections)
	t.mutex.Unlock()
	// Always spawn up to configured limit.
	// TODO: discuss how to manage GPU load. Maybe close idle connections as current load reaches maximum?
	for i := 0; i < missing; i++ {
		// TODO: consider disconnect reason, apply exponential backoff in case O refuses login
		t.spawnNewConnection()
	}
}

// When worker(transcoder) is disconnected we get this event.
func (t *StreamingTranscoder) remove(transcoder *TranscoderConnection) {
	removed := false
	t.mutex.Lock()
	// Removing pointer from slice copypasta:
	if len(t.connections) == 0 {
		return
	}
	for i := 0; i < len(t.connections); {
		if t.connections[i] == transcoder {
			removed = true
			// Unordered list of pointers. t.connections slice SHOULD be sole owner of underlaying array.
			t.connections[i] = t.connections[len(t.connections)-1]
			t.connections = t.connections[:len(t.connections)-1]
		} else {
			i += 1
		}
	}
	t.mutex.Unlock()
	// maybe reconnect:
	if removed {
		t.reconnect()
	}
}

func (t *StreamingTranscoder) Start() {
	// spawn goroutine and detach
	go t.run()
}

// Construct worker with all credentials for the job.
func (t *StreamingTranscoder) spawnNewConnection() {
	var transcoder *TranscoderConnection
	transcoder = &TranscoderConnection{
		hwDeviceIndex:   t.hwDeviceIndex,
		orchestratorUrl: t.orchestratorUrl,
		loginPassword:   t.loginPassword,
		deleteFromBooks: func() { t.remove(transcoder) },
	}
	t.mutex.Lock()
	t.connections = append(t.connections, transcoder)
	t.mutex.Unlock()
	go transcoder.run()
}

func verifyMediaMetadata(frame *InputChunk) error {
	_, acodec, vcodec, _, err := ffmpeg.GetCodecInfoBytes(frame.Bytes)
	if err != nil {
		return fmt.Errorf("Media metadata detect error: %v\n", err)
	}
	if acodec != frame.Format.AudioCodec {
		return fmt.Errorf("Metadata mismatch acodec: %s; signaled to be %s\n", acodec, frame.Format.AudioCodec)
	}
	if vcodec != frame.Format.VideoCodec {
		return fmt.Errorf("Metadata mismatch vcodec: %s; signaled to be %s\n", vcodec, frame.Format.VideoCodec)
	}
	return nil
}
