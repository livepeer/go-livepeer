package core

import (
	"context"
	ogErrors "errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/url"
	"os"
	"path"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"

	lpmon "github.com/livepeer/go-livepeer/monitor"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
)

var transcodeLoopTimeout = 1 * time.Minute

// Transcoder / orchestrator RPC interface implementation
type orchestrator struct {
	address ethcommon.Address
	node    *LivepeerNode
}

func (orch *orchestrator) ServiceURI() *url.URL {
	return orch.node.GetServiceURI()
}

func (orch *orchestrator) CurrentBlock() *big.Int {
	if orch.node == nil || orch.node.Database == nil {
		return nil
	}
	block, _ := orch.node.Database.LastSeenBlock()
	return block
}

func (orch *orchestrator) Sign(msg []byte) ([]byte, error) {
	if orch.node == nil || orch.node.Eth == nil {
		return []byte{}, nil
	}
	return orch.node.Eth.Sign(crypto.Keccak256(msg))
}

func (orch *orchestrator) VerifySig(addr ethcommon.Address, msg string, sig []byte) bool {
	if orch.node == nil || orch.node.Eth == nil {
		return true
	}
	return pm.VerifySig(addr, crypto.Keccak256([]byte(msg)), sig)
}

func (orch *orchestrator) Address() ethcommon.Address {
	return orch.address
}

func (orch *orchestrator) TranscoderSecret() string {
	return orch.node.OrchSecret
}

func (orch *orchestrator) CheckCapacity(mid ManifestID) error {
	orch.node.segmentMutex.RLock()
	defer orch.node.segmentMutex.RUnlock()
	if _, ok := orch.node.SegmentChans[mid]; ok {
		return nil
	}
	if len(orch.node.SegmentChans) >= MaxSessions {
		return ErrOrchCap
	}
	return nil
}

func (orch *orchestrator) TranscodeSeg(md *SegTranscodingMetadata, seg *stream.HLSSegment) (*TranscodeResult, error) {
	return orch.node.sendToTranscodeLoop(md, seg)
}

func (orch *orchestrator) ServeTranscoder(stream net.Transcoder_RegisterTranscoderServer, capacity int) {
	orch.node.serveTranscoder(stream, capacity)
}

func (orch *orchestrator) TranscoderResults(tcId int64, res *RemoteTranscoderResult) {
	orch.node.TranscoderManager.transcoderResults(tcId, res)
}

func (orch *orchestrator) ProcessPayment(payment net.Payment, manifestID ManifestID) error {
	if orch.node == nil || orch.node.Recipient == nil {
		return nil
	}

	if payment.TicketParams == nil {
		return fmt.Errorf("Could not find TicketParams for payment %v", payment)
	}

	if payment.Sender == nil || len(payment.Sender) == 0 {
		return fmt.Errorf("Could not find Sender for payment: %v", payment)
	}

	seed := new(big.Int).SetBytes(payment.TicketParams.Seed)

	ticket := &pm.Ticket{
		Recipient:              ethcommon.BytesToAddress(payment.TicketParams.Recipient),
		Sender:                 ethcommon.BytesToAddress(payment.Sender),
		FaceValue:              new(big.Int).SetBytes(payment.TicketParams.FaceValue),
		WinProb:                new(big.Int).SetBytes(payment.TicketParams.WinProb),
		RecipientRandHash:      ethcommon.BytesToHash(payment.TicketParams.RecipientRandHash),
		CreationRound:          payment.ExpirationParams.CreationRound,
		CreationRoundBlockHash: ethcommon.BytesToHash(payment.ExpirationParams.CreationRoundBlockHash),
	}

	var didReceiveErr bool

	for _, tsp := range payment.TicketSenderParams {

		ticket.SenderNonce = tsp.SenderNonce

		glog.V(common.DEBUG).Infof("Receiving ticket manifestID=%v faceValue=%v winProb=%v ev=%v", manifestID, ticket.FaceValue, ticket.WinProbRat().FloatString(10), ticket.EV().FloatString(2))

		_, won, err := orch.node.Recipient.ReceiveTicket(
			ticket,
			tsp.Sig,
			seed,
		)
		if err != nil {
			glog.Errorf("Error receiving ticket manifestID=%v recipientRandHash=%x senderNonce=%v: %v", manifestID, ticket.RecipientRandHash, ticket.SenderNonce, err)

			didReceiveErr = true
		}

		pmErr, ok := err.(pm.Error)
		if err == nil || (ok && pmErr.Acceptable()) {
			// Add ticket EV to credit
			orch.node.Balances.Credit(manifestID, ticket.EV())
		}

		if won {
			glog.V(common.DEBUG).Infof("Received winning ticket manifestID=%v recipientRandHash=%x senderNonce=%v", manifestID, ticket.RecipientRandHash, ticket.SenderNonce)

			go func(ticket *pm.Ticket, sig []byte, seed *big.Int) {
				if err := orch.node.Recipient.RedeemWinningTicket(ticket, sig, seed); err != nil {
					glog.Errorf("error redeeming ticket manifestID=%v recipientRandHash=%x senderNonce=%v: %v", manifestID, ticket.RecipientRandHash, ticket.SenderNonce, err)
				}
			}(ticket, tsp.Sig, seed)
		}
	}

	if didReceiveErr {
		return fmt.Errorf("error receiving tickets with payment")
	}

	return nil
}

func (orch *orchestrator) TicketParams(sender ethcommon.Address) (*net.TicketParams, error) {
	if orch.node == nil || orch.node.Recipient == nil {
		return nil, nil
	}

	params, err := orch.node.Recipient.TicketParams(sender)
	if err != nil {
		return nil, err
	}

	return &net.TicketParams{
		Recipient:         params.Recipient.Bytes(),
		FaceValue:         params.FaceValue.Bytes(),
		WinProb:           params.WinProb.Bytes(),
		RecipientRandHash: params.RecipientRandHash.Bytes(),
		Seed:              params.Seed.Bytes(),
	}, nil
}

func (orch *orchestrator) PriceInfo(sender ethcommon.Address) (*net.PriceInfo, error) {
	if orch.node == nil || orch.node.Recipient == nil {
		return nil, nil
	}

	txCostMultiplier, err := orch.node.Recipient.TxCostMultiplier(sender)
	if err != nil {
		return nil, err
	}
	// pricePerPixel = basePrice * (1 + 1/ txCostMultiplier)
	overhead := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Inv(txCostMultiplier))
	price := new(big.Rat).Mul(orch.node.PriceInfo, overhead)
	return &net.PriceInfo{
		PricePerUnit:  price.Num().Int64(),
		PixelsPerUnit: price.Denom().Int64(),
	}, nil
}

// SufficientBalance checks whether the credit balance for a stream is sufficient
// to proceed with downloading and transcoding
func (orch *orchestrator) SufficientBalance(manifestID ManifestID) bool {
	if orch.node == nil || orch.node.Recipient == nil || orch.node.Balances == nil {
		return true
	}
	if orch.node.Balances.Balance(manifestID).Cmp(orch.node.Recipient.EV()) < 0 {
		return false
	}
	return true
}

// DebitFees debits the balance for a ManifestID based on the amount of output pixels * price
func (orch *orchestrator) DebitFees(manifestID ManifestID, price *net.PriceInfo, pixels int64) {
	// Don't debit in offchain mode
	if orch.node == nil || orch.node.Balances == nil {
		return
	}
	priceRat := big.NewRat(price.GetPricePerUnit(), price.GetPixelsPerUnit())
	orch.node.Balances.Debit(manifestID, priceRat.Mul(priceRat, big.NewRat(pixels, 1)))
}

func NewOrchestrator(n *LivepeerNode) *orchestrator {
	var addr ethcommon.Address
	if n.Eth != nil {
		addr = n.Eth.Account().Address
	}
	return &orchestrator{
		node:    n,
		address: addr,
	}
}

// LivepeerNode transcode methods

var ErrOrchBusy = ogErrors.New("OrchestratorBusy")
var ErrOrchCap = ogErrors.New("OrchestratorCapped")

type TranscodeResult struct {
	Err           error
	Sig           []byte
	TranscodeData []*TranscodeData
	OS            drivers.OSSession
}

type TranscodeData struct {
	Data   []byte
	Pixels int64
}

type SegChanData struct {
	seg *stream.HLSSegment
	md  *SegTranscodingMetadata
	res chan *TranscodeResult
}

type RemoteTranscoderResult struct {
	Segments [][]byte
	Err      error
}

type SegmentChan chan *SegChanData
type TranscoderChan chan *RemoteTranscoderResult

type transcodeConfig struct {
	OS      drivers.OSSession
	LocalOS drivers.OSSession
}

func (n *RemoteTranscoderManager) getTaskChan(taskId int64) (TranscoderChan, error) {
	n.taskMutex.RLock()
	defer n.taskMutex.RUnlock()
	if tc, ok := n.taskChans[taskId]; ok {
		return tc, nil
	}
	return nil, fmt.Errorf("No transcoder channel")
}
func (n *RemoteTranscoderManager) addTaskChan() (int64, TranscoderChan) {
	n.taskMutex.Lock()
	defer n.taskMutex.Unlock()
	taskId := n.taskCount
	n.taskCount++
	if tc, ok := n.taskChans[taskId]; ok {
		// should really never happen
		glog.V(common.DEBUG).Info("Transcoder channel already exists for ", taskId)
		return taskId, tc
	}
	n.taskChans[taskId] = make(TranscoderChan, 1)
	return taskId, n.taskChans[taskId]
}
func (n *RemoteTranscoderManager) removeTaskChan(taskId int64) {
	n.taskMutex.Lock()
	defer n.taskMutex.Unlock()
	if _, ok := n.taskChans[taskId]; !ok {
		glog.V(common.DEBUG).Info("Transcoder channel nonexistent for job ", taskId)
		return
	}
	delete(n.taskChans, taskId)
}

func (n *LivepeerNode) getSegmentChan(md *SegTranscodingMetadata) (SegmentChan, error) {
	// concurrency concerns here? what if a chan is added mid-call?
	n.segmentMutex.Lock()
	defer n.segmentMutex.Unlock()
	if sc, ok := n.SegmentChans[md.ManifestID]; ok {
		return sc, nil
	}
	if len(n.SegmentChans) >= MaxSessions {
		return nil, ErrOrchCap
	}
	sc := make(SegmentChan, 1)
	glog.V(common.DEBUG).Info("Creating new segment chan for manifest ", md.ManifestID)
	if err := n.transcodeSegmentLoop(md, sc); err != nil {
		return nil, err
	}
	n.SegmentChans[md.ManifestID] = sc
	if lpmon.Enabled {
		lpmon.CurrentSessions(len(n.SegmentChans))
	}
	return sc, nil
}

func (n *LivepeerNode) sendToTranscodeLoop(md *SegTranscodingMetadata, seg *stream.HLSSegment) (*TranscodeResult, error) {
	glog.V(common.DEBUG).Infof("Starting to transcode segment manifest=%s seqNo=%d", string(md.ManifestID), md.Seq)
	ch, err := n.getSegmentChan(md)
	if err != nil {
		glog.Error("Could not find segment chan ", err)
		return nil, err
	}
	segChanData := &SegChanData{seg: seg, md: md, res: make(chan *TranscodeResult, 1)}
	select {
	case ch <- segChanData:
		glog.V(common.DEBUG).Infof("Submitted segment to transcode loop manifestID=%s seqNo=%d", md.ManifestID, md.Seq)
	default:
		// sending segChan should not block; if it does, the channel is busy
		glog.Errorf("Transcoder was busy with a previous segment manifestID=%s seqNo=%d", md.ManifestID, md.Seq)
		return nil, ErrOrchBusy
	}
	res := <-segChanData.res
	return res, res.Err
}

func (n *LivepeerNode) transcodeSeg(config transcodeConfig, seg *stream.HLSSegment, md *SegTranscodingMetadata) *TranscodeResult {
	var fnamep *string
	terr := func(err error) *TranscodeResult {
		if fnamep != nil {
			os.Remove(*fnamep)
		}
		return &TranscodeResult{Err: err}
	}

	// Prevent unnecessary work, check for replayed sequence numbers.
	// NOTE: If we ever process segments from the same job concurrently,
	// we may still end up doing work multiple times. But this is OK for now.

	//Assume d is in the right format, write it to disk
	inName := common.RandName() + ".ts"
	if _, err := os.Stat(n.WorkDir); os.IsNotExist(err) {
		err := os.Mkdir(n.WorkDir, 0700)
		if err != nil {
			glog.Errorf("Transcoder cannot create workdir: %v", err)
			return terr(err)
		}
	}
	// Create input file from segment. Removed after claiming complete or error
	fname := path.Join(n.WorkDir, inName)
	fnamep = &fname
	if err := ioutil.WriteFile(fname, seg.Data, 0644); err != nil {
		glog.Errorf("Transcoder cannot write file: %v", err)
		return terr(err)
	}

	// Check if there's a transcoder available
	if n.Transcoder == nil {
		return terr(ErrTranscoderAvail)
	}
	transcoder := n.Transcoder

	var url string
	_, isLocal := transcoder.(*LocalTranscoder)
	// Small optimization: serve from disk for local transcoding
	if isLocal {
		url = fname
	} else if drivers.IsOwnExternal(seg.Name) {
		// We're using a remote TC and segment is already in our own OS
		// Incurs an additional download for topologies with T on local network!
		url = seg.Name
	} else {
		// Need to store segment in our local OS
		var err error
		name := fmt.Sprintf("%d.ts", seg.SeqNo)
		url, err = config.LocalOS.SaveData(name, seg.Data)
		if err != nil {
			return terr(err)
		}
		seg.Name = url
	}

	//Do the transcoding
	start := time.Now()
	tData, err := transcoder.Transcode(url, md.Profiles)
	if err != nil {
		glog.Errorf("Error transcoding manifest=%s segNo=%d segName=%s - %v", string(md.ManifestID), seg.SeqNo, seg.Name, err)
		return terr(err)
	}
	// transcodeEnd := time.Now().UTC()
	if len(tData) != len(md.Profiles) {
		glog.Errorf("Did not receive the correct number of transcoded segments; got %v expected %v manifest=%s seqNo=%d", len(tData),
			len(md.Profiles), string(md.ManifestID), seg.SeqNo)
		return terr(fmt.Errorf("MismatchedSegments"))
	}

	took := time.Since(start)
	tProfileData := make(map[ffmpeg.VideoProfile][]byte, 0)
	glog.V(common.DEBUG).Infof("Transcoding of segment manifestID=%s seqNo=%d took=%v", string(md.ManifestID), seg.SeqNo, took)
	if isLocal && monitor.Enabled {
		monitor.SegmentTranscoded(0, seg.SeqNo, took, common.ProfilesNames(md.Profiles))
	}

	// Prepare the result object
	var tr TranscodeResult
	segHashes := make([][]byte, len(tData))

	for i := range md.Profiles {
		if tData[i] == nil || len(tData[i]) < 25 {
			glog.Errorf("Cannot find transcoded segment for manifest=%s seqNo=%d dataLength=%d",
				string(md.ManifestID), seg.SeqNo, len(tData[i]))
			return terr(fmt.Errorf("ZeroSegments"))
		}
		tProfileData[md.Profiles[i]] = tData[i]
		tr.TranscodeData = append(tr.TranscodeData, &TranscodeData{
			Data: tData[i],
			// TODO: ADD NUMBER OF OUTPUT PIXELS
		})
		glog.V(common.DEBUG).Infof("Transcoded segment manifest=%s seqNo=%d profile=%s len=%d",
			string(md.ManifestID), seg.SeqNo, md.Profiles[i].Name, len(tData[i]))
		hash := crypto.Keccak256(tData[i])
		segHashes[i] = hash
	}
	os.Remove(fname)
	tr.OS = config.OS

	if n == nil || n.Eth == nil {
		return &tr
	}

	segHash := crypto.Keccak256(segHashes...)
	tr.Sig, tr.Err = n.Eth.Sign(segHash)
	if tr.Err != nil {
		glog.Error("Unable to sign hash of transcoded segment hashes: ", tr.Err)
	}
	return &tr
}

func (n *LivepeerNode) transcodeSegmentLoop(md *SegTranscodingMetadata, segChan SegmentChan) error {
	glog.V(common.DEBUG).Info("Starting transcode segment loop for ", md.ManifestID)

	// Set up local OS for any remote transcoders to use if necessary
	if drivers.NodeStorage == nil {
		return fmt.Errorf("Missing local storage")
	}

	los := drivers.NodeStorage.NewSession(string(md.ManifestID))

	// determine appropriate OS to use
	os := drivers.NewSession(md.OS)
	if os == nil {
		// no preference (or unknown pref), so use our own
		os = los
	}

	config := transcodeConfig{
		OS:      os,
		LocalOS: los,
	}
	go func() {
		for {
			// XXX make context timeout configurable
			ctx, cancel := context.WithTimeout(context.Background(), transcodeLoopTimeout)
			select {
			case <-ctx.Done():
				// timeout; clean up goroutine here
				os.EndSession()
				los.EndSession()
				glog.V(common.DEBUG).Info("Segment loop timed out; closing ", md.ManifestID)
				n.segmentMutex.Lock()
				if _, ok := n.SegmentChans[md.ManifestID]; ok {
					close(n.SegmentChans[md.ManifestID])
					delete(n.SegmentChans, md.ManifestID)
					if lpmon.Enabled {
						lpmon.CurrentSessions(len(n.SegmentChans))
					}
				}
				n.segmentMutex.Unlock()
				return
			case chanData := <-segChan:
				chanData.res <- n.transcodeSeg(config, chanData.seg, chanData.md)
			}
			cancel()
		}
	}()
	return nil
}

func (n *LivepeerNode) serveTranscoder(stream net.Transcoder_RegisterTranscoderServer, capacity int) {
	from := common.GetConnectionAddr(stream.Context())
	n.TranscoderManager.Manage(stream, capacity)
	glog.V(common.DEBUG).Infof("Closing transcoder=%s channel", from)
}

func (n *RemoteTranscoderManager) transcoderResults(tcId int64, res *RemoteTranscoderResult) {
	remoteChan, err := n.getTaskChan(tcId)
	if err != nil {
		return // do we need to return anything?
	}
	remoteChan <- res
}

type RemoteTranscoder struct {
	manager  *RemoteTranscoderManager
	stream   net.Transcoder_RegisterTranscoderServer
	eof      chan struct{}
	addr     string
	capacity int
}

// RemoteTranscoderFatalError wraps error to indicate that error is fatal
type RemoteTranscoderFatalError struct {
	error
}

// NewRemoteTranscoderFatalError creates new RemoteTranscoderFatalError
// Exported here to be used in other packages
func NewRemoteTranscoderFatalError(err error) error {
	return RemoteTranscoderFatalError{err}
}

var RemoteTranscoderTimeout = 8 * time.Second
var ErrRemoteTranscoderTimeout = errors.New("Remote transcoder took too long")

func (rt *RemoteTranscoder) done() {
	// select so we don't block indefinitely if there's no listener
	select {
	case rt.eof <- struct{}{}:
	default:
	}
}

// Transcode do actual transcoding by sending work to remote transcoder and waiting for the result
func (rt *RemoteTranscoder) Transcode(fname string, profiles []ffmpeg.VideoProfile) ([][]byte, error) {
	taskId, taskChan := rt.manager.addTaskChan()
	defer rt.manager.removeTaskChan(taskId)
	signalEOF := func(err error) ([][]byte, error) {
		rt.done()
		glog.Errorf("Fatal error with remote transcoder=%s taskId=%d fname=%s err=%v", rt.addr, taskId, fname, err)
		return [][]byte{}, RemoteTranscoderFatalError{err}
	}
	msg := &net.NotifySegment{
		Url:      fname,
		TaskId:   taskId,
		Profiles: common.ProfilesToTranscodeOpts(profiles),
	}
	err := rt.stream.Send(msg)
	if err != nil {
		return signalEOF(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), RemoteTranscoderTimeout)
	defer cancel()
	select {
	case <-ctx.Done():
		return signalEOF(ErrRemoteTranscoderTimeout)
	case chanData := <-taskChan:
		glog.Infof("Successfully received results from remote transcoder=%s segments=%d taskId=%d fname=%s",
			rt.addr, len(chanData.Segments), taskId, fname)
		return chanData.Segments, chanData.Err
	}
}
func NewRemoteTranscoder(m *RemoteTranscoderManager, stream net.Transcoder_RegisterTranscoderServer, capacity int) *RemoteTranscoder {
	return &RemoteTranscoder{
		manager:  m,
		stream:   stream,
		eof:      make(chan struct{}, 1),
		capacity: capacity,
		addr:     common.GetConnectionAddr(stream.Context()),
	}
}

func NewRemoteTranscoderManager() *RemoteTranscoderManager {
	return &RemoteTranscoderManager{
		remoteTranscoders: []*RemoteTranscoder{},
		liveTranscoders:   map[net.Transcoder_RegisterTranscoderServer]*RemoteTranscoder{},
		RTmutex:           &sync.Mutex{},

		taskMutex: &sync.RWMutex{},
		taskChans: make(map[int64]TranscoderChan),
	}
}

type RemoteTranscoderManager struct {
	remoteTranscoders []*RemoteTranscoder
	liveTranscoders   map[net.Transcoder_RegisterTranscoderServer]*RemoteTranscoder
	RTmutex           *sync.Mutex

	// For tracking tasks assigned to remote transcoders
	taskMutex *sync.RWMutex
	taskChans map[int64]TranscoderChan
	taskCount int64
}

func (rtm *RemoteTranscoderManager) RegisteredTranscodersCount() int {
	rtm.RTmutex.Lock()
	defer rtm.RTmutex.Unlock()
	return len(rtm.liveTranscoders)
}

func (rtm *RemoteTranscoderManager) RegisteredTranscodersInfo() []net.RemoteTranscoderInfo {
	rtm.RTmutex.Lock()
	res := make([]net.RemoteTranscoderInfo, 0, len(rtm.liveTranscoders))
	for _, transcoder := range rtm.liveTranscoders {
		res = append(res, net.RemoteTranscoderInfo{Address: transcoder.addr, Capacity: transcoder.capacity})
	}
	rtm.RTmutex.Unlock()
	return res
}

func (rtm *RemoteTranscoderManager) Manage(stream net.Transcoder_RegisterTranscoderServer, capacity int) {
	from := common.GetConnectionAddr(stream.Context())
	transcoder := NewRemoteTranscoder(rtm, stream, capacity)
	go func() {
		ctx := stream.Context()
		<-ctx.Done()
		err := ctx.Err()
		glog.Errorf("Stream closed for transcoder=%s, err=%v", from, err)
		transcoder.done()
	}()

	rtm.RTmutex.Lock()
	rtm.liveTranscoders[transcoder.stream] = transcoder
	for i := 0; i < capacity; i++ {
		rtm.remoteTranscoders = append(rtm.remoteTranscoders, transcoder)
	}
	rtm.RTmutex.Unlock()

	<-transcoder.eof
	glog.Infof("Got transcoder=%s eof, removing from live transcoders map", from)

	rtm.RTmutex.Lock()
	delete(rtm.liveTranscoders, transcoder.stream)
	rtm.RTmutex.Unlock()
}

func (rtm *RemoteTranscoderManager) selectTranscoder() *RemoteTranscoder {
	rtm.RTmutex.Lock()
	defer rtm.RTmutex.Unlock()

	checkTranscoders := func(rtm *RemoteTranscoderManager) bool {
		return len(rtm.remoteTranscoders) > 0
	}

	for checkTranscoders(rtm) {
		last := len(rtm.remoteTranscoders) - 1
		currentTranscoder, remoteTranscoders := rtm.remoteTranscoders[last], rtm.remoteTranscoders[:last]
		rtm.remoteTranscoders = remoteTranscoders
		if _, ok := rtm.liveTranscoders[currentTranscoder.stream]; ok {
			return currentTranscoder
		}
		// try again if transcoder does not exist in table
	}

	return nil
}

func (rtm *RemoteTranscoderManager) completeTranscoders(trans *RemoteTranscoder) {
	rtm.RTmutex.Lock()
	defer rtm.RTmutex.Unlock()

	if _, ok := rtm.liveTranscoders[trans.stream]; ok {
		rtm.remoteTranscoders = append(rtm.remoteTranscoders, trans)
	}
}

func (rtm *RemoteTranscoderManager) Transcode(fname string, profiles []ffmpeg.VideoProfile) ([][]byte, error) {
	currentTranscoder := rtm.selectTranscoder()
	if currentTranscoder == nil {
		return nil, errors.New("No transcoders available")
	}
	res, err := currentTranscoder.Transcode(fname, profiles)
	_, fatal := err.(RemoteTranscoderFatalError)
	if fatal {
		// Don't retry if we've timed out; broadcaster likely to have moved on
		// XXX problematic for VOD when we *should* retry
		if err.(RemoteTranscoderFatalError).error == ErrRemoteTranscoderTimeout {
			return res, err
		}
		return rtm.Transcode(fname, profiles)
	}
	rtm.completeTranscoders(currentTranscoder)
	return res, err
}
