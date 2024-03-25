package core

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/url"
	"os"
	"path"
	"sort"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/go-tools/drivers"

	lpcrypto "github.com/livepeer/go-livepeer/crypto"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/lpms/stream"
)

const maxSegmentChannels = 4

// this is set to be higher than httpPushTimeout in server/mediaserver.go so that B has a chance to end the session
// based on httpPushTimeout before transcodeLoopTimeout is reached
var transcodeLoopTimeout = 70 * time.Second

// Gives us more control of "timeout" / cancellation behavior during testing
var transcodeLoopContext = func() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), transcodeLoopTimeout)
}

// Transcoder / orchestrator RPC interface implementation
type orchestrator struct {
	address ethcommon.Address
	node    *LivepeerNode
	rm      common.RoundsManager
	secret  []byte
}

func (orch *orchestrator) ServiceURI() *url.URL {
	return orch.node.GetServiceURI()
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
	return lpcrypto.VerifySig(addr, crypto.Keccak256([]byte(msg)), sig)
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

func (orch *orchestrator) TranscodeSeg(ctx context.Context, md *SegTranscodingMetadata, seg *stream.HLSSegment) (*TranscodeResult, error) {
	return orch.node.sendToTranscodeLoop(ctx, md, seg)
}

func (orch *orchestrator) ServeTranscoder(stream net.Transcoder_RegisterTranscoderServer, capacity int, capabilities *net.Capabilities) {
	orch.node.serveTranscoder(stream, capacity, capabilities)
}

func (orch *orchestrator) TranscoderResults(tcID int64, res *RemoteTranscoderResult) {
	orch.node.TranscoderManager.transcoderResults(tcID, res)
}

func (orch *orchestrator) ProcessPayment(ctx context.Context, payment net.Payment, manifestID ManifestID) error {
	if orch.node == nil || orch.node.Recipient == nil {
		return nil
	}

	if (payment.Sender == nil || len(payment.Sender) == 0) && payment.TicketParams != nil {
		return fmt.Errorf("Could not find Sender for payment: %v", payment)
	}
	sender := ethcommon.BytesToAddress(payment.Sender)

	if payment.TicketParams == nil {
		// No ticket params means that the price is 0, then set the fixed price per session to 0
		orch.setFixedPricePerSession(sender, manifestID, big.NewRat(0, 1))
		return nil
	}

	recipientAddr := ethcommon.BytesToAddress(payment.TicketParams.Recipient)
	ok, err := orch.isActive(recipientAddr)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("orchestrator %v is inactive in round %v, cannot process payments", recipientAddr.Hex(), orch.rm.LastInitializedRound())
	}

	priceInfo := payment.GetExpectedPrice()

	seed := new(big.Int).SetBytes(payment.TicketParams.Seed)

	priceInfoRat, err := common.RatPriceInfo(priceInfo)
	if err != nil {
		return fmt.Errorf("invalid expected price sent with payment err=%q", err)
	}
	if priceInfoRat == nil {
		return fmt.Errorf("invalid expected price sent with payment err=%q", "expected price is nil")
	}

	// During the first payment, set the fixed price per session
	orch.setFixedPricePerSession(sender, manifestID, priceInfoRat)

	ticketParams := &pm.TicketParams{
		Recipient:         ethcommon.BytesToAddress(payment.TicketParams.Recipient),
		FaceValue:         new(big.Int).SetBytes(payment.TicketParams.FaceValue),
		WinProb:           new(big.Int).SetBytes(payment.TicketParams.WinProb),
		RecipientRandHash: ethcommon.BytesToHash(payment.TicketParams.RecipientRandHash),
		Seed:              seed,
		ExpirationBlock:   new(big.Int).SetBytes(payment.TicketParams.ExpirationBlock),
		PricePerPixel:     priceInfoRat,
	}

	ticketExpirationParams := &pm.TicketExpirationParams{
		CreationRound:          payment.ExpirationParams.CreationRound,
		CreationRoundBlockHash: ethcommon.BytesToHash(payment.ExpirationParams.CreationRoundBlockHash),
	}

	totalEV := big.NewRat(0, 1)
	totalTickets := 0
	totalWinningTickets := 0

	var receiveErr error

	for _, tsp := range payment.TicketSenderParams {

		ticket := pm.NewTicket(
			ticketParams,
			ticketExpirationParams,
			sender,
			tsp.SenderNonce,
		)

		_, won, err := orch.node.Recipient.ReceiveTicket(
			ticket,
			tsp.Sig,
			seed,
		)
		if err != nil {
			clog.Errorf(ctx, "Error receiving ticket sessionID=%v recipientRandHash=%x senderNonce=%v: %v", manifestID, ticket.RecipientRandHash, ticket.SenderNonce, err)

			if monitor.Enabled {
				monitor.PaymentRecvError(ctx, sender.Hex(), err.Error())
			}
			if _, ok := err.(*pm.FatalReceiveErr); ok {
				return err
			}
			receiveErr = err
		}

		if receiveErr == nil {
			// Add ticket EV to credit
			ev := ticket.EV()
			orch.node.Balances.Credit(sender, manifestID, ev)
			totalEV.Add(totalEV, ev)
			totalTickets++
		}

		if won {
			clog.V(common.DEBUG).Infof(ctx, "Received winning ticket sessionID=%v recipientRandHash=%x senderNonce=%v", manifestID, ticket.RecipientRandHash, ticket.SenderNonce)

			totalWinningTickets++

			go func(ticket *pm.Ticket, sig []byte, seed *big.Int) {
				if err := orch.node.Recipient.RedeemWinningTicket(ticket, sig, seed); err != nil {
					clog.Errorf(ctx, "error redeeming ticket sessionID=%v recipientRandHash=%x senderNonce=%v err=%q", manifestID, ticket.RecipientRandHash, ticket.SenderNonce, err)
				}
			}(ticket, tsp.Sig, seed)
		}
	}

	clog.V(common.DEBUG).Infof(ctx, "Payment tickets processed sessionID=%v faceValue=%v winProb=%v totalTickets=%v totalEV=%v", manifestID, eth.FormatUnits(ticketParams.FaceValue, "ETH"), ticketParams.WinProbRat().FloatString(10), totalTickets, totalEV.FloatString(2))

	if monitor.Enabled {
		monitor.TicketValueRecv(ctx, sender.Hex(), totalEV)
		monitor.TicketsRecv(ctx, sender.Hex(), totalTickets)
		monitor.WinningTicketsRecv(ctx, sender.Hex(), totalWinningTickets)
	}

	if receiveErr != nil {
		return receiveErr
	}

	return nil
}

func (orch *orchestrator) TicketParams(sender ethcommon.Address, priceInfo *net.PriceInfo) (*net.TicketParams, error) {
	if orch.node == nil || orch.node.Recipient == nil {
		return nil, nil
	}

	ratPriceInfo, err := common.RatPriceInfo(priceInfo)
	if err != nil {
		return nil, err
	}

	params, err := orch.node.Recipient.TicketParams(sender, ratPriceInfo)
	if err != nil {
		return nil, err
	}

	return &net.TicketParams{
		Recipient:         params.Recipient.Bytes(),
		FaceValue:         params.FaceValue.Bytes(),
		WinProb:           params.WinProb.Bytes(),
		RecipientRandHash: params.RecipientRandHash.Bytes(),
		Seed:              params.Seed.Bytes(),
		ExpirationBlock:   params.ExpirationBlock.Bytes(),
		ExpirationParams: &net.TicketExpirationParams{
			CreationRound:          params.ExpirationParams.CreationRound,
			CreationRoundBlockHash: params.ExpirationParams.CreationRoundBlockHash.Bytes(),
		},
	}, nil
}

func (orch *orchestrator) PriceInfo(sender ethcommon.Address, manifestID ManifestID) (*net.PriceInfo, error) {
	if orch.node == nil || orch.node.Recipient == nil {
		return nil, nil
	}

	price, err := orch.priceInfo(sender, manifestID)
	if err != nil {
		return nil, err
	}

	if monitor.Enabled {
		monitor.TranscodingPrice(sender.String(), price)
	}

	return &net.PriceInfo{
		PricePerUnit:  price.Num().Int64(),
		PixelsPerUnit: price.Denom().Int64(),
	}, nil
}

// priceInfo returns price per pixel as a fixed point number wrapped in a big.Rat
func (orch *orchestrator) priceInfo(sender ethcommon.Address, manifestID ManifestID) (*big.Rat, error) {
	basePrice := orch.node.GetBasePrice(sender.String())

	// If there is already a fixed price for the given session, use this price
	if manifestID != "" {
		if balances, ok := orch.node.Balances.balances[sender]; ok {
			fixedPrice := balances.FixedPrice(manifestID)
			if fixedPrice != nil {
				return fixedPrice, nil
			}
		}
	}

	if basePrice == nil {
		basePrice = orch.node.GetBasePrice("default")
	}

	if !orch.node.AutoAdjustPrice {
		return basePrice, nil
	}

	// If price = 0, overhead is 1
	// If price > 0, overhead = 1 + (1 / txCostMultiplier)
	overhead := big.NewRat(1, 1)
	if basePrice.Num().Cmp(big.NewInt(0)) > 0 {
		txCostMultiplier, err := orch.node.Recipient.TxCostMultiplier(sender)
		if err != nil {
			return nil, err
		}

		if txCostMultiplier.Cmp(big.NewRat(0, 1)) > 0 {
			overhead = overhead.Add(overhead, new(big.Rat).Inv(txCostMultiplier))
		}

	}
	// pricePerPixel = basePrice * overhead
	fixedPrice, err := common.PriceToFixed(new(big.Rat).Mul(basePrice, overhead))
	if err != nil {
		return nil, err
	}
	return common.FixedToPrice(fixedPrice), nil
}

// SufficientBalance checks whether the credit balance for a stream is sufficient
// to proceed with downloading and transcoding
func (orch *orchestrator) SufficientBalance(addr ethcommon.Address, manifestID ManifestID) bool {
	if orch.node == nil || orch.node.Recipient == nil || orch.node.Balances == nil {
		return true
	}

	balance := orch.node.Balances.Balance(addr, manifestID)
	if balance == nil || balance.Cmp(orch.node.Recipient.EV()) < 0 {
		return false
	}
	return true
}

// DebitFees debits the balance for a ManifestID based on the amount of output pixels * price
func (orch *orchestrator) DebitFees(addr ethcommon.Address, manifestID ManifestID, price *net.PriceInfo, pixels int64) {
	// Don't debit in offchain mode
	if orch.node == nil || orch.node.Balances == nil {
		return
	}
	priceRat := big.NewRat(price.GetPricePerUnit(), price.GetPixelsPerUnit())
	orch.node.Balances.Debit(addr, manifestID, priceRat.Mul(priceRat, big.NewRat(pixels, 1)))
}

func (orch *orchestrator) Capabilities() *net.Capabilities {
	if orch.node == nil {
		return nil
	}
	return orch.node.Capabilities.ToNetCapabilities()
}

func (orch *orchestrator) AuthToken(sessionID string, expiration int64) *net.AuthToken {
	h := hmac.New(sha256.New, orch.secret)
	msg := append([]byte(sessionID), new(big.Int).SetInt64(expiration).Bytes()...)
	h.Write(msg)

	return &net.AuthToken{
		Token:      h.Sum(nil),
		SessionId:  sessionID,
		Expiration: expiration,
	}
}

func (orch *orchestrator) isActive(addr ethcommon.Address) (bool, error) {
	filter := &common.DBOrchFilter{
		CurrentRound: orch.rm.LastInitializedRound(),
		Addresses:    []ethcommon.Address{addr},
	}
	orchs, err := orch.node.Database.SelectOrchs(filter)
	if err != nil {
		return false, err
	}

	return len(orchs) > 0, nil
}

func (orch *orchestrator) setFixedPricePerSession(sender ethcommon.Address, manifestID ManifestID, priceInfoRat *big.Rat) {
	if orch.node.Balances == nil {
		glog.Warning("Node balances are not initialized")
		return
	}
	if balances, ok := orch.node.Balances.balances[sender]; ok {
		if balances.FixedPrice(manifestID) == nil {
			balances.SetFixedPrice(manifestID, priceInfoRat)
			glog.V(6).Infof("Setting fixed price=%v for session=%v", priceInfoRat, manifestID)
		}
	}
}

func NewOrchestrator(n *LivepeerNode, rm common.RoundsManager) *orchestrator {
	var addr ethcommon.Address
	if n.Eth != nil {
		addr = n.Eth.Account().Address
	}
	return &orchestrator{
		node:    n,
		address: addr,
		rm:      rm,
		secret:  common.RandomBytesGenerator(32),
	}
}

// LivepeerNode transcode methods

var ErrOrchBusy = errors.New("OrchestratorBusy")
var ErrOrchCap = errors.New("OrchestratorCapped")

type TranscodeResult struct {
	Err           error
	Sig           []byte
	TranscodeData *TranscodeData
	OS            drivers.OSSession
}

// TranscodeData contains the transcoding output for an input segment
type TranscodeData struct {
	Segments []*TranscodedSegmentData
	Pixels   int64 // Decoded pixels
}

// TranscodedSegmentData contains encoded data for a profile
type TranscodedSegmentData struct {
	Data   []byte
	PHash  []byte // Perceptual hash data (maybe nil)
	Pixels int64  // Encoded pixels
}

type SegChanData struct {
	ctx context.Context
	seg *stream.HLSSegment
	md  *SegTranscodingMetadata
	res chan *TranscodeResult
}

type RemoteTranscoderResult struct {
	TranscodeData *TranscodeData
	Err           error
}

type SegmentChan chan *SegChanData
type TranscoderChan chan *RemoteTranscoderResult

type transcodeConfig struct {
	OS      drivers.OSSession
	LocalOS drivers.OSSession
}

func (rtm *RemoteTranscoderManager) getTaskChan(taskID int64) (TranscoderChan, error) {
	rtm.taskMutex.RLock()
	defer rtm.taskMutex.RUnlock()
	if tc, ok := rtm.taskChans[taskID]; ok {
		return tc, nil
	}
	return nil, fmt.Errorf("No transcoder channel")
}

func (rtm *RemoteTranscoderManager) addTaskChan() (int64, TranscoderChan) {
	rtm.taskMutex.Lock()
	defer rtm.taskMutex.Unlock()
	taskID := rtm.taskCount
	rtm.taskCount++
	if tc, ok := rtm.taskChans[taskID]; ok {
		// should really never happen
		glog.V(common.DEBUG).Info("Transcoder channel already exists for ", taskID)
		return taskID, tc
	}
	rtm.taskChans[taskID] = make(TranscoderChan, 1)
	return taskID, rtm.taskChans[taskID]
}

func (rtm *RemoteTranscoderManager) removeTaskChan(taskID int64) {
	rtm.taskMutex.Lock()
	defer rtm.taskMutex.Unlock()
	if _, ok := rtm.taskChans[taskID]; !ok {
		glog.V(common.DEBUG).Info("Transcoder channel nonexistent for job ", taskID)
		return
	}
	delete(rtm.taskChans, taskID)
}

func (n *LivepeerNode) getSegmentChan(ctx context.Context, md *SegTranscodingMetadata) (SegmentChan, error) {
	// concurrency concerns here? what if a chan is added mid-call?
	n.segmentMutex.Lock()
	defer n.segmentMutex.Unlock()
	if sc, ok := n.SegmentChans[ManifestID(md.AuthToken.SessionId)]; ok {
		return sc, nil
	}
	if len(n.SegmentChans) >= MaxSessions {
		return nil, ErrOrchCap
	}
	sc := make(SegmentChan, maxSegmentChannels)
	clog.V(common.DEBUG).Infof(ctx, "Creating new segment chan")
	if err := n.transcodeSegmentLoop(clog.Clone(context.Background(), ctx), md, sc); err != nil {
		return nil, err
	}
	n.SegmentChans[ManifestID(md.AuthToken.SessionId)] = sc
	if lpmon.Enabled {
		lpmon.CurrentSessions(len(n.SegmentChans))
	}
	return sc, nil
}

func (n *LivepeerNode) sendToTranscodeLoop(ctx context.Context, md *SegTranscodingMetadata, seg *stream.HLSSegment) (*TranscodeResult, error) {
	clog.V(common.DEBUG).Infof(ctx, "Starting to transcode segment")
	ch, err := n.getSegmentChan(ctx, md)
	if err != nil {
		clog.Errorf(ctx, "Could not find segment chan err=%q", err)
		return nil, err
	}
	segChanData := &SegChanData{ctx: ctx, seg: seg, md: md, res: make(chan *TranscodeResult, 1)}
	select {
	case ch <- segChanData:
		clog.V(common.DEBUG).Infof(ctx, "Submitted segment to transcode loop ")
	default:
		// sending segChan should not block; if it does, the channel is busy
		clog.Errorf(ctx, "Transcoder was busy with a previous segment")
		return nil, ErrOrchBusy
	}
	res := <-segChanData.res
	return res, res.Err
}

func (n *LivepeerNode) transcodeSeg(ctx context.Context, config transcodeConfig, seg *stream.HLSSegment, md *SegTranscodingMetadata) *TranscodeResult {
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
	inName := common.RandName() + ".tempfile"
	if _, err := os.Stat(n.WorkDir); os.IsNotExist(err) {
		err := os.Mkdir(n.WorkDir, 0700)
		if err != nil {
			clog.Errorf(ctx, "Transcoder cannot create workdir err=%q", err)
			return terr(err)
		}
	}
	// Create input file from segment. Removed after claiming complete or error
	fname := path.Join(n.WorkDir, inName)
	fnamep = &fname
	if err := ioutil.WriteFile(fname, seg.Data, 0644); err != nil {
		clog.Errorf(ctx, "Transcoder cannot write file err=%q", err)
		return terr(err)
	}

	// Check if there's a transcoder available
	if n.Transcoder == nil {
		return terr(ErrTranscoderAvail)
	}
	transcoder := n.Transcoder

	var url string
	_, isRemote := transcoder.(*RemoteTranscoderManager)
	// Small optimization: serve from disk for local transcoding
	if !isRemote {
		url = fname
	} else if config.OS.IsExternal() && config.OS.IsOwn(seg.Name) {
		// We're using a remote TC and segment is already in our own OS
		// Incurs an additional download for topologies with T on local network!
		url = seg.Name
	} else {
		// Need to store segment in our local OS
		var err error
		name := fmt.Sprintf("%d.tempfile", seg.SeqNo)
		url, err = config.LocalOS.SaveData(ctx, name, bytes.NewReader(seg.Data), nil, 0)
		if err != nil {
			return terr(err)
		}
		seg.Name = url
	}
	md.Fname = url

	//Do the transcoding
	start := time.Now()
	tData, err := transcoder.Transcode(ctx, md)
	if err != nil {
		if _, ok := err.(UnrecoverableError); ok {
			panic(err)
		}
		clog.Errorf(ctx, "Error transcoding segName=%s err=%q", seg.Name, err)
		return terr(err)
	}

	tSegments := tData.Segments
	if len(tSegments) != len(md.Profiles) {
		clog.Errorf(ctx, "Did not receive the correct number of transcoded segments; got %v expected %v", len(tSegments),
			len(md.Profiles))
		return terr(fmt.Errorf("MismatchedSegments"))
	}

	took := time.Since(start)
	clog.V(common.DEBUG).Infof(ctx, "Transcoding of segment took=%v", took)
	if monitor.Enabled {
		monitor.SegmentTranscoded(ctx, 0, seg.SeqNo, md.Duration, took, common.ProfilesNames(md.Profiles), true, true)
	}

	// Prepare the result object
	var tr TranscodeResult
	segHashes := make([][]byte, len(tSegments))

	for i := range md.Profiles {
		if tSegments[i].Data == nil || len(tSegments[i].Data) < 25 {
			clog.Errorf(ctx, "Cannot find transcoded segment for bytes=%d", len(tSegments[i].Data))
			return terr(fmt.Errorf("ZeroSegments"))
		}
		if md.CalcPerceptualHash && tSegments[i].PHash == nil {
			clog.Errorf(ctx, "Could not find perceptual hash for profile=%v", md.Profiles[i].Name)
			// FIXME: Return the error once everyone has upgraded their nodes
			// return terr(fmt.Errorf("MissingPerceptualHash"))
		}
		clog.V(common.DEBUG).Infof(ctx, "Transcoded segment profile=%s bytes=%d",
			md.Profiles[i].Name, len(tSegments[i].Data))
		hash := crypto.Keccak256(tSegments[i].Data)
		segHashes[i] = hash
	}
	os.Remove(fname)
	tr.OS = config.OS
	tr.TranscodeData = tData

	if n == nil || n.Eth == nil {
		return &tr
	}

	segHash := crypto.Keccak256(segHashes...)
	tr.Sig, tr.Err = n.Eth.Sign(segHash)
	if tr.Err != nil {
		clog.Errorf(ctx, "Unable to sign hash of transcoded segment hashes err=%q", tr.Err)
	}
	return &tr
}

func (n *LivepeerNode) transcodeSegmentLoop(logCtx context.Context, md *SegTranscodingMetadata, segChan SegmentChan) error {
	clog.V(common.DEBUG).Infof(logCtx, "Starting transcode segment loop for manifestID=%s sessionID=%s", md.ManifestID, md.AuthToken.SessionId)

	// Set up local OS for any remote transcoders to use if necessary
	if drivers.NodeStorage == nil {
		return fmt.Errorf("Missing local storage")
	}

	los := drivers.NodeStorage.NewSession(md.AuthToken.SessionId)

	// determine appropriate OS to use
	os := drivers.NewSession(FromNetOsInfo(md.OS))
	if os == nil {
		// no preference (or unknown pref), so use our own
		os = los
	}
	storageConfig := transcodeConfig{
		OS:      os,
		LocalOS: los,
	}
	n.storageMutex.Lock()
	n.StorageConfigs[md.AuthToken.SessionId] = &storageConfig
	n.storageMutex.Unlock()
	go func() {
		for {
			// XXX make context timeout configurable
			ctx, cancel := context.WithTimeout(context.Background(), transcodeLoopTimeout)
			select {
			case <-ctx.Done():
				clog.V(common.DEBUG).Infof(logCtx, "Segment loop timed out; closing ")
				n.endTranscodingSession(md.AuthToken.SessionId, logCtx)
				return
			case chanData, ok := <-segChan:
				// Check if channel was closed due to endTranscodingSession being called by B
				if !ok {
					cancel()
					return
				}
				chanData.res <- n.transcodeSeg(chanData.ctx, storageConfig, chanData.seg, chanData.md)
			}
			cancel()
		}
	}()
	return nil
}

func (n *LivepeerNode) endTranscodingSession(sessionId string, logCtx context.Context) {
	// timeout; clean up goroutine here
	var (
		exists  bool
		storage *transcodeConfig
		sess    *RemoteTranscoder
	)
	n.storageMutex.Lock()
	if storage, exists = n.StorageConfigs[sessionId]; exists {
		storage.OS.EndSession()
		storage.LocalOS.EndSession()
		delete(n.StorageConfigs, sessionId)
	}
	n.storageMutex.Unlock()
	// check to avoid nil pointer caused by garbage collection while this go routine is still running
	if n.TranscoderManager != nil {
		n.TranscoderManager.RTmutex.Lock()
		// send empty segment to signal transcoder internal session teardown if session exist
		if sess, exists = n.TranscoderManager.streamSessions[sessionId]; exists {
			segData := &net.SegData{
				AuthToken: &net.AuthToken{SessionId: sessionId},
			}
			msg := &net.NotifySegment{
				SegData: segData,
			}
			_ = sess.stream.Send(msg)
		}
		n.TranscoderManager.completeStreamSession(sessionId)
		n.TranscoderManager.RTmutex.Unlock()
	}
	n.segmentMutex.Lock()
	mid := ManifestID(sessionId)
	if _, exists = n.SegmentChans[mid]; exists {
		close(n.SegmentChans[mid])
		delete(n.SegmentChans, mid)
		if lpmon.Enabled {
			lpmon.CurrentSessions(len(n.SegmentChans))
		}
	}
	n.segmentMutex.Unlock()
	if exists {
		clog.V(common.DEBUG).Infof(logCtx, "Transcoding session ended by the Broadcaster for sessionID=%v", sessionId)
	}
}

func (n *LivepeerNode) serveTranscoder(stream net.Transcoder_RegisterTranscoderServer, capacity int, capabilities *net.Capabilities) {
	from := common.GetConnectionAddr(stream.Context())
	coreCaps := CapabilitiesFromNetCapabilities(capabilities)
	n.Capabilities.AddCapacity(coreCaps)
	defer n.Capabilities.RemoveCapacity(coreCaps)

	if n.AutoSessionLimit {
		n.SetMaxSessions(n.GetCurrentCapacity() + capacity)
	}

	// Manage blocks while transcoder is connected
	n.TranscoderManager.Manage(stream, capacity, capabilities)
	glog.V(common.DEBUG).Infof("Closing transcoder=%s channel", from)

	if n.AutoSessionLimit {
		defer n.SetMaxSessions(n.GetCurrentCapacity())
	}
}

func (rtm *RemoteTranscoderManager) transcoderResults(tcID int64, res *RemoteTranscoderResult) {
	remoteChan, err := rtm.getTaskChan(tcID)
	if err != nil {
		return // do we need to return anything?
	}
	remoteChan <- res
}

type RemoteTranscoder struct {
	manager      *RemoteTranscoderManager
	stream       net.Transcoder_RegisterTranscoderServer
	capabilities *Capabilities
	eof          chan struct{}
	addr         string
	capacity     int
	load         int
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

var ErrRemoteTranscoderTimeout = errors.New("Remote transcoder took too long")
var ErrNoTranscodersAvailable = errors.New("no transcoders available")
var ErrNoCompatibleTranscodersAvailable = errors.New("no transcoders can provide requested capabilities")

func (rt *RemoteTranscoder) done() {
	// select so we don't block indefinitely if there's no listener
	select {
	case rt.eof <- struct{}{}:
	default:
	}
}

// Transcode do actual transcoding by sending work to remote transcoder and waiting for the result
func (rt *RemoteTranscoder) Transcode(logCtx context.Context, md *SegTranscodingMetadata) (*TranscodeData, error) {
	taskID, taskChan := rt.manager.addTaskChan()
	defer rt.manager.removeTaskChan(taskID)
	fname := md.Fname
	signalEOF := func(err error) (*TranscodeData, error) {
		rt.done()
		clog.Errorf(logCtx, "Fatal error with remote transcoder=%s taskId=%d fname=%s err=%q", rt.addr, taskID, fname, err)
		return nil, RemoteTranscoderFatalError{err}
	}

	// Copy and remove some fields to minimize unneeded transfer
	mdCopy := *md
	mdCopy.OS = nil // remote transcoders currently upload directly back to O
	mdCopy.Hash = ethcommon.Hash{}
	segData, err := NetSegData(&mdCopy)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	msg := &net.NotifySegment{
		Url:     fname,
		TaskId:  taskID,
		SegData: segData,
		// Triggers failure on Os that don't know how to use SegData
		Profiles: []byte("invalid"),
	}
	err = rt.stream.Send(msg)

	if err != nil {
		return signalEOF(err)
	}

	// set a minimum timeout to accommodate transport / processing overhead
	dur := common.HTTPTimeout
	paddedDur := 4.0 * md.Duration // use a multiplier of 4 for now
	if paddedDur > dur {
		dur = paddedDur
	}

	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()
	select {
	case <-ctx.Done():
		return signalEOF(ErrRemoteTranscoderTimeout)
	case chanData := <-taskChan:
		segmentLen := 0
		if chanData.TranscodeData != nil {
			segmentLen = len(chanData.TranscodeData.Segments)
		}
		clog.InfofErr(logCtx, "Successfully received results from remote transcoder=%s segments=%d taskId=%d fname=%s dur=%v",
			rt.addr, segmentLen, taskID, fname, time.Since(start), chanData.Err)
		return chanData.TranscodeData, chanData.Err
	}
}
func NewRemoteTranscoder(m *RemoteTranscoderManager, stream net.Transcoder_RegisterTranscoderServer, capacity int, caps *Capabilities) *RemoteTranscoder {
	return &RemoteTranscoder{
		manager:      m,
		stream:       stream,
		eof:          make(chan struct{}, 1),
		capacity:     capacity,
		addr:         common.GetConnectionAddr(stream.Context()),
		capabilities: caps,
	}
}

func NewRemoteTranscoderManager() *RemoteTranscoderManager {
	return &RemoteTranscoderManager{
		remoteTranscoders: []*RemoteTranscoder{},
		liveTranscoders:   map[net.Transcoder_RegisterTranscoderServer]*RemoteTranscoder{},
		RTmutex:           sync.Mutex{},

		taskMutex: &sync.RWMutex{},
		taskChans: make(map[int64]TranscoderChan),

		streamSessions: make(map[string]*RemoteTranscoder),
	}
}

type byLoadFactor []*RemoteTranscoder

func loadFactor(r *RemoteTranscoder) float64 {
	return float64(r.load) / float64(r.capacity)
}

func (r byLoadFactor) Len() int      { return len(r) }
func (r byLoadFactor) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r byLoadFactor) Less(i, j int) bool {
	return loadFactor(r[j]) < loadFactor(r[i]) // sort descending
}

type RemoteTranscoderManager struct {
	remoteTranscoders []*RemoteTranscoder
	liveTranscoders   map[net.Transcoder_RegisterTranscoderServer]*RemoteTranscoder
	RTmutex           sync.Mutex

	// For tracking tasks assigned to remote transcoders
	taskMutex *sync.RWMutex
	taskChans map[int64]TranscoderChan
	taskCount int64

	// Map for keeping track of sessions and their respective transcoders
	streamSessions map[string]*RemoteTranscoder
}

// RegisteredTranscodersCount returns number of registered transcoders
func (rtm *RemoteTranscoderManager) RegisteredTranscodersCount() int {
	rtm.RTmutex.Lock()
	defer rtm.RTmutex.Unlock()
	return len(rtm.liveTranscoders)
}

// RegisteredTranscodersInfo returns list of restered transcoder's information
func (rtm *RemoteTranscoderManager) RegisteredTranscodersInfo() []common.RemoteTranscoderInfo {
	rtm.RTmutex.Lock()
	res := make([]common.RemoteTranscoderInfo, 0, len(rtm.liveTranscoders))
	for _, transcoder := range rtm.liveTranscoders {
		res = append(res, common.RemoteTranscoderInfo{Address: transcoder.addr, Capacity: transcoder.capacity})
	}
	rtm.RTmutex.Unlock()
	return res
}

// Manage adds transcoder to list of live transcoders. Doesn't return until transcoder disconnects
func (rtm *RemoteTranscoderManager) Manage(stream net.Transcoder_RegisterTranscoderServer, capacity int, capabilities *net.Capabilities) {
	from := common.GetConnectionAddr(stream.Context())
	transcoder := NewRemoteTranscoder(rtm, stream, capacity, CapabilitiesFromNetCapabilities(capabilities))
	go func() {
		ctx := stream.Context()
		<-ctx.Done()
		err := ctx.Err()
		glog.Errorf("Stream closed for transcoder=%s, err=%q", from, err)
		transcoder.done()
	}()

	rtm.RTmutex.Lock()
	rtm.liveTranscoders[transcoder.stream] = transcoder
	rtm.remoteTranscoders = append(rtm.remoteTranscoders, transcoder)
	sort.Sort(byLoadFactor(rtm.remoteTranscoders))
	var totalLoad, totalCapacity, liveTranscodersNum int
	if monitor.Enabled {
		totalLoad, totalCapacity, liveTranscodersNum = rtm.totalLoadAndCapacity()
	}
	rtm.RTmutex.Unlock()
	if monitor.Enabled {
		monitor.SetTranscodersNumberAndLoad(totalLoad, totalCapacity, liveTranscodersNum)
	}

	<-transcoder.eof
	glog.Infof("Got transcoder=%s eof, removing from live transcoders map", from)

	rtm.RTmutex.Lock()
	delete(rtm.liveTranscoders, transcoder.stream)
	if monitor.Enabled {
		totalLoad, totalCapacity, liveTranscodersNum = rtm.totalLoadAndCapacity()
	}
	rtm.RTmutex.Unlock()
	if monitor.Enabled {
		monitor.SetTranscodersNumberAndLoad(totalLoad, totalCapacity, liveTranscodersNum)
	}
}

func removeFromRemoteTranscoders(rt *RemoteTranscoder, remoteTranscoders []*RemoteTranscoder) []*RemoteTranscoder {
	if len(remoteTranscoders) == 0 {
		// No transcoders to remove, return
		return remoteTranscoders
	}

	newRemoteTs := make([]*RemoteTranscoder, 0)
	for _, t := range remoteTranscoders {
		if t != rt {
			newRemoteTs = append(newRemoteTs, t)
		}
	}
	return newRemoteTs
}

func (rtm *RemoteTranscoderManager) selectTranscoder(sessionId string, caps *Capabilities) (*RemoteTranscoder, error) {
	rtm.RTmutex.Lock()
	defer rtm.RTmutex.Unlock()

	checkTranscoders := func(rtm *RemoteTranscoderManager) bool {
		return len(rtm.remoteTranscoders) > 0
	}

	findCompatibleTranscoder := func(rtm *RemoteTranscoderManager) int {
		for i := len(rtm.remoteTranscoders) - 1; i >= 0; i-- {
			// no capabilities = default capabilities, all transcoders must support them
			if caps == nil || caps.bitstring.CompatibleWith(rtm.remoteTranscoders[i].capabilities.bitstring) {
				return i
			}
		}
		return -1
	}

	for checkTranscoders(rtm) {
		currentTranscoder, sessionExists := rtm.streamSessions[sessionId]
		lastCompatibleTranscoder := findCompatibleTranscoder(rtm)
		if lastCompatibleTranscoder == -1 {
			return nil, ErrNoCompatibleTranscodersAvailable
		}
		if !sessionExists {
			currentTranscoder = rtm.remoteTranscoders[lastCompatibleTranscoder]
		}

		if _, ok := rtm.liveTranscoders[currentTranscoder.stream]; !ok {
			// Remove the stream session because the transcoder is no longer live
			if sessionExists {
				rtm.completeStreamSession(sessionId)
			}
			// transcoder does not exist in table; remove and retry
			rtm.remoteTranscoders = removeFromRemoteTranscoders(currentTranscoder, rtm.remoteTranscoders)
			continue
		}
		if !sessionExists {
			if currentTranscoder.load == currentTranscoder.capacity {
				// Head of queue is at capacity, so the rest must be too. Exit early
				return nil, ErrNoTranscodersAvailable
			}

			// Assinging transcoder to session for future use
			rtm.streamSessions[sessionId] = currentTranscoder
			currentTranscoder.load++
			sort.Sort(byLoadFactor(rtm.remoteTranscoders))
		}
		return currentTranscoder, nil
	}

	return nil, ErrNoTranscodersAvailable
}

// ends transcoding session and releases resources
func (node *LivepeerNode) EndTranscodingSession(sessionId string) {
	node.endTranscodingSession(sessionId, context.TODO())
}

func (node *RemoteTranscoderManager) EndTranscodingSession(sessionId string) {
	panic("shouldn't be called on RemoteTranscoderManager")
}

// completeStreamSessions end a stream session for a remote transcoder and decrements its load
// caller should hold the mutex lock
func (rtm *RemoteTranscoderManager) completeStreamSession(sessionId string) {
	t, ok := rtm.streamSessions[sessionId]
	if !ok {
		return
	}
	t.load--
	sort.Sort(byLoadFactor(rtm.remoteTranscoders))
	delete(rtm.streamSessions, sessionId)
}

// Caller of this function should hold RTmutex lock
func (rtm *RemoteTranscoderManager) totalLoadAndCapacity() (int, int, int) {
	var load, capacity int
	for _, t := range rtm.liveTranscoders {
		load += t.load
		capacity += t.capacity
	}
	return load, capacity, len(rtm.liveTranscoders)
}

// Transcode does actual transcoding using remote transcoder from the pool
func (rtm *RemoteTranscoderManager) Transcode(ctx context.Context, md *SegTranscodingMetadata) (*TranscodeData, error) {
	currentTranscoder, err := rtm.selectTranscoder(md.AuthToken.SessionId, md.Caps)
	if err != nil {
		return nil, err
	}
	res, err := currentTranscoder.Transcode(ctx, md)
	if err != nil {
		rtm.RTmutex.Lock()
		rtm.completeStreamSession(md.AuthToken.SessionId)
		rtm.RTmutex.Unlock()
	}
	_, fatal := err.(RemoteTranscoderFatalError)
	if fatal {
		// Don't retry if we've timed out; broadcaster likely to have moved on
		// XXX problematic for VOD when we *should* retry
		if err.(RemoteTranscoderFatalError).error == ErrRemoteTranscoderTimeout {
			return res, err
		}
		return rtm.Transcode(ctx, md)
	}
	return res, err
}
