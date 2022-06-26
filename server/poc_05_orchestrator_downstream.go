package server

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/livepeer/go-livepeer/clog"
)

// Registers transcoder workers for O to assign job to.
// Multiple transcoder connections belong to one GPU.
// Configure connection count on transcoder to tune for GPU preference or capacity.
// TODO: Cleanup missing. Decide when to keep or break worker connection.
type OrchestratorTranscoderPool struct {
	loginPassword  string
	maxConnections int
	mutex          sync.Mutex
	idle           []*OrchestratorTranscoderConnection
	occupied       []*OrchestratorTranscoderConnection
}

func (p *OrchestratorTranscoderPool) Init() {
	p.idle = make([]*OrchestratorTranscoderConnection, 0)
	p.occupied = make([]*OrchestratorTranscoderConnection, 0)
}

// Reserve idle connection
func (p *OrchestratorTranscoderPool) allocate() (*OrchestratorTranscoderConnection, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if len(p.idle) == 0 {
		return nil, fmt.Errorf("No Ts available, job denied")
	}
	// grab random one
	index := rand.Intn(len(p.idle))
	chosen := p.idle[index]
	// replace with last one
	p.idle[index] = p.idle[len(p.idle)-1]
	// remove last one from our books
	p.idle = p.idle[:len(p.idle)-1]
	// add it to occupied
	p.occupied = append(p.occupied, chosen)
	return chosen, nil
}

func (p *OrchestratorTranscoderPool) removeWorker(transcoder *OrchestratorTranscoderConnection) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	defer transcoder.Close()
	removeFromSlice(&p.occupied, transcoder)
	removeFromSlice(&p.idle, transcoder)
}

func (p *OrchestratorTranscoderPool) workerIdle(transcoder *OrchestratorTranscoderConnection) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	removeFromSlice(&p.occupied, transcoder)
	removeFromSlice(&p.idle, transcoder)
	p.idle = append(p.idle, transcoder)
}

// Very funny lookgin func *[]*
func removeFromSlice(slice *[]*OrchestratorTranscoderConnection, transcoder *OrchestratorTranscoderConnection) {
	// Remove pointer from slice copy-pasta follows.
	if len(*slice) == 0 {
		return
	}
	for i := 0; i < len(*slice); {
		if (*slice)[i] == transcoder {
			// Unordered list of pointers. slice SHOULD be sole owner of underlaying array.
			(*slice)[i] = (*slice)[len(*slice)-1]
			(*slice) = (*slice)[:len(*slice)-1]
		} else {
			i += 1
		}
	}
}

// Connection to T worker.
type OrchestratorTranscoderConnection struct {
	connection *websocket.Conn
	pool       *OrchestratorTranscoderPool
	ctx        context.Context
}

func (o *OrchestratorTranscoderConnection) Init() {}

func (o *OrchestratorTranscoderConnection) Recv() interface{} {
	return recvMessage(o.connection)
}

func (o *OrchestratorTranscoderConnection) Send(message interface{}) error {
	return sendMessage(o.connection, message)
}

func (o *OrchestratorTranscoderConnection) Close() {
	o.connection.Close()
}

// Make sure first message is TranscoderLogin JSON with correct passcode
func (o *OrchestratorTranscoderConnection) login() error {
	switch message := recvMessage(o.connection).(type) {
	case *TranscoderLogin:
		correctPasscode := message.Passcode == o.pool.loginPassword
		if correctPasscode {
			return nil
		}
		return fmt.Errorf("wrong passcode")
	case error:
		clog.Errorf(o.ctx, "first message read err %v", message)
		return message
	default:
		return fmt.Errorf("login error")
	}
}
