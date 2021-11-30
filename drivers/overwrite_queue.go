package drivers

import (
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
)

const (
	timeoutMultiplier    = 1.5
	overwriteQueueLength = 32
)

type (
	OverwriteQueue struct {
		desc           string
		name           string
		session        OSSession
		maxRetries     int
		initialTimeout time.Duration
		maxTimeout     time.Duration
		queue          chan []byte
		quit           chan struct{}
	}
)

func NewOverwriteQueue(session OSSession, name, desc string, maxRetries int, initialTimeout, maxTimeout time.Duration) *OverwriteQueue {
	oq := &OverwriteQueue{
		desc:           desc,
		name:           name,
		maxRetries:     maxRetries,
		session:        session,
		initialTimeout: initialTimeout,
		maxTimeout:     maxTimeout,
		queue:          make(chan []byte, overwriteQueueLength),
		quit:           make(chan struct{}),
	}
	if maxRetries < 1 {
		panic("maxRetries should be greater than zero")
	}
	go oq.workerLoop()
	return oq
}

// Save queues data to be saved
func (oq *OverwriteQueue) Save(data []byte) {
	oq.queue <- data
}

// StopAfter stops reading loop after some time
func (oq *OverwriteQueue) StopAfter(pause time.Duration) {
	go func(p time.Duration) {
		time.Sleep(p)
		close(oq.quit)
	}(pause)
}

func (oq *OverwriteQueue) workerLoop() {
	var err error
	var took time.Duration
	for {
		select {
		case data := <-oq.queue:
			timeout := oq.initialTimeout
			for try := 0; try < oq.maxRetries; try++ {
				// we only care about last data
				data = oq.getLastMessage(data)
				glog.V(common.VERBOSE).Infof("Start saving %s name=%s bytes=%d try=%d", oq.desc, oq.name, len(data), try)
				now := time.Now()
				_, err = oq.session.SaveData(oq.name, data, nil, timeout)
				took := time.Since(now)
				if err == nil {
					glog.V(common.VERBOSE).Infof("Saving %s name=%s bytes=%d took=%s try=%d", oq.desc, oq.name,
						len(data), took, try)
					break
				}
				timeout = time.Duration(float64(timeout) * timeoutMultiplier)
				if timeout > oq.maxTimeout {
					timeout = oq.maxTimeout
				}
			}
			if err != nil {
				glog.Errorf("Error saving %s name=%s bytes=%d took=%s try=%d err=%q", oq.desc, oq.name,
					len(data), took, oq.maxRetries, err)
			}

		case <-oq.quit:
			return
		}
	}
}

func (oq *OverwriteQueue) getLastMessage(current []byte) []byte {
	res := current
	for {
		select {
		case data := <-oq.queue:
			res = data
		default:
			return res
		}
	}
}
