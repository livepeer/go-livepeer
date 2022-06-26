package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/lpms/ffmpeg"
)

// Info required for transcoder. We get frames from the Mist, all output spec is here.
// Currently B calculates this. Because there is webhook call.
// If we want to get this from Mist we can use HTTP headers or a new control message.
// New control message is preffered if other nodes need to receive it.
type TranscodeJobSpec struct {
	ManifestID core.ManifestID
	Detection  core.DetectionConfig
	Renditions []ffmpeg.VideoProfile
	// VerificationFreq uint
}

// Minimal subset of info to contact specific O.
type OrchestratorContact struct {
	WsAddress url.URL
	verifyer  SignatureChecker
	Trusted   bool
}

// Produced by BroadcasterRouter when choosing group of Os to use for the job.
type ChosenOrchestrators struct {
	Orchestrators      []*OrchestratorContact
	CalcPerceptualHash bool
	verified           bool
}

// Represents Livepeer-network logic. Should change a lot, soon.
type BroadcasterRouter interface {
	GetTranscodeSpec(r *http.Request, info MediaFormatInfo) (TranscodeJobSpec, error)
	ChooseOrchestrators(r *http.Request, info MediaFormatInfo) (ChosenOrchestrators, error)
	OutputProblem(context *BDownstreamDecision) error
}

type FrameCount int

// Process Mist input stream and handle Os output streams.
type BroadcasterConnection struct {
	connection *websocket.Conn
	router     BroadcasterRouter
	ctx        context.Context
	wallet     TestWallet

	jobspec     TranscodeJobSpec
	firstFrame  *InputChunk
	mutex       sync.Mutex
	chosenNodes ChosenOrchestrators
	downstreams []*BroadcasterOutConnection
	// downstreams produce messages to recvFrame chan
	recvFrame chan *MessageFromO

	// parameters
	slowOutputTolerance FrameCount
}

func (c *BroadcasterConnection) Init() {
	c.downstreams = make([]*BroadcasterOutConnection, 0)
	c.slowOutputTolerance = 25
}

// Receive first frame describing input stream.
// Choose suitable orchestrators to send job in parallel.
// TODO: discuss how to create payment ticket. Can we do it without pixel count of future segment?
func (c *BroadcasterConnection) Handshake(r *http.Request) error {
	var err error
	if c.firstFrame, err = recvFirstFrame(c.connection); err != nil {
		return err
	}
	// Mist does not provide signature, B signs media data later.
	// We get MediaFormatInfo in c.firstFrame let's verify it:
	if err := verifyMediaMetadata(c.firstFrame); err != nil {
		// B considers Mist to have good intentions. We may consider to update metadata here
		//   instead of returning error.
		return err
	}
	c.jobspec, err = c.router.GetTranscodeSpec(r, c.firstFrame.Format)
	if err != nil {
		c.closeWithError(fmt.Sprint(err))
		return err
	}
	// Initail choice of orchestrators for this job
	c.chosenNodes, err = c.router.ChooseOrchestrators(r, c.firstFrame.Format)
	if err != nil {
		c.closeWithError(fmt.Sprint(err))
		return err
	}
	return nil
}

// Creates OutputSpec message for transcoder, defines outputs transcoder produces.
// Conencts to chosen Os and forwards all incoming messages. Signs media data before forwarding.
// Spawns new goroutines for each downstream(O connection). B sends stream to several Os at
//   different pace and receives renditions at different pace. B receives message from Mist,
//   adds signature and forwards to chosen Os. We use N channels on N goroutines for N Os.
//   Each received frame is posted on each of N channels to achieve different send pace.
//   Sometimes O would be less responsive causing frames to pile up in its buffered channel.
//   We detect this and take action - usually breaking connection makes sense.
// With resulting renditions the situation is reversed - fan-in. For ex. we have 3 Os each producing
//   3 rendition streams comming at its own pace. Here each frame is sent to single channel for
//   BroadcasterConnection to inspect and forward back to Mist. Makes sense to send single resulting
//   renditions back to Mist.
// TODO: discuss what is proper action on O's slow pace
// TODO: discuss how slow pace handling differs in VOD case
// TODO: discuss best method for choosing one O to forward renditions back to Mist, in advance
// TODO: discuss method and conditions for switching to different O.
func (c *BroadcasterConnection) RunUntilCompletion() error {
	// prepare OutputSpec
	spec := newOutputSpec()
	for i := 0; i < len(c.jobspec.Renditions); i++ {
		// TODO: compose different name?
		spec.Outputs = append(spec.Outputs, RenditionSpec{
			Index:   i,
			Name:    fmt.Sprintf("%s-%d.ts", c.jobspec.ManifestID, i),
			Profile: c.jobspec.Renditions[i],
		})
	}
	// Connect to all Os
	for i := 0; i < len(c.chosenNodes.Orchestrators); i++ {
		var downstream *BroadcasterOutConnection
		downstream = &BroadcasterOutConnection{
			oAddress:        c.chosenNodes.Orchestrators[i].WsAddress,
			sendResult:      c.recvFrame,
			oSignCheck:      c.chosenNodes.Orchestrators[i].verifyer,
			deleteFromBooks: func() { c.remove(downstream) },
		}
		downstream.Init()
		// Store objects to our books
		c.downstreams = append(c.downstreams, downstream)
		// queue OutputSpec as first message
		downstream.SendFrame <- spec
	}
	for i := 0; i < len(c.chosenNodes.Orchestrators); i++ {
		// Start goro for processing downstream connection
		go c.downstreams[i].Run()
	}
	// Start separate goro for write loop; Consuming all O alternatives and choosing one to return back to Mist.
	go c.multiplexResults()

	// Sign first frame
	c.firstFrame.Sign(&c.wallet)

	// Receive loop follows. We get frame from socket and push it to each downstream queue.
	// Individual downstream is not allowed to apply back-pressure to input data flow. In VOD case this might change.
	// When latency compounds in downstream result data flow beyond our tolerance we do failover to other available nodes.
	//   This can happen when downstream stops responding or network bottleneck or processing bottleneck.
	//   In any case downstream is then considered unsuitable.
	downstreams := c.getActiveConnections()
	for i := 0; i < len(downstreams); i++ {
		// SendFrame chan should not block, ever. This is for first frame, later we will check the queue to take action on slow nodes.
		downstreams[i].SendFrame <- c.firstFrame
	}
	for {
		breakOperation, err := c.processMessage()
		if err != nil {
			return err
		}
		if breakOperation {
			break
		}
	}
	return nil
}

// Takes message from Mist and forwards it to all active Os.
// Mist pace can be faster than O pace. Then frames would be piling inside BroadcasterOutConnection.
//   We have threshold defined `.slowOutputTolerance` to declare O unsuitable. When sendRateAlarming is
//   detected BroadcasterRouter is consulted to make a decision. Sometimes we are stuck with single O,
//   so we keep that slow O.
func (c *BroadcasterConnection) processMessage() (BreakOperation, error) {
	switch message := recvMessage(c.connection).(type) {
	case *InputChunk:
		// Sign message before forwarding
		if err := message.Sign(&c.wallet); err != nil {
			return true, err
		}
		c.forwardMessageToO(message)
	case error:
		return true, message
	default:
		c.forwardMessageToO(message)
	}
	return false, nil
}

// Queue same message to all connected Os.
// Detect if there is excessive amount of frames already queued and take action.
func (c *BroadcasterConnection) forwardMessageToO(message interface{}) {
	connectedOs := c.getActiveConnections()
	for i := 0; i < len(connectedOs); i++ {
		connectedOs[i].SendFrame <- message
		// Note: framesInBuffer actually contains other unread control messages besides InputChunk.
		framesInBuffer := FrameCount(len(connectedOs[i].SendFrame))
		// Check channel size. In case we are piling frames and downstream is not receiving:
		sendRateAlarming := framesInBuffer > c.slowOutputTolerance
		if sendRateAlarming {
			// Inactivity detected. Either peer is gone or network congested or slow to process input.
			// Ask router for confirmation:
			decision := &BDownstreamDecision{framesInBuffer, c, connectedOs[i]}
			if err := c.router.OutputProblem(decision); err != nil {
				// TODO: report this event to error-funnel
				// TODO: More detailed error message, how many connections left, how late are they..
				clog.Errorf(c.ctx, "Output congested, removing %s frames-late=%d", connectedOs[i].oAddress.String(), framesInBuffer)
				decision.BreakConnection()
				continue
			}
		}
	}
}

// Separate goro for reading resulting,encoded frames from multiple available Os.
// Chooses single O to return its results to Mist and CDN
func (c *BroadcasterConnection) multiplexResults() {
	// First stage. Quickest O gets selected for output.
	// TODO: Discuss which logic is actually suitable for output selection.
	// We are using signing of each frame data to establish trust. However we
	//   should not wait for entire segment to validate and then choose.
	// Each frame we delay our choice, is extra latency introduced.
	messageFrom, exitEarly := c.receiveFirstFrame()
	if exitEarly {
		return
	}
	chosenOutput := messageFrom.downstream

	// Second stage. Results of chosenOutput are forwarded to Mist and CDN
	// TODO: Discuss how frame by frame verification can be applied here
	// TODO: CDN uploading in separate goros. Is CDN moved to Mist? What about recording?

	// TODO: handle virtual segment boundary
	for {
		goesToMist := messageFrom.downstream == chosenOutput
		if goesToMist {
			if err := sendMessage(c.connection, messageFrom.message); err != nil {
				return
			}
		}
		if messageFrom, exitEarly = <-c.recvFrame; exitEarly {
			return
		}
		// TODO: c.router.ChooseOrchestrators() need to happen after each segment
	}
}

// Similar to `func recvFirstFrame()`. Using recvFrame channel instead of network.
func (c *BroadcasterConnection) receiveFirstFrame() (*MessageFromO, BreakOperation) {
	for {
		messageFrom, ok := <-c.recvFrame
		if !ok {
			return nil, true
		}
		switch messageFrom.message.(type) {
		case *OutputChunk:
			return messageFrom, false
		case *EndOfInput:
			return nil, true
		default:
			// Skip all other messages. There should not be other messages at this stage.
			fmt.Printf("BroadcasterConnection.receiveFirstFrame skipping %T", messageFrom.message)
			continue
		}
	}
}

// Used when downstream connection breaks to update our books.
func (c *BroadcasterConnection) remove(downstream *BroadcasterOutConnection) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	// Removing pointer from our slice. Looks like naive code
	if len(c.downstreams) == 0 {
		return
	}
	for i := 0; i < len(c.downstreams); {
		if c.downstreams[i] == downstream {
			// Unordered list of pointers. c.downstreams slice SHOULD be sole owner of underlaying array.
			c.downstreams[i] = c.downstreams[len(c.downstreams)-1]
			c.downstreams = c.downstreams[:len(c.downstreams)-1]
		} else {
			i += 1
		}
	}
}

// TODO: .. cleanup not done yet ..
func (c *BroadcasterConnection) closeWithError(reason string) error {
	if err := sendMessage(c.connection, newFatalError()); err != nil {
		return err
	}
	// TODO: Close() would break connection immediately. Send CloseMessage type.
	c.connection.Close()
	return nil
}

// Used to fetch snapshot of active downstream connections
func (c *BroadcasterConnection) getActiveConnections() []*BroadcasterOutConnection {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.downstreams[:]
}

// HTTP part for accepting Mist connections
type BroadcasterIngest struct {
	connections []*BroadcasterConnection
	verifyer    SignatureChecker
}

// Installs HTTP handler for upgrading to websocket connection. Setup broadcasterConnection object.
func (b *BroadcasterIngest) Init(httpMux *http.ServeMux, router BroadcasterRouter, wallet TestWallet) {
	// Each new broadcasterConnection gets a copy of B wallet to sign outgoing media data.
	//   Maybe this should be provided by BroadcasterRouter if wallet changes during our operation?
	b.verifyer = wallet.CreatePublicInfo()
	connectionInit := func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			glog.Error("B websocket upgrade error")
			http.Error(w, "websocket upgrade error", 500)
			return
		}
		broadcasterConnection := BroadcasterConnection{
			connection: ws,
			router:     router,
			ctx:        r.Context(),
			wallet:     wallet,
		}
		broadcasterConnection.Init()
		clog.Infof(broadcasterConnection.ctx, "Mist connected, doing handshake")
		if err := broadcasterConnection.Handshake(r); err != nil {
			clog.Errorf(broadcasterConnection.ctx, "B Handshake error %v", err)
			return
		}

		if err := broadcasterConnection.RunUntilCompletion(); err != nil {
			clog.Errorf(broadcasterConnection.ctx, "B error %v", err)
			return
		}
		clog.Infof(broadcasterConnection.ctx, "B ws complete")
	}
	httpMux.HandleFunc("/streaming", connectionInit)
}

// Settings used for TCP in our connections
var upgrader = websocket.Upgrader{
	ReadBufferSize:  16384,
	WriteBufferSize: 16384,
}
