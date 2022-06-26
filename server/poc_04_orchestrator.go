package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/livepeer/go-livepeer/clog"
)

// work in progress ..
type OrchestratorRouter interface{}

// Receives stream from B. Validates media data signature.
// Chooses available T from the pool of T workers. Forwards input stream to T.
// Receives output rendition stream from T. Signs media data of rendition streams.
// TODO: payment ticket verification
// TODO: payment processing on segment end
type OrchestratorConnection struct {
	connection *websocket.Conn
	router     OrchestratorRouter
	ctx        context.Context
	pool       *OrchestratorTranscoderPool
	wallet     TestWallet
	bSignCheck SignatureChecker
	jobSpec    *OutputSpec
	firstFrame *InputChunk
	transcoder *OrchestratorTranscoderConnection
}

func (o *OrchestratorConnection) Init() {}

func (o *OrchestratorConnection) Handshake(r *http.Request) error {
	var err error
	// expect OutputSpec first
	switch message := recvMessage(o.connection).(type) {
	case *OutputSpec:
		o.jobSpec = message
	case error:
		return message
	default:
		return fmt.Errorf("expected OutputSpec")
	}

	o.firstFrame, err = recvFirstFrame(o.connection)
	if err != nil {
		return err
	}
	// TODO: discuss how to agree on payment ticket before we know segment lenght / pixel count
	if err := verifyMediaMetadata(o.firstFrame); err != nil {
		// media differs from advertized, payment ticket may be different from reality
		return err
	}
	// Check initial signature
	signatureValid := o.bSignCheck.Check(o.firstFrame.Bytes, o.firstFrame.Signature)
	if !signatureValid {
		return fmt.Errorf("Signature check failed")
	}
	return nil
}

// Allocate T for the job. Check B signature. Forward messages to T.
func (o *OrchestratorConnection) RunUntilCompletion() error {
	// Allocate T
	var err error
	if o.transcoder, err = o.pool.allocate(); err != nil {
		return err
	}
	defer o.releaseTranscoder()
	go o.runReceiveLoop()

	if err := o.transcoder.Send(o.jobSpec); err != nil {
		return err
	}
	if err := o.transcoder.Send(o.firstFrame); err != nil {
		return err
	}
	// Reading loop
	// Here backpressure is desired. Will indicate to B our processing pace.
	for {
		switch message := recvMessage(o.connection).(type) {
		case *InputChunk:
			// Check signature
			signatureValid := o.bSignCheck.Check(message.Bytes, message.Signature)
			if !signatureValid {
				return fmt.Errorf("Signature check failed")
			}
			if err := o.transcoder.Send(message); err != nil {
				return err
			}
		case error:
			return message
		default:
			if err := o.transcoder.Send(message); err != nil {
				return err
			}
		}
	}
}

// Receive Messages from T. Sign media data. Forward messages to B.
func (o *OrchestratorConnection) runReceiveLoop() {
	for {
		switch message := o.transcoder.Recv().(type) {
		case *OutputChunk:
			// Place signature
			var err error
			if message.Signature, err = o.wallet.SignMediaData(message.Bytes); err != nil {
				clog.Errorf(o.ctx, "OrchestratorConnection.runReceiveLoop SignMediaData %v", err)
				return
			}
			sendMessage(o.connection, message)
		case error:
			clog.Errorf(o.ctx, "OrchestratorConnection.runReceiveLoop recv %v", message)
			return
		default:
			sendMessage(o.connection, message)
		}
	}
}

func (o *OrchestratorConnection) releaseTranscoder() {
	// We don't need to close connection to transcoder, ensuring GPU resources are released.
	// Expecting reconnect from T
	o.transcoder.Close()
	o.pool.removeWorker(o.transcoder)
}

// HTTP part for accepting B & T connections
type OrchestratorIngest struct {
	port        int
	connections []*OrchestratorConnection
	pool        *OrchestratorTranscoderPool
	verifyer    SignatureChecker

	// Test only. In production we deduce this from chain
	SingleKnownBaddress SignatureChecker
}

// O accepts connections from B and connections from T.
// Ts are placed into idle pool to be used for jobs comming from B.
func (o *OrchestratorIngest) Init(httpMux *http.ServeMux, router OrchestratorRouter, pool *OrchestratorTranscoderPool, wallet TestWallet) {
	o.verifyer = wallet.CreatePublicInfo()
	o.connections = make([]*OrchestratorConnection, 0)
	o.pool = pool

	// B connection setup:
	initNewTranscodeJob := func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			glog.Error("O websocket upgrade error")
			http.Error(w, "websocket upgrade error", 500)
			return
		}
		defer ws.Close()
		connection := OrchestratorConnection{
			connection: ws,
			pool:       pool,
			router:     router,
			ctx:        r.Context(),
			wallet:     wallet,
			bSignCheck: o.SingleKnownBaddress,
		}
		connection.Init()
		clog.Infof(connection.ctx, "O ingest connected, doing handshake")
		if err := connection.Handshake(r); err != nil {
			clog.Errorf(connection.ctx, "O Handshake error %v", err)
			return
		}

		if err := connection.RunUntilCompletion(); err != nil {
			clog.Errorf(connection.ctx, "O ingest error %v", err)
			return
		}
		clog.Infof(connection.ctx, "O ingest complete")
	}

	// T connection setup
	acceptTranscoder := func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			glog.Error("transcoder websocket upgrade error")
			http.Error(w, "websocket upgrade error", 500)
			return
		}
		connection := &OrchestratorTranscoderConnection{
			connection: ws,
			pool:       pool,
			ctx:        r.Context(),
		}
		connection.Init()
		// Run login step. If ok will be part of the pool.
		if err := connection.login(); err != nil {
			clog.Errorf(connection.ctx, "T login error %v", err)
			connection.Close()
		}
		o.pool.workerIdle(connection)
	}
	httpMux.HandleFunc("/streaming/", initNewTranscodeJob)
	httpMux.HandleFunc("/transcoder/", acceptTranscoder)
}
