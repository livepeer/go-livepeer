package server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/ai/runner"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type testEthClient struct {
	*eth.StubClient
	key  *ecdsa.PrivateKey
	addr ethcommon.Address
}

type apiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func newTestEthClient(t *testing.T) *testEthClient {
	t.Helper()

	key, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	require.NoError(t, err)

	addr := crypto.PubkeyToAddress(key.PublicKey)
	return &testEthClient{
		StubClient: &eth.StubClient{TranscoderAddress: addr},
		key:        key,
		addr:       addr,
	}
}

func (c *testEthClient) Account() accounts.Account {
	return accounts.Account{Address: c.addr}
}

func (c *testEthClient) Sign(msg []byte) ([]byte, error) {
	sig, err := crypto.Sign(accounts.TextHash(msg), c.key)
	if err != nil {
		return nil, err
	}
	sig[64] += 27
	return sig, nil
}

func (c *testEthClient) SignTypedData(apitypes.TypedData) ([]byte, error) {
	return []byte("stub"), nil
}

func TestGenerateLivePayment_RequestValidationErrors(t *testing.T) {
	require := require.New(t)

	// Set default max price for all tests
	maxPrice := big.NewRat(500, 1) // 500 wei per pixel
	autoPrice, err := core.NewAutoConvertedPrice("", maxPrice, nil)
	require.NoError(err)
	BroadcastCfg.SetMaxPrice(autoPrice)
	defer BroadcastCfg.SetMaxPrice(nil) // Clean up

	// Set a capability-specific price (lower than max price)
	capability1 := core.Capability_LiveVideoToVideo
	modelID1 := "livepeer/model1"
	capMaxPrice := core.NewFixedPrice(big.NewRat(200, 1))
	BroadcastCfg.SetCapabilityMaxPrice(capability1, modelID1, capMaxPrice)
	defer BroadcastCfg.SetCapabilityMaxPrice(capability1, modelID1, nil) // Clean up

	baseOrchInfo := &net.OrchestratorInfo{
		Address:    ethcommon.HexToAddress("0x1").Bytes(),
		Transcoder: "http://orch.example",
		PriceInfo:  &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1},
		TicketParams: &net.TicketParams{
			Recipient: pm.RandAddress().Bytes(),
		},
		AuthToken: stubAuthToken,
	}

	makeOrchBlob := func(oInfo *net.OrchestratorInfo) []byte {
		b, err := proto.Marshal(oInfo)
		require.NoError(err)
		return b
	}

	baseReq := func() RemotePaymentRequest {
		return RemotePaymentRequest{
			Orchestrator: makeOrchBlob(baseOrchInfo),
			InPixels:     1,
		}
	}

	tests := []struct {
		name       string
		req        RemotePaymentRequest
		rawBody    []byte
		sender     *pm.MockSender
		wantStatus int
		wantMsg    string
		wantHeader string
	}{
		{
			name:       "invalid JSON",
			rawBody:    []byte("{not-json"),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid character",
		},
		{
			name:       "missing orchestrator blob",
			req:        RemotePaymentRequest{InPixels: 1},
			wantStatus: http.StatusBadRequest,
			wantMsg:    "missing orchestrator",
		},
		{
			name: "orchestrator blob not protobuf",
			req: RemotePaymentRequest{
				Orchestrator: []byte("not-proto"),
				InPixels:     1,
			},
			wantStatus: http.StatusBadRequest,
			wantMsg:    "proto:",
		},
		{
			name: "missing price info",
			req: func() RemotePaymentRequest {
				oInfo := proto.Clone(baseOrchInfo).(*net.OrchestratorInfo)
				oInfo.PriceInfo = nil
				return RemotePaymentRequest{Orchestrator: makeOrchBlob(oInfo), InPixels: 1}
			}(),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "missing or zero priceInfo",
		},
		{
			name: "zero price per unit",
			req: func() RemotePaymentRequest {
				oInfo := proto.Clone(baseOrchInfo).(*net.OrchestratorInfo)
				oInfo.PriceInfo = &net.PriceInfo{PricePerUnit: 0, PixelsPerUnit: 1}
				return RemotePaymentRequest{Orchestrator: makeOrchBlob(oInfo), InPixels: 1}
			}(),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "missing or zero priceInfo",
		},
		{
			name: "zero pixels per unit",
			req: func() RemotePaymentRequest {
				oInfo := proto.Clone(baseOrchInfo).(*net.OrchestratorInfo)
				oInfo.PriceInfo = &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 0}
				return RemotePaymentRequest{Orchestrator: makeOrchBlob(oInfo), InPixels: 1}
			}(),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "missing or zero priceInfo",
		},
		{
			name: "missing ticket params",
			req: func() RemotePaymentRequest {
				oInfo := proto.Clone(baseOrchInfo).(*net.OrchestratorInfo)
				oInfo.TicketParams = nil
				return RemotePaymentRequest{Orchestrator: makeOrchBlob(oInfo), InPixels: 1}
			}(),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "missing ticketParams in OrchestratorInfo",
		},
		{
			name: "missing auth token",
			req: func() RemotePaymentRequest {
				oInfo := proto.Clone(baseOrchInfo).(*net.OrchestratorInfo)
				oInfo.AuthToken = nil
				return RemotePaymentRequest{Orchestrator: makeOrchBlob(oInfo), InPixels: 1}
			}(),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "remote signer could not check whether to refresh session",
		},
		{
			name: "auth token expired triggers 480",
			req: func() RemotePaymentRequest {
				oInfo := proto.Clone(baseOrchInfo).(*net.OrchestratorInfo)
				oInfo.AuthToken = &net.AuthToken{
					Token:      []byte("tok"),
					SessionId:  "sess",
					Expiration: time.Now().Add(-1 * time.Minute).Unix(),
				}
				return RemotePaymentRequest{Orchestrator: makeOrchBlob(oInfo), InPixels: 1}
			}(),
			wantStatus: HTTPStatusRefreshSession,
			wantMsg:    "refresh session for remote signer",
			wantHeader: "http://orch.example",
		},
		{
			name: "ticket params expired triggers 480",
			req: func() RemotePaymentRequest {
				r := baseReq()
				r.InPixels = 1
				return r
			}(),
			sender:     newMockSender(mockSenderConfig{validateErr: pm.ErrTicketParamsExpired}),
			wantStatus: HTTPStatusRefreshSession,
			wantMsg:    "refresh session for remote signer",
			wantHeader: "http://orch.example",
		},
		{
			name: "invalid job type",
			req: func() RemotePaymentRequest {
				r := baseReq()
				r.Type = "bogus"
				return r
			}(),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid job type",
		},
		{
			name: "missing pixels without type",
			req: func() RemotePaymentRequest {
				r := baseReq()
				r.InPixels = 0
				return r
			}(),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "missing pixels or job type",
		},
		{
			name: "num tickets exceeds limit",
			req: func() RemotePaymentRequest {
				r := baseReq()
				r.InPixels = 101
				return r
			}(),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "exceeds maximum of 100",
		},
		{
			name: "invalid capabilities protobuf",
			req: func() RemotePaymentRequest {
				r := baseReq()
				r.Type = RemoteType_LiveVideoToVideo
				r.Capabilities = []byte("not-valid-protobuf")
				return r
			}(),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "cannot parse invalid wire-format data",
		},
		{
			name: "orchestrator price exceeds max price",
			req: func() RemotePaymentRequest {
				oInfo := proto.Clone(baseOrchInfo).(*net.OrchestratorInfo)
				// Set orchestrator price to 1000 wei per pixel (will exceed max price of 500)
				oInfo.PriceInfo = &net.PriceInfo{PricePerUnit: 1000, PixelsPerUnit: 1}
				return RemotePaymentRequest{
					Orchestrator: makeOrchBlob(oInfo),
					InPixels:     1,
				}
			}(),
			wantStatus: HTTPStatusPriceExceeded,
			wantMsg:    "orchestrator price",
		},
		{
			name: "orchestrator price exceeds per-capability max price",
			req: func() RemotePaymentRequest {
				oInfo := proto.Clone(baseOrchInfo).(*net.OrchestratorInfo)
				// Set orchestrator price above per-capability max (200) but below global max (500)
				oInfo.PriceInfo = &net.PriceInfo{PricePerUnit: 300, PixelsPerUnit: 1}
				constraints := core.NewCapabilities(nil, nil)
				caps := newAICapabilities(capability1, modelID1, true, constraints).ToNetCapabilities()
				capsBlob, err := proto.Marshal(caps)
				require.NoError(err)
				return RemotePaymentRequest{
					Orchestrator: makeOrchBlob(oInfo),
					InPixels:     1,
					Capabilities: capsBlob,
				}
			}(),
			wantStatus: HTTPStatusPriceExceeded,
			wantMsg:    "orchestrator price",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, _ := core.NewLivepeerNode(nil, "", nil)
			node.Balances = core.NewAddressBalances(1 * time.Minute)
			if tt.sender != nil {
				node.Sender = tt.sender
			} else {
				node.Sender = newMockSender(mockSenderConfig{})
			}
			ls := &LivepeerServer{LivepeerNode: node}

			body := tt.rawBody
			if body == nil {
				var err error
				body, err = json.Marshal(tt.req)
				require.NoError(err)
			}

			req := httptest.NewRequest(http.MethodPost, "/generate-live-payment", bytes.NewReader(body))
			rr := httptest.NewRecorder()

			ls.GenerateLivePayment(rr, req)

			require.Equal(tt.wantStatus, rr.Code)
			var apiErr apiErrorResponse
			require.NoError(json.NewDecoder(rr.Body).Decode(&apiErr))
			require.NotEmpty(apiErr.Error.Message)
			require.Contains(apiErr.Error.Message, tt.wantMsg)
			if tt.wantHeader != "" {
				require.Equal(tt.wantHeader, rr.Header().Get(RefreshSessionOrchestratorURLHeader))
			}
		})
	}
}

func TestGenerateLivePayment_StateValidationErrors(t *testing.T) {
	require := require.New(t)

	key, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	require.NoError(err)
	addr := crypto.PubkeyToAddress(key.PublicKey)

	node, _ := core.NewLivepeerNode(&eth.StubClient{TranscoderAddress: addr}, "", nil)
	node.Balances = core.NewAddressBalances(1 * time.Minute)
	node.Sender = newMockSender(mockSenderConfig{})
	ls := &LivepeerServer{LivepeerNode: node}

	orchInfo := &net.OrchestratorInfo{
		Address:   addr.Bytes(),
		PriceInfo: &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1},
		TicketParams: &net.TicketParams{
			Recipient: pm.RandAddress().Bytes(),
		},
		AuthToken: stubAuthToken,
	}

	sign := func(msg []byte) []byte {
		sig, err := crypto.Sign(accounts.TextHash(msg), key)
		require.NoError(err)
		sig[64] += 27
		return sig
	}

	priceIncreaseStateBytes := func() []byte {
		stateBytes, err := json.Marshal(RemotePaymentState{
			StateID:              "state",
			OrchestratorAddress:  ethcommon.BytesToAddress(orchInfo.Address),
			InitialPricePerUnit:  100,
			InitialPixelsPerUnit: 1,
		})
		require.NoError(err)
		return stateBytes
	}()

	tests := []struct {
		name           string
		stateBytes     []byte
		stateSig       []byte
		orchInfo       *net.OrchestratorInfo
		omitManifestID bool
		wantStatus     int
		wantMsg        string
	}{
		{
			name: "missing manifest id with state",
			stateBytes: func() []byte {
				state, err := json.Marshal(RemotePaymentState{
					StateID:              "state",
					OrchestratorAddress:  ethcommon.BytesToAddress(orchInfo.Address),
					InitialPricePerUnit:  1,
					InitialPixelsPerUnit: 1,
				})
				require.NoError(err)
				return state
			}(),
			omitManifestID: true,
			wantStatus:     http.StatusBadRequest,
			wantMsg:        "missing manifestID",
		},
		{
			name:       "invalid state signature",
			stateBytes: []byte(`{"stateID":"state","orchestratorAddress":"0x1"}`),
			stateSig:   []byte("bad"),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid sig",
		},
		{
			name:       "invalid state json",
			stateBytes: []byte("not-json"),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid state",
		},
		{
			name: "orchestrator address mismatch",
			stateBytes: func() []byte {
				state, err := json.Marshal(RemotePaymentState{
					StateID:              "state",
					OrchestratorAddress:  ethcommon.HexToAddress("0x1"),
					InitialPixelsPerUnit: 1,
				})
				require.NoError(err)
				return state
			}(),
			orchInfo: func() *net.OrchestratorInfo {
				oInfo := proto.Clone(orchInfo).(*net.OrchestratorInfo)
				oInfo.Address = ethcommon.HexToAddress("0x2").Bytes()
				return oInfo
			}(),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "orchestratorAddress mismatch",
		},
		{
			name:       "orchestrator price increased more than 2x",
			stateBytes: priceIncreaseStateBytes,
			orchInfo: func() *net.OrchestratorInfo {
				oInfo := proto.Clone(orchInfo).(*net.OrchestratorInfo)
				oInfo.PriceInfo = &net.PriceInfo{PricePerUnit: 250, PixelsPerUnit: 1}
				return oInfo
			}(),
			wantStatus: HTTPStatusPriceExceeded,
			wantMsg:    "Orchestrator price has more than doubled",
		},
		{
			name: "zero tickets returns 482",
			stateBytes: func() []byte {
				stateBytes, err := json.Marshal(RemotePaymentState{
					StateID:             "state",
					OrchestratorAddress: ethcommon.BytesToAddress(orchInfo.Address),
					// Existing balance large enough so StageUpdate yields NumTickets == 0.
					Balance:              "1000",
					InitialPricePerUnit:  1,
					InitialPixelsPerUnit: 1,
				})
				require.NoError(err)
				return stateBytes
			}(),
			wantStatus: HTTPStatusNoTickets,
			wantMsg:    "no tickets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oInfo := tt.orchInfo
			if oInfo == nil {
				oInfo = orchInfo
			}
			orchBlob, err := proto.Marshal(oInfo)
			require.NoError(err)

			var manifestID string
			if !tt.omitManifestID {
				manifestID = "manifest"
			}
			stateSig := tt.stateSig
			if stateSig == nil {
				stateSig = sign(tt.stateBytes)
			}

			reqBody, err := json.Marshal(RemotePaymentRequest{
				Orchestrator: orchBlob,
				ManifestID:   manifestID,
				InPixels:     1,
				State:        RemotePaymentStateSig{State: tt.stateBytes, Sig: stateSig},
			})
			require.NoError(err)

			req := httptest.NewRequest(http.MethodPost, "/generate-live-payment", bytes.NewReader(reqBody))
			rr := httptest.NewRecorder()

			ls.GenerateLivePayment(rr, req)

			require.Equal(tt.wantStatus, rr.Code)
			var apiErr apiErrorResponse
			require.NoError(json.NewDecoder(rr.Body).Decode(&apiErr))
			require.NotEmpty(apiErr.Error.Message)
			require.Contains(apiErr.Error.Message, tt.wantMsg)
		})
	}
}

func TestGenerateLivePayment_LV2V_Succeeds(t *testing.T) {
	require := require.New(t)

	ethClient := newTestEthClient(t)
	node, _ := core.NewLivepeerNode(ethClient, "", nil)
	node.Balances = core.NewAddressBalances(1 * time.Minute)
	var totalTickets uint32
	sender := newMockSender(mockSenderConfig{
		ev: big.NewRat(35, 1),
		createTicketBatchFn: func(args mock.Arguments, batch *pm.TicketBatch) {
			size := args.Int(1)
			*batch = *defaultTicketBatch()
			baseSig := []byte(nil)
			if len(batch.SenderParams) > 0 && batch.SenderParams[0] != nil {
				baseSig = batch.SenderParams[0].Sig
			}
			batch.SenderParams = make([]*pm.TicketSenderParams, size)
			for i := 0; i < size; i++ {
				totalTickets++
				batch.SenderParams[i] = &pm.TicketSenderParams{
					SenderNonce: totalTickets,
					Sig:         baseSig,
				}
			}
		},
	})
	node.Sender = sender
	ls := &LivepeerServer{LivepeerNode: node}

	doPayment := func(reqPayload RemotePaymentRequest) (RemotePaymentResponse, net.Payment) {
		reqBody, err := json.Marshal(reqPayload)
		require.NoError(err)

		req := httptest.NewRequest(http.MethodPost, "/generate-live-payment", bytes.NewReader(reqBody))
		rr := httptest.NewRecorder()

		ls.GenerateLivePayment(rr, req)
		require.Equal(http.StatusOK, rr.Code)

		var resp RemotePaymentResponse
		require.NoError(json.NewDecoder(rr.Body).Decode(&resp))
		require.NotEmpty(resp.Payment)

		paymentBytes, err := base64.StdEncoding.DecodeString(resp.Payment)
		require.NoError(err)
		var payment net.Payment
		require.NoError(proto.Unmarshal(paymentBytes, &payment))

		return resp, payment
	}

	ev, err := sender.EV("pmSession")
	expectedBalance := func(oldBal *big.Rat, numTickets int, fee *big.Rat) *big.Rat {
		// expected = oldBal + (numTickets * EV) - fee(initialPrice)
		expected := new(big.Rat).Add(oldBal, new(big.Rat).Mul(new(big.Rat).SetInt64(int64(numTickets)), ev))
		expected.Sub(expected, fee)
		return expected
	}

	// Set a global max price; initial request price is below it.
	maxPrice := big.NewRat(200, 1)
	autoPrice, err := core.NewAutoConvertedPrice("", maxPrice, nil)
	require.NoError(err)
	BroadcastCfg.SetMaxPrice(autoPrice)
	defer BroadcastCfg.SetMaxPrice(nil)

	// Use small integers so the fee math is simple and deterministic.
	//
	// price = 150/175 wei per pixel = 6/7 wei per pixel
	// inPixels = 1000 => fee = 1000 * 6/7 = 6000/7 ~= 857.14 wei
	const inPixels int64 = 1000
	oInfo := &net.OrchestratorInfo{
		Address:   ethClient.addr.Bytes(),
		PriceInfo: &net.PriceInfo{PricePerUnit: 150, PixelsPerUnit: 175},
		TicketParams: &net.TicketParams{
			Recipient: pm.RandAddress().Bytes(),
		},
		AuthToken: stubAuthToken,
	}
	orchBlob, err := proto.Marshal(oInfo)
	require.NoError(err)
	const manifestID = "lv2v-manifest"

	resp, payment := doPayment(RemotePaymentRequest{
		Orchestrator: orchBlob,
		ManifestID:   manifestID,
		InPixels:     inPixels,
	})
	require.NotEmpty(resp.Payment)
	require.NotEmpty(resp.SegCreds)
	require.NotEmpty(resp.State.State)
	require.NotEmpty(resp.State.Sig)

	var state RemotePaymentState
	require.NoError(json.Unmarshal(resp.State.State, &state))
	require.NotEmpty(state.StateID)
	require.Equal(ethClient.addr, state.OrchestratorAddress)
	require.Equal("pmSession", state.PMSessionID)
	require.EqualValues(0, state.SequenceNumber)
	require.NotEmpty(state.Balance)
	require.False(state.LastUpdate.IsZero())

	_, err = base64.StdEncoding.DecodeString(resp.SegCreds)
	require.NoError(err)

	// Check that the initial price is still used even after a 1.5x increase
	var state1 RemotePaymentState
	require.NoError(json.Unmarshal(resp.State.State, &state1))
	oldBal := new(big.Rat)
	_, ok := oldBal.SetString(state1.Balance)
	require.True(ok, "failed to parse old balance: %q", state1.Balance)

	fee := new(big.Rat).Mul(new(big.Rat).SetFrac64(150, 175), new(big.Rat).SetInt64(inPixels)) // 6000/7
	require.NoError(err)
	expectedBal := expectedBalance(big.NewRat(0, 1), len(payment.TicketSenderParams), fee)
	require.Zero(oldBal.Cmp(expectedBal), "unexpected state balance: got=%s want=%s fee=%s tickets=%d ev=%s",
		oldBal.RatString(), expectedBal.RatString(), fee.RatString(), len(payment.TicketSenderParams), ev.RatString(),
	)

	updatedInfo := proto.Clone(oInfo).(*net.OrchestratorInfo)
	updatedInfo.PriceInfo = &net.PriceInfo{
		PricePerUnit:  oInfo.PriceInfo.PricePerUnit * 3 / 2, // 1.5x increase
		PixelsPerUnit: oInfo.PriceInfo.PixelsPerUnit,
	}
	orchBlob, err = proto.Marshal(updatedInfo)
	require.NoError(err)

	const inPixelsUpdated int64 = 2500
	resp2, payment2 := doPayment(RemotePaymentRequest{
		Orchestrator: orchBlob,
		ManifestID:   manifestID,
		InPixels:     inPixelsUpdated,
		State:        resp.State,
	})

	var stateFee2 RemotePaymentState
	require.NoError(json.Unmarshal(resp2.State.State, &stateFee2))
	require.EqualValues(1, stateFee2.SequenceNumber)
	newBal := new(big.Rat)
	_, ok = newBal.SetString(stateFee2.Balance)
	require.True(ok, "failed to parse new balance: %q", stateFee2.Balance)

	// Validate fee behavior via balance update.
	fee = new(big.Rat).Mul(new(big.Rat).SetFrac64(150, 175), new(big.Rat).SetInt64(inPixelsUpdated)) // 15000/7
	expectedNewBal := expectedBalance(oldBal, len(payment2.TicketSenderParams), fee)
	require.Zero(newBal.Cmp(expectedNewBal), "unexpected balance: got=%s want=%s fee=%s old=%s tickets=%d ev=%s",
		newBal.RatString(), expectedNewBal.RatString(), fee.RatString(), oldBal.RatString(), len(payment2.TicketSenderParams), ev.RatString(),
	)

	// Per-capability max price higher than global max should allow the request.
	cap := core.Capability_LiveVideoToVideo
	modelID := "livepeer/model-high"
	capMaxPrice := core.NewFixedPrice(big.NewRat(500, 1))
	BroadcastCfg.SetCapabilityMaxPrice(cap, modelID, capMaxPrice)
	defer BroadcastCfg.SetCapabilityMaxPrice(cap, modelID, nil)

	caps := newAICapabilities(cap, modelID, true, core.NewCapabilities(nil, nil)).ToNetCapabilities()
	capsBlob, err := proto.Marshal(caps)
	require.NoError(err)

	oInfo.PriceInfo = &net.PriceInfo{PricePerUnit: 300, PixelsPerUnit: 1}
	orchBlob, err = proto.Marshal(oInfo)
	require.NoError(err)

	capResp, capPayment := doPayment(RemotePaymentRequest{
		Orchestrator: orchBlob,
		InPixels:     1,
		Capabilities: capsBlob,
	})

	var capState RemotePaymentState
	require.NoError(json.Unmarshal(capResp.State.State, &capState))
	capBal := new(big.Rat)
	_, ok = capBal.SetString(capState.Balance)
	require.True(ok, "failed to parse cap balance: %q", capState.Balance)

	capFee := new(big.Rat).Mul(new(big.Rat).SetFrac64(300, 1), new(big.Rat).SetInt64(1))
	expectedCapBal := expectedBalance(big.NewRat(0, 1), len(capPayment.TicketSenderParams), capFee)
	require.Zero(capBal.Cmp(expectedCapBal), "unexpected cap balance: got=%s want=%s fee=%s tickets=%d ev=%s",
		capBal.RatString(), expectedCapBal.RatString(), capFee.RatString(), len(capPayment.TicketSenderParams), ev.RatString(),
	)

}

func TestGenerateLivePayment_WebhookCallback(t *testing.T) {
	require := require.New(t)

	ethClient := newTestEthClient(t)
	node, _ := core.NewLivepeerNode(ethClient, "", nil)
	node.Balances = core.NewAddressBalances(1 * time.Minute)
	node.Sender = newMockSender(mockSenderConfig{})
	ls := &LivepeerServer{LivepeerNode: node}

	oInfo := &net.OrchestratorInfo{
		Address:   ethClient.addr.Bytes(),
		PriceInfo: &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1},
		TicketParams: &net.TicketParams{
			Recipient: pm.RandAddress().Bytes(),
		},
		AuthToken: stubAuthToken,
	}
	orchBlob, err := proto.Marshal(oInfo)
	require.NoError(err)

	doPaymentWithStateAndAuthID := func(requestHeader string, authID string, state RemotePaymentStateSig) *httptest.ResponseRecorder {
		reqBody, err := json.Marshal(RemotePaymentRequest{
			Orchestrator: orchBlob,
			ManifestID:   "manifest",
			InPixels:     1,
			State:        state,
		})
		require.NoError(err)

		req := httptest.NewRequest(http.MethodPost, "/generate-live-payment", bytes.NewReader(reqBody))
		req.Header.Set("X-Request-ID", requestHeader)
		if authID != "" {
			req.Header.Set(remoteSignerAuthIDHeader, authID)
		}
		rr := httptest.NewRecorder()
		ls.GenerateLivePayment(rr, req)
		return rr
	}
	doPaymentWithState := func(requestHeader string, state RemotePaymentStateSig) *httptest.ResponseRecorder {
		return doPaymentWithStateAndAuthID(requestHeader, "", state)
	}
	doPaymentWithAuthID := func(requestHeader string, authID string) *httptest.ResponseRecorder {
		return doPaymentWithStateAndAuthID(requestHeader, authID, RemotePaymentStateSig{})
	}
	doPayment := func(requestHeader string) *httptest.ResponseRecorder {
		return doPaymentWithState(requestHeader, RemotePaymentStateSig{})
	}
	parseResponseState := func(rr *httptest.ResponseRecorder) RemotePaymentState {
		var resp RemotePaymentResponse
		require.NoError(json.NewDecoder(rr.Body).Decode(&resp))
		var state RemotePaymentState
		require.NoError(json.Unmarshal(resp.State.State, &state))
		return state
	}

	t.Run("callback omitted succeeds", func(t *testing.T) {
		ls.LivepeerNode.RemoteSignerWebhookURL = nil
		ls.LivepeerNode.RemoteSignerWebhookHeaders = nil

		rr := doPayment("no-callback")
		require.Equal(http.StatusOK, rr.Code)
	})

	t.Run("callback receives request headers and outbound auth headers", func(t *testing.T) {
		callbackCalled := false
		var payload generateLivePaymentWebhookBody
		webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callbackCalled = true
			require.Equal("Bearer abc", r.Header.Get("Authorization"))
			require.Equal("secret", r.Header.Get("X-API-Key"))
			require.Equal("application/json", r.Header.Get("Content-Type"))
			require.NoError(json.NewDecoder(r.Body).Decode(&payload))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":200}`))
		}))
		defer webhook.Close()

		webhookURL, err := url.Parse(webhook.URL)
		require.NoError(err)
		ls.LivepeerNode.RemoteSignerWebhookURL = webhookURL
		ls.LivepeerNode.RemoteSignerWebhookHeaders = map[string]string{
			"Authorization": "Bearer abc",
			"X-API-Key":     "secret",
		}

		rr := doPayment("req-123")
		require.Equal(http.StatusOK, rr.Code)
		require.True(callbackCalled)
		require.Equal([]string{"req-123"}, payload.Headers["X-Request-Id"])
		require.NotNil(payload.State)
		require.Equal("pmSession", payload.State.PMSessionID)
	})

	t.Run("callback 200 with status 200 succeeds", func(t *testing.T) {
		webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":200}`))
		}))
		defer webhook.Close()

		webhookURL, err := url.Parse(webhook.URL)
		require.NoError(err)
		ls.LivepeerNode.RemoteSignerWebhookURL = webhookURL
		ls.LivepeerNode.RemoteSignerWebhookHeaders = nil

		rr := doPayment("req-status-200")
		require.Equal(http.StatusOK, rr.Code)
	})

	t.Run("callback 200 with expiry sets state auth expiry", func(t *testing.T) {
		expiry := time.Now().Add(5 * time.Minute).Unix()
		webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"status":200,"expiry":%d}`, expiry)))
		}))
		defer webhook.Close()

		webhookURL, err := url.Parse(webhook.URL)
		require.NoError(err)
		ls.LivepeerNode.RemoteSignerWebhookURL = webhookURL
		ls.LivepeerNode.RemoteSignerWebhookHeaders = nil

		rr := doPayment("req-expiry-set")
		require.Equal(http.StatusOK, rr.Code)
		state := parseResponseState(rr)
		require.Equal(expiry, state.AuthExpiry)
	})

	t.Run("callback auth id sets state", func(t *testing.T) {
		webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":200,"auth_id":"webhook-auth-id"}`))
		}))
		defer webhook.Close()

		webhookURL, err := url.Parse(webhook.URL)
		require.NoError(err)
		ls.LivepeerNode.RemoteSignerWebhookURL = webhookURL
		ls.LivepeerNode.RemoteSignerWebhookHeaders = nil

		rr := doPayment("req-auth-id")
		require.Equal(http.StatusOK, rr.Code)
		state := parseResponseState(rr)
		require.Equal("webhook-auth-id", state.AuthID)
	})

	t.Run("request auth id header is fallback when callback omits auth id", func(t *testing.T) {
		webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":200}`))
		}))
		defer webhook.Close()

		webhookURL, err := url.Parse(webhook.URL)
		require.NoError(err)
		ls.LivepeerNode.RemoteSignerWebhookURL = webhookURL
		ls.LivepeerNode.RemoteSignerWebhookHeaders = nil

		rr := doPaymentWithAuthID("req-auth-id-fallback", "header-auth-id")
		require.Equal(http.StatusOK, rr.Code)
		state := parseResponseState(rr)
		require.Equal("header-auth-id", state.AuthID)
	})

	t.Run("callback auth id wins over request header", func(t *testing.T) {
		webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":200,"auth_id":"webhook-auth-id"}`))
		}))
		defer webhook.Close()

		webhookURL, err := url.Parse(webhook.URL)
		require.NoError(err)
		ls.LivepeerNode.RemoteSignerWebhookURL = webhookURL
		ls.LivepeerNode.RemoteSignerWebhookHeaders = nil

		rr := doPaymentWithAuthID("req-auth-id-priority", "header-auth-id")
		require.Equal(http.StatusOK, rr.Code)
		state := parseResponseState(rr)
		require.Equal("webhook-auth-id", state.AuthID)
	})

	t.Run("sequential requests skip callback while auth expiry still valid", func(t *testing.T) {
		callbackCalls := 0
		expiry := time.Now().Add(5 * time.Minute).Unix()
		webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callbackCalls++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"status":200,"expiry":%d,"auth_id":"cached-auth-id"}`, expiry)))
		}))
		defer webhook.Close()

		webhookURL, err := url.Parse(webhook.URL)
		require.NoError(err)
		ls.LivepeerNode.RemoteSignerWebhookURL = webhookURL
		ls.LivepeerNode.RemoteSignerWebhookHeaders = nil

		first := doPayment("req-skip-first")
		require.Equal(http.StatusOK, first.Code)

		var firstResp RemotePaymentResponse
		require.NoError(json.NewDecoder(first.Body).Decode(&firstResp))
		var firstState RemotePaymentState
		require.NoError(json.Unmarshal(firstResp.State.State, &firstState))
		require.Equal("cached-auth-id", firstState.AuthID)

		second := doPaymentWithState("req-skip-second", firstResp.State)
		require.Equal(http.StatusOK, second.Code)
		var secondResp RemotePaymentResponse
		require.NoError(json.NewDecoder(second.Body).Decode(&secondResp))
		var secondState RemotePaymentState
		require.NoError(json.Unmarshal(secondResp.State.State, &secondState))
		require.Equal("cached-auth-id", secondState.AuthID)
		require.Equal(1, callbackCalls)

		third := doPaymentWithStateAndAuthID("req-skip-third", "new-header-auth-id", secondResp.State)
		require.Equal(http.StatusInternalServerError, third.Code)
		var apiErr apiErrorResponse
		require.NoError(json.NewDecoder(third.Body).Decode(&apiErr))
		require.Equal("Internal Server Error", apiErr.Error.Message)
		require.Equal(1, callbackCalls)
	})

	t.Run("expired auth expiry triggers callback on next request", func(t *testing.T) {
		callbackCalls := 0
		webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callbackCalls++
			expiry := time.Now().Add(-1 * time.Minute).Unix()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"status":200,"expiry":%d}`, expiry)))
		}))
		defer webhook.Close()

		webhookURL, err := url.Parse(webhook.URL)
		require.NoError(err)
		ls.LivepeerNode.RemoteSignerWebhookURL = webhookURL
		ls.LivepeerNode.RemoteSignerWebhookHeaders = nil

		first := doPayment("req-expired-first")
		require.Equal(http.StatusOK, first.Code)

		var firstResp RemotePaymentResponse
		require.NoError(json.NewDecoder(first.Body).Decode(&firstResp))
		second := doPaymentWithState("req-expired-second", firstResp.State)
		require.Equal(http.StatusOK, second.Code)
		require.Equal(2, callbackCalls)
	})

	t.Run("callback auth id change returns 500", func(t *testing.T) {
		webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":200,"auth_id":"updated-auth-id"}`))
		}))
		defer webhook.Close()

		webhookURL, err := url.Parse(webhook.URL)
		require.NoError(err)
		ls.LivepeerNode.RemoteSignerWebhookURL = webhookURL
		ls.LivepeerNode.RemoteSignerWebhookHeaders = nil

		initialState := RemotePaymentState{
			StateID:              string(core.RandomManifestID()),
			OrchestratorAddress:  ethcommon.BytesToAddress(oInfo.Address),
			InitialPricePerUnit:  oInfo.PriceInfo.PricePerUnit,
			InitialPixelsPerUnit: oInfo.PriceInfo.PixelsPerUnit,
			AuthID:               "cached-auth-id",
		}
		stateBytes, err := json.Marshal(initialState)
		require.NoError(err)
		stateSig, err := signState(ls, stateBytes)
		require.NoError(err)

		rr := doPaymentWithState("req-auth-id-replaced", RemotePaymentStateSig{State: stateBytes, Sig: stateSig})
		require.Equal(http.StatusInternalServerError, rr.Code)
		var apiErr apiErrorResponse
		require.NoError(json.NewDecoder(rr.Body).Decode(&apiErr))
		require.Equal("Internal Server Error", apiErr.Error.Message)
	})

	t.Run("callback 200 missing status returns 500", func(t *testing.T) {
		webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"expiry":123}`))
		}))
		defer webhook.Close()

		webhookURL, err := url.Parse(webhook.URL)
		require.NoError(err)
		ls.LivepeerNode.RemoteSignerWebhookURL = webhookURL
		ls.LivepeerNode.RemoteSignerWebhookHeaders = nil

		rr := doPayment("req-missing-status")
		require.Equal(http.StatusInternalServerError, rr.Code)

		var apiErr apiErrorResponse
		require.NoError(json.NewDecoder(rr.Body).Decode(&apiErr))
		require.Equal("Internal Server Error", apiErr.Error.Message)
	})

	t.Run("callback 200 with rejection status returns reason", func(t *testing.T) {
		webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":403,"reason":"denied"}`))
		}))
		defer webhook.Close()

		webhookURL, err := url.Parse(webhook.URL)
		require.NoError(err)
		ls.LivepeerNode.RemoteSignerWebhookURL = webhookURL
		ls.LivepeerNode.RemoteSignerWebhookHeaders = nil

		rr := doPayment("req-rejected")
		require.Equal(http.StatusForbidden, rr.Code)

		var apiErr apiErrorResponse
		require.NoError(json.NewDecoder(rr.Body).Decode(&apiErr))
		require.Contains(apiErr.Error.Message, "denied")
	})

	t.Run("callback 200 with rejection status and no reason uses fallback", func(t *testing.T) {
		webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":429}`))
		}))
		defer webhook.Close()

		webhookURL, err := url.Parse(webhook.URL)
		require.NoError(err)
		ls.LivepeerNode.RemoteSignerWebhookURL = webhookURL
		ls.LivepeerNode.RemoteSignerWebhookHeaders = nil

		rr := doPayment("req-no-reason")
		require.Equal(http.StatusTooManyRequests, rr.Code)

		var apiErr apiErrorResponse
		require.NoError(json.NewDecoder(rr.Body).Decode(&apiErr))
		require.Contains(apiErr.Error.Message, "signer auth rejected request with status 429")
	})

	t.Run("callback HTTP non-200 returns 500", func(t *testing.T) {
		webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`denied`))
		}))
		defer webhook.Close()

		webhookURL, err := url.Parse(webhook.URL)
		require.NoError(err)
		ls.LivepeerNode.RemoteSignerWebhookURL = webhookURL
		ls.LivepeerNode.RemoteSignerWebhookHeaders = nil

		rr := doPayment("req-401")
		require.Equal(http.StatusInternalServerError, rr.Code)

		var apiErr apiErrorResponse
		require.NoError(json.NewDecoder(rr.Body).Decode(&apiErr))
		require.Equal("Internal Server Error", apiErr.Error.Message)
	})
}

func TestRemoteSigner_Discovery(t *testing.T) {
	require := require.New(t)

	BroadcastCfg.SetCapabilityMaxPrice(core.Capability_LiveVideoToVideo, "model-a", core.NewFixedPrice(big.NewRat(100, 1)))
	defer BroadcastCfg.SetCapabilityMaxPrice(core.Capability_LiveVideoToVideo, "model-a", nil)

	orch1Caps := &net.Capabilities{
		Constraints: &net.Capabilities_Constraints{
			PerCapability: map[uint32]*net.Capabilities_CapabilityConstraints{
				uint32(core.Capability_LiveVideoToVideo): {
					Models: map[string]*net.Capabilities_CapabilityConstraints_ModelConstraint{
						"model-a": {},
					},
				},
			},
		},
	}
	orch2Caps := &net.Capabilities{
		Constraints: &net.Capabilities_Constraints{
			PerCapability: map[uint32]*net.Capabilities_CapabilityConstraints{
				uint32(core.Capability_LiveVideoToVideo): {
					Models: map[string]*net.Capabilities_CapabilityConstraints_ModelConstraint{
						"model-a": {},
					},
				},
				uint32(core.Capability_TextToImage): {
					Models: map[string]*net.Capabilities_CapabilityConstraints_ModelConstraint{
						"model-b": {},
					},
				},
			},
		},
	}

	// Remote discovery refresh reads from the node's network capability cache.
	//
	// Each orchestrator below has a specific purpose:
	// - orch1: Has a single eligible capability priced below the configured max price.
	// - orch2: Has two capabilities, but only one is eligible (the other is filtered out).
	// - orch3: Has capabilities priced above the max price, so it should be dropped entirely.
	// - invalid: Has an invalid URI and should never be returned from discovery.
	//
	// Note: Prices are returned as `CapabilitiesPrices` (capability menu pricing), not via global
	// `PriceInfo` (which is intentionally expensive in these fixtures).
	node := &core.LivepeerNode{}
	require.NoError(node.UpdateNetworkCapabilities([]*common.OrchNetworkCapabilities{
		{
			OrchURI:      "https://orch1.example.com:8935",
			Capabilities: orch1Caps,
			// Global fallback price is intentionally expensive.
			PriceInfo: &net.PriceInfo{PricePerUnit: 200, PixelsPerUnit: 1},
			CapabilitiesPrices: []*net.PriceInfo{
				{
					PricePerUnit:  80,
					PixelsPerUnit: 1,
					Capability:    uint32(core.Capability_LiveVideoToVideo),
					Constraint:    "model-a",
				},
			},
			Discovery: discoveryRaw(t, `[
				{
					"address": "https://orch1.example.com:8935/",
					"score": 99,
					"capabilities": ["evil/app"],
					"unknown_field": "must-not-leak",
					"runners": [
						{"url":"https://orch1.example.com:8935/scope-ok","app":"live-video-to-video/model-a","price_info":{"price_per_unit":80,"pixels_per_unit":1,"unit":"WEI"}},
						{"url":"https://orch1.example.com:8935/scope-missing-price","app":"live-video-to-video/model-a"},
						{"url":"https://orch1.example.com:8935/scope-too-expensive","app":"live-video-to-video/model-a","price_info":{"price_per_unit":120,"pixels_per_unit":1,"unit":"WEI"}},
						{"url":"https://orch1.example.com:8935/scope-usd","app":"live-video-to-video/model-a","price_info":{"price_per_unit":1,"pixels_per_unit":1,"unit":"USD"}},
						{"url":"https://orch1.example.com:8935/scope-invalid-price","app":"live-video-to-video/model-a","price_info":{"price_per_unit":1,"pixels_per_unit":0,"unit":"WEI"}},
						{"url":"https://orch1.example.com:8935/scope-missing-unit","app":"live-video-to-video/model-a","price_info":{"price_per_unit":1,"pixels_per_unit":1}},
						{"url":"https://orch1.example.com:8935/txt","app":"text-to-image/model-b","price_info":{"price_per_unit":1,"pixels_per_unit":1,"unit":"WEI"}}
					]
				},
				{
					"address": "https://discovered.example.com:8935",
					"runners": [
						{"url":"https://discovered.example.com:8935/scope","app":"live-video-to-video/model-a","capacity":3,"price_info":{"price_per_unit":80,"pixels_per_unit":1,"unit":"WEI"}},
						{"url":"https://discovered.example.com:8935/dupe","app":"live-video-to-video/model-a","capacity":1,"price_info":{"price_per_unit":80,"pixels_per_unit":1,"unit":"WEI"}},
						{"url":"https://discovered.example.com:8935/missing-price","app":"live-video-to-video/model-a"},
						{"url":"https://discovered.example.com:8935/too-expensive","app":"live-video-to-video/model-a","price_info":{"price_per_unit":120,"pixels_per_unit":1,"unit":"WEI"}}
					]
				},
				{
					"address": "https://discovered.example.com:8935/",
					"runners": [
						{"url":"https://discovered.example.com:8935/dupe","app":"live-video-to-video/model-a","capacity":1,"capacity_used":1,"capacity_available":0,"price_info":{"price_per_unit":80,"pixels_per_unit":1,"unit":"WEI"}},
						{"url":"https://discovered.example.com:8935/dupe","app":"live-video-to-video/model-a","capacity":2,"price_info":{"price_per_unit":80,"pixels_per_unit":1,"unit":"WEI"}}
					]
				}
			]`),
		},
		{
			OrchURI:      "https://orch2.example.com:8935",
			Capabilities: orch2Caps,
			// Global fallback price is intentionally expensive.
			PriceInfo: &net.PriceInfo{PricePerUnit: 200, PixelsPerUnit: 1},
			CapabilitiesPrices: []*net.PriceInfo{
				// This capability is above max price and should be filtered out.
				{
					PricePerUnit:  120,
					PixelsPerUnit: 1,
					Capability:    uint32(core.Capability_LiveVideoToVideo),
					Constraint:    "model-a",
				},
				// This capability remains eligible and should be exposed via discovery.
				{
					PricePerUnit:  50,
					PixelsPerUnit: 1,
					Capability:    uint32(core.Capability_TextToImage),
					Constraint:    "model-b",
				},
			},
			Discovery: discoveryRaw(t, `[
				{
					"address": "https://orch2.example.com:8935",
					"runners": [
						{"url":"https://orch2.example.com:8935/txt","app":"text-to-image/model-b","price_info":{"price_per_unit":999,"pixels_per_unit":1,"unit":"WEI"}},
						{"url":"https://orch2.example.com:8935/scope","app":"live-video-to-video/model-a","price_info":{"price_per_unit":1,"pixels_per_unit":1,"unit":"WEI"}}
					]
				},
				{
					"address": "https://discovered.example.com:8935",
					"runners": [
						{"url":"https://discovered.example.com:8935/from-orch2","app":"live-video-to-video/model-a","price_info":{"price_per_unit":80,"pixels_per_unit":1,"unit":"WEI"}}
					]
				}
			]`),
		},
		{
			OrchURI:      "https://orch3.example.com:8935",
			Capabilities: orch1Caps,
			// Global fallback price is intentionally expensive.
			PriceInfo: &net.PriceInfo{PricePerUnit: 200, PixelsPerUnit: 1},
			CapabilitiesPrices: []*net.PriceInfo{
				// All capabilities above max price, so the orchestrator should be dropped.
				{
					PricePerUnit:  200,
					PixelsPerUnit: 1,
					Capability:    uint32(core.Capability_LiveVideoToVideo),
					Constraint:    "model-a",
				},
			},
			Discovery: discoveryRaw(t, `[{
				"address": "https://orch3.example.com:8935",
				"runners": [
					{"url":"https://orch3.example.com:8935/scope","app":"live-video-to-video/model-a","price_info":{"price_per_unit":1,"pixels_per_unit":1,"unit":"WEI"}}
				]
			}]`),
		},
		// Invalid URI should be dropped during refresh and never returned from discovery.
		{
			OrchURI:      "://invalid-uri",
			Capabilities: orch1Caps,
			PriceInfo:    &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1},
		},
	}))

	rdp := &remoteDiscoveryPool{
		node:         node,
		refreshEvery: time.Hour,
	}
	rdp.refresh()
	ls := &LivepeerServer{}

	httpReq := httptest.NewRequest(http.MethodGet, "/discover-orchestrators", nil)
	rr := httptest.NewRecorder()

	ls.GetOrchestrators(rdp, rr, httpReq)

	require.Equal(http.StatusOK, rr.Code)
	require.Equal("application/json", rr.Header().Get("Content-Type"))

	body := rr.Body.Bytes()
	var resp []discoveryResponse
	require.NoError(json.Unmarshal(body, &resp))
	require.Len(resp, 3)
	orch1Resp := discoveryResponseByAddress(t, resp, "https://orch1.example.com:8935")
	require.Equal(float32(common.Score_Trusted), orch1Resp.Score)
	require.Equal([]string{"live-video-to-video/model-a"}, orch1Resp.Capabilities)
	require.Equal([]string{"live-video-to-video/model-a"}, discoveryRunnerApps(t, orch1Resp))
	requireNotContainsDiscoveryField(t, body, "unknown_field")
	discoveredResp := discoveryResponseByAddress(t, resp, "https://discovered.example.com:8935")
	require.Equal(float32(common.Score_Trusted), discoveredResp.Score)
	require.Equal([]string{"live-video-to-video/model-a"}, discoveredResp.Capabilities)
	require.Equal([]string{"live-video-to-video/model-a", "live-video-to-video/model-a", "live-video-to-video/model-a"}, discoveryRunnerApps(t, discoveredResp))
	require.Equal(1, discoveredResp.Runners[1].Capacity)
	require.Equal(0, discoveredResp.Runners[1].CapacityUsed)
	orch2Resp := discoveryResponseByAddress(t, resp, "https://orch2.example.com:8935")
	require.Equal(float32(common.Score_Trusted), orch2Resp.Score)
	require.Equal([]string{"text-to-image/model-b"}, orch2Resp.Capabilities)
	require.Equal([]string{"text-to-image/model-b"}, discoveryRunnerApps(t, orch2Resp))

	capsReq := httptest.NewRequest(http.MethodGet, "/discover-orchestrators?caps=live-video-to-video/model-a", nil)
	capsRR := httptest.NewRecorder()
	ls.GetOrchestrators(rdp, capsRR, capsReq)
	require.Equal(http.StatusOK, capsRR.Code)
	var capsResp []discoveryResponse
	require.NoError(json.NewDecoder(capsRR.Body).Decode(&capsResp))
	require.Len(capsResp, 2)
	require.Equal([]string{"live-video-to-video/model-a"}, discoveryRunnerApps(t, discoveryResponseByAddress(t, capsResp, "https://orch1.example.com:8935")))
	require.Equal([]string{"live-video-to-video/model-a", "live-video-to-video/model-a", "live-video-to-video/model-a"}, discoveryRunnerApps(t, discoveryResponseByAddress(t, capsResp, "https://discovered.example.com:8935")))

	repeatedReq := httptest.NewRequest(http.MethodGet, "/discover-orchestrators?caps=live-video-to-video/model-a&caps=text-to-image/model-b", nil)
	repeatedRR := httptest.NewRecorder()
	ls.GetOrchestrators(rdp, repeatedRR, repeatedReq)
	require.Equal(http.StatusOK, repeatedRR.Code)
	var repeatedResp []discoveryResponse
	require.NoError(json.NewDecoder(repeatedRR.Body).Decode(&repeatedResp))
	require.Len(repeatedResp, 3)
	discoveryResponseByAddress(t, repeatedResp, "https://orch1.example.com:8935")
	discoveryResponseByAddress(t, repeatedResp, "https://discovered.example.com:8935")
	discoveryResponseByAddress(t, repeatedResp, "https://orch2.example.com:8935")

	// If refresh only receives invalid or ineligible network capability entries,
	// cache should remain empty and discovery should return service unavailable.
	ineligibleNode := &core.LivepeerNode{}
	require.NoError(ineligibleNode.UpdateNetworkCapabilities([]*common.OrchNetworkCapabilities{
		{
			OrchURI: "",
			Capabilities: &net.Capabilities{
				Constraints: &net.Capabilities_Constraints{
					PerCapability: map[uint32]*net.Capabilities_CapabilityConstraints{
						uint32(core.Capability_LiveVideoToVideo): {
							Models: map[string]*net.Capabilities_CapabilityConstraints_ModelConstraint{
								"model-a": {},
							},
						},
					},
				},
			},
			PriceInfo: &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1},
		},
		{
			OrchURI:      "://also-invalid",
			Capabilities: orch1Caps,
			PriceInfo:    &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1},
		},
		{
			OrchURI:      "https://too-expensive.example.com:8935",
			Capabilities: orch1Caps,
			PriceInfo:    &net.PriceInfo{PricePerUnit: 200, PixelsPerUnit: 1},
		},
		{
			OrchURI:      "https://missing-price.example.com:8935",
			Capabilities: orch1Caps,
			// Missing global and per-capability prices should be treated as ineligible
			// when a max price is configured for this capability/model.
			PriceInfo: &net.PriceInfo{PricePerUnit: 0, PixelsPerUnit: 0},
		},
		{
			OrchURI:      "https://nil-price.example.com:8935",
			Capabilities: orch1Caps,
			// Explicit nil global price should also be treated as ineligible.
			PriceInfo: nil,
		},
	}))
	ineligibleRDP := &remoteDiscoveryPool{
		node:         ineligibleNode,
		refreshEvery: time.Hour,
	}
	ineligibleRDP.refresh()
	ineligibleReq := httptest.NewRequest(http.MethodGet, "/discover-orchestrators", nil)
	ineligibleRR := httptest.NewRecorder()
	ls.GetOrchestrators(ineligibleRDP, ineligibleRR, ineligibleReq)
	require.Equal(http.StatusServiceUnavailable, ineligibleRR.Code)
	require.Equal("application/json", ineligibleRR.Header().Get("Content-Type"))
}

func TestRemoteSigner_Discovery_EmptyCacheRetriesBeforeInterval(t *testing.T) {
	require := require.New(t)

	capability := core.Capability_LiveVideoToVideo
	modelID := "scope"
	BroadcastCfg.SetCapabilityMaxPrice(capability, modelID, core.NewFixedPrice(big.NewRat(200, 995328000000)))
	defer BroadcastCfg.SetCapabilityMaxPrice(capability, modelID, nil)

	// Node starts with no network capabilities (pool poll has not populated the cache yet).
	node := &core.LivepeerNode{}

	// Long interval: with the bug, an empty first refresh would lock discovery out for an hour.
	rdp := &remoteDiscoveryPool{
		node:         node,
		refreshEvery: time.Hour,
	}
	ls := &LivepeerServer{}

	// First request lands before the cache is populated: 503, cache empty.
	emptyReq := httptest.NewRequest(http.MethodGet, "/discover-orchestrators?caps=live-video-to-video/scope", nil)
	emptyRR := httptest.NewRecorder()
	ls.GetOrchestrators(rdp, emptyRR, emptyReq)
	require.Equal(http.StatusServiceUnavailable, emptyRR.Code)

	// The orchestrator pool finishes its poll and populates network capabilities.
	require.NoError(node.UpdateNetworkCapabilities([]*common.OrchNetworkCapabilities{
		{
			OrchURI: "https://late.example.com:8935",
			Discovery: discoveryRaw(t, `[{
				"address": "https://late.example.com:8935",
				"runners": [
					{"url":"https://late.example.com:8935/apps/runner/session","app":"live-video-to-video/scope","capacity":1,"price_info":{"price_per_unit":5,"pixels_per_unit":995328000000,"unit":"WEI"}}
				]
			}]`),
		},
	}))

	// Follow-up within refreshEvery: the empty cache re-derives instead of staying rate-limited.
	req := httptest.NewRequest(http.MethodGet, "/discover-orchestrators?caps=live-video-to-video/scope", nil)
	rr := httptest.NewRecorder()
	ls.GetOrchestrators(rdp, rr, req)

	require.Equal(http.StatusOK, rr.Code)
	var resp []discoveryResponse
	require.NoError(json.NewDecoder(rr.Body).Decode(&resp))
	require.Len(resp, 1)
	require.Equal("https://late.example.com:8935", resp[0].Address)
}

func TestRemoteSigner_Discovery_UsesRunnerDiscoveryWhenRPCCapabilitiesMissing(t *testing.T) {
	require := require.New(t)

	capability := core.Capability_LiveVideoToVideo
	modelID := "scope"
	BroadcastCfg.SetCapabilityMaxPrice(capability, modelID, core.NewFixedPrice(big.NewRat(200, 995328000000)))
	defer BroadcastCfg.SetCapabilityMaxPrice(capability, modelID, nil)

	node := &core.LivepeerNode{}
	require.NoError(node.UpdateNetworkCapabilities([]*common.OrchNetworkCapabilities{
		{
			OrchURI: "https://runner-derived.example.com:8935",
			Discovery: discoveryRaw(t, `[{
				"address": "https://runner-derived.example.com:8935",
				"runners": [
					{"url":"https://runner-derived.example.com:8935/apps/runner/session","app":"live-video-to-video/scope","capacity":1,"price_info":{"price_per_unit":5,"pixels_per_unit":995328000000,"unit":"WEI"}}
				]
			}]`),
		},
	}))

	rdp := &remoteDiscoveryPool{
		node:         node,
		refreshEvery: time.Hour,
	}
	ls := &LivepeerServer{}

	req := httptest.NewRequest(http.MethodGet, "/discover-orchestrators?caps=live-video-to-video/scope", nil)
	rr := httptest.NewRecorder()
	ls.GetOrchestrators(rdp, rr, req)

	require.Equal(http.StatusOK, rr.Code)
	var resp []discoveryResponse
	require.NoError(json.NewDecoder(rr.Body).Decode(&resp))
	require.Len(resp, 1)
	require.Equal("https://runner-derived.example.com:8935", resp[0].Address)
	require.Equal([]string{"live-video-to-video/scope"}, resp[0].Capabilities)
	require.Equal([]string{"live-video-to-video/scope"}, discoveryRunnerApps(t, resp[0]))
}

func TestRemoteSigner_Discovery_RunnerDiscoveryKeepsEligibleRunnersWhenRPCCapabilitiesMissing(t *testing.T) {
	require := require.New(t)

	capability := core.Capability_LiveVideoToVideo
	modelID := "scope"
	BroadcastCfg.SetCapabilityMaxPrice(capability, modelID, core.NewFixedPrice(big.NewRat(200, 995328000000)))
	defer BroadcastCfg.SetCapabilityMaxPrice(capability, modelID, nil)

	node := &core.LivepeerNode{}
	require.NoError(node.UpdateNetworkCapabilities([]*common.OrchNetworkCapabilities{
		{
			OrchURI: "https://mixed-runner-derived.example.com:8935",
			Discovery: discoveryRaw(t, `[{
				"address": "https://mixed-runner-derived.example.com:8935",
				"runners": [
					{"url":"https://mixed-runner-derived.example.com:8935/too-expensive","app":"live-video-to-video/scope","price_info":{"price_per_unit":201,"pixels_per_unit":995328000000,"unit":"WEI"}},
					{"url":"https://mixed-runner-derived.example.com:8935/usd","app":"live-video-to-video/scope","price_info":{"price_per_unit":5,"pixels_per_unit":995328000000,"unit":"USD"}},
					{"url":"https://mixed-runner-derived.example.com:8935/missing-price","app":"live-video-to-video/scope"},
					{"url":"https://mixed-runner-derived.example.com:8935/invalid-pixels","app":"live-video-to-video/scope","price_info":{"price_per_unit":5,"pixels_per_unit":0,"unit":"WEI"}},
					{"url":"https://mixed-runner-derived.example.com:8935/apps/runner/session","app":"live-video-to-video/scope","capacity":1,"price_info":{"price_per_unit":5,"pixels_per_unit":995328000000,"unit":"WEI"}}
				]
			}]`),
		},
	}))

	rdp := &remoteDiscoveryPool{
		node:         node,
		refreshEvery: time.Hour,
	}
	ls := &LivepeerServer{}

	req := httptest.NewRequest(http.MethodGet, "/discover-orchestrators?caps=live-video-to-video/scope", nil)
	rr := httptest.NewRecorder()
	ls.GetOrchestrators(rdp, rr, req)

	require.Equal(http.StatusOK, rr.Code)
	var resp []discoveryResponse
	require.NoError(json.NewDecoder(rr.Body).Decode(&resp))
	require.Len(resp, 1)
	require.Equal("https://mixed-runner-derived.example.com:8935", resp[0].Address)
	require.Equal([]string{"live-video-to-video/scope"}, resp[0].Capabilities)
	require.Len(resp[0].Runners, 1)
	require.Equal("https://mixed-runner-derived.example.com:8935/apps/runner/session", resp[0].Runners[0].URL)
	require.Equal([]string{"live-video-to-video/scope"}, discoveryRunnerApps(t, resp[0]))
}

func TestRemoteSigner_Discovery_FiltersRunnerDiscoveryPricing(t *testing.T) {
	require := require.New(t)

	capability := core.Capability_LiveVideoToVideo
	modelID := "scope"
	BroadcastCfg.SetCapabilityMaxPrice(capability, modelID, core.NewFixedPrice(big.NewRat(200, 995328000000)))
	defer BroadcastCfg.SetCapabilityMaxPrice(capability, modelID, nil)

	node := &core.LivepeerNode{}
	require.NoError(node.UpdateNetworkCapabilities([]*common.OrchNetworkCapabilities{
		{
			OrchURI: "https://filtered.example.com:8935",
			Discovery: discoveryRaw(t, `[{
				"address": "https://filtered.example.com:8935",
				"runners": [
					{"url":"https://filtered.example.com:8935/too-expensive","app":"live-video-to-video/scope","price_info":{"price_per_unit":201,"pixels_per_unit":995328000000,"unit":"WEI"}},
					{"url":"https://filtered.example.com:8935/usd","app":"live-video-to-video/scope","price_info":{"price_per_unit":5,"pixels_per_unit":995328000000,"unit":"USD"}},
					{"url":"https://filtered.example.com:8935/missing-price","app":"live-video-to-video/scope"},
					{"url":"https://filtered.example.com:8935/non-positive-price","app":"live-video-to-video/scope","price_info":{"price_per_unit":0,"pixels_per_unit":995328000000,"unit":"WEI"}},
					{"url":"https://filtered.example.com:8935/non-positive-pixels","app":"live-video-to-video/scope","price_info":{"price_per_unit":5,"pixels_per_unit":0,"unit":"WEI"}}
				]
			}]`),
		},
	}))

	rdp := &remoteDiscoveryPool{
		node:         node,
		refreshEvery: time.Hour,
	}
	ls := &LivepeerServer{}

	req := httptest.NewRequest(http.MethodGet, "/discover-orchestrators", nil)
	rr := httptest.NewRecorder()
	ls.GetOrchestrators(rdp, rr, req)

	require.Equal(http.StatusServiceUnavailable, rr.Code)
	require.Equal("application/json", rr.Header().Get("Content-Type"))
}

func discoveryRaw(t *testing.T, data string) json.RawMessage {
	t.Helper()
	require.True(t, json.Valid([]byte(data)))
	return json.RawMessage(data)
}

func discoveryRunnerApps(t *testing.T, resp discoveryResponse) []string {
	t.Helper()
	apps := make([]string, 0, len(resp.Runners))
	for _, runner := range resp.Runners {
		apps = append(apps, runner.App)
	}
	return apps
}

func discoveryResponseByAddress(t *testing.T, resp []discoveryResponse, address string) discoveryResponse {
	t.Helper()
	for _, entry := range resp {
		if entry.Address == address {
			return entry
		}
	}
	require.Failf(t, "missing discovery response", "address=%s", address)
	return discoveryResponse{}
}

func requireNotContainsDiscoveryField(t *testing.T, body []byte, field string) {
	t.Helper()
	var raw []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &raw))
	for _, entry := range raw {
		require.NotContains(t, entry, field)
	}
}

func TestRemoteDiscoveryRunnerDuplicateComparison(t *testing.T) {
	base := runner.LiveRunnerDiscoveryRunner{
		URL:               "https://runner.example.com/scope",
		App:               "live-video-to-video/model-a",
		Capacity:          2,
		CapacityUsed:      1,
		CapacityAvailable: 1,
		PriceInfo: &runner.LiveRunnerPriceInfo{
			PricePerUnit:  7,
			PixelsPerUnit: 1,
			Unit:          "WEI",
		},
	}

	usageChanged := base
	usageChanged.CapacityUsed = 0
	usageChanged.CapacityAvailable = 2
	require.True(t, sameRemoteDiscoveryRunner(base, usageChanged))

	capacityChanged := base
	capacityChanged.Capacity = 3
	require.False(t, sameRemoteDiscoveryRunner(base, capacityChanged))
}

func TestRemoteSigner_Discovery_RequiresAdvertisedPriceWithoutMaxPrice(t *testing.T) {
	require := require.New(t)

	capability := core.Capability_LiveVideoToVideo
	modelPriced := "model-priced"
	modelUnpriced := "model-unpriced"
	BroadcastCfg.SetMaxPrice(nil)
	BroadcastCfg.SetCapabilityMaxPrice(capability, modelPriced, nil)
	BroadcastCfg.SetCapabilityMaxPrice(capability, modelUnpriced, nil)

	caps := core.NewCapabilities([]core.Capability{capability}, nil)
	caps.SetPerCapabilityConstraints(core.PerCapabilityConstraints{
		capability: &core.CapabilityConstraints{
			Models: map[string]*core.ModelConstraint{
				modelPriced:   {},
				modelUnpriced: {},
			},
		},
	})

	node := &core.LivepeerNode{}
	require.NoError(node.UpdateNetworkCapabilities([]*common.OrchNetworkCapabilities{
		{
			OrchURI:      "https://priced.example.com:8935",
			Capabilities: caps.ToNetCapabilities(),
			CapabilitiesPrices: []*net.PriceInfo{
				{
					PricePerUnit:  7,
					PixelsPerUnit: 1,
					Capability:    uint32(capability),
					Constraint:    modelPriced,
				},
			},
			Discovery: discoveryRaw(t, `[{
				"address": "https://priced.example.com:8935",
				"runners": [
					{"url":"https://priced.example.com:8935/priced","app":"live-video-to-video/model-priced","price_info":{"price_per_unit":7,"pixels_per_unit":1,"unit":"WEI"}},
					{"url":"https://priced.example.com:8935/unpriced","app":"live-video-to-video/model-unpriced","price_info":{"price_per_unit":7,"pixels_per_unit":1,"unit":"WEI"}}
				]
			}]`),
		},
		{
			OrchURI:      "https://unpriced.example.com:8935",
			Capabilities: caps.ToNetCapabilities(),
			Discovery: discoveryRaw(t, `[{
				"address": "https://unpriced.example.com:8935",
				"runners": [
					{"url":"https://unpriced.example.com:8935/priced","app":"live-video-to-video/model-priced","price_info":{"price_per_unit":7,"pixels_per_unit":1,"unit":"WEI"}},
					{"url":"https://unpriced.example.com:8935/unpriced","app":"live-video-to-video/model-unpriced","price_info":{"price_per_unit":7,"pixels_per_unit":1,"unit":"WEI"}}
				]
			}]`),
		},
	}))

	rdp := &remoteDiscoveryPool{
		node:         node,
		refreshEvery: time.Hour,
	}
	ls := &LivepeerServer{}

	req := httptest.NewRequest(http.MethodGet, "/discover-orchestrators", nil)
	rr := httptest.NewRecorder()
	ls.GetOrchestrators(rdp, rr, req)

	require.Equal(http.StatusOK, rr.Code)
	var resp []discoveryResponse
	require.NoError(json.NewDecoder(rr.Body).Decode(&resp))
	require.Len(resp, 1)
	require.Equal("https://priced.example.com:8935", resp[0].Address)
	require.Equal([]string{"live-video-to-video/model-priced"}, resp[0].Capabilities)
	require.Equal([]string{"live-video-to-video/model-priced"}, discoveryRunnerApps(t, resp[0]))

	capsReq := httptest.NewRequest(http.MethodGet, "/discover-orchestrators?caps=live-video-to-video/model-priced", nil)
	capsRR := httptest.NewRecorder()
	ls.GetOrchestrators(rdp, capsRR, capsReq)
	require.Equal(http.StatusOK, capsRR.Code)
	var capsResp []discoveryResponse
	require.NoError(json.NewDecoder(capsRR.Body).Decode(&capsResp))
	require.Len(capsResp, 1)
	require.Equal("https://priced.example.com:8935", capsResp[0].Address)

	unpricedReq := httptest.NewRequest(http.MethodGet, "/discover-orchestrators?caps=live-video-to-video/model-unpriced", nil)
	unpricedRR := httptest.NewRecorder()
	ls.GetOrchestrators(rdp, unpricedRR, unpricedReq)
	require.Equal(http.StatusOK, unpricedRR.Code)
	var unpricedResp []discoveryResponse
	require.NoError(json.NewDecoder(unpricedRR.Body).Decode(&unpricedResp))
	require.Empty(unpricedResp)
}

func TestRemoteSigner_Discovery_RefreshesAfterInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		require := require.New(t)

		capability := core.Capability_LiveVideoToVideo
		modelA := "model-a"
		modelB := "model-b"
		BroadcastCfg.SetCapabilityMaxPrice(capability, modelA, core.NewFixedPrice(big.NewRat(100, 1)))
		BroadcastCfg.SetCapabilityMaxPrice(capability, modelB, core.NewFixedPrice(big.NewRat(100, 1)))
		defer BroadcastCfg.SetCapabilityMaxPrice(capability, modelA, nil)
		defer BroadcastCfg.SetCapabilityMaxPrice(capability, modelB, nil)

		capsA := core.NewCapabilities([]core.Capability{capability}, nil)
		capsA.SetPerCapabilityConstraints(core.PerCapabilityConstraints{
			capability: &core.CapabilityConstraints{
				Models: map[string]*core.ModelConstraint{
					modelA: {},
				},
			},
		})
		capsB := core.NewCapabilities([]core.Capability{capability}, nil)
		capsB.SetPerCapabilityConstraints(core.PerCapabilityConstraints{
			capability: &core.CapabilityConstraints{
				Models: map[string]*core.ModelConstraint{
					modelB: {},
				},
			},
		})

		node := &core.LivepeerNode{}
		require.NoError(node.UpdateNetworkCapabilities([]*common.OrchNetworkCapabilities{
			{
				OrchURI:      "https://orch-a.example.com:8935",
				Capabilities: capsA.ToNetCapabilities(),
				PriceInfo:    &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1},
			},
		}))

		rdp := &remoteDiscoveryPool{
			node:         node,
			refreshEvery: time.Minute,
		}
		ls := &LivepeerServer{}

		callDiscovery := func() []discoveryResponse {
			req := httptest.NewRequest(http.MethodGet, "/discover-orchestrators", nil)
			rr := httptest.NewRecorder()
			ls.GetOrchestrators(rdp, rr, req)
			require.Equal(http.StatusOK, rr.Code)
			var resp []discoveryResponse
			require.NoError(json.NewDecoder(rr.Body).Decode(&resp))
			return resp
		}

		// First request populates cache.
		resp := callDiscovery()
		require.Len(resp, 1)
		require.Equal("https://orch-a.example.com:8935", resp[0].Address)
		require.Equal([]string{"live-video-to-video/model-a"}, resp[0].Capabilities)

		// Update source network capabilities, but stay within refresh interval.
		require.NoError(node.UpdateNetworkCapabilities([]*common.OrchNetworkCapabilities{
			{
				OrchURI:      "https://orch-b.example.com:8935",
				Capabilities: capsB.ToNetCapabilities(),
				PriceInfo:    &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1},
			},
		}))
		time.Sleep(30 * time.Second)
		resp = callDiscovery()
		require.Len(resp, 1)
		require.Equal("https://orch-a.example.com:8935", resp[0].Address)

		// Once interval elapses, discovery response should reflect refreshed cache.
		time.Sleep(31 * time.Second)
		resp = callDiscovery()
		require.Len(resp, 1)
		require.Equal("https://orch-b.example.com:8935", resp[0].Address)
		require.Equal([]string{"live-video-to-video/model-b"}, resp[0].Capabilities)
	})
}

func TestGetOrchInfoSig_SendsConfiguredHeaders(t *testing.T) {
	require := require.New(t)

	remoteTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(http.MethodPost, r.Method)
		require.Equal("/sign-orchestrator-info", r.URL.Path)
		require.Equal("application/json", r.Header.Get("Content-Type"))
		require.Equal("Bearer gateway-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"address":   "0x1234",
			"signature": "0xabcd",
		})
	}))
	defer remoteTS.Close()

	remoteURL, err := url.Parse(remoteTS.URL)
	require.NoError(err)

	resp, err := GetOrchInfoSig(remoteURL, map[string]string{"Authorization": "Bearer gateway-token"})
	require.NoError(err)
	require.Equal([]byte{0x12, 0x34}, []byte(resp.Address))
	require.Equal([]byte{0xab, 0xcd}, []byte(resp.Signature))
}

func byocCaps(t *testing.T, modelID string) *core.Capabilities {
	t.Helper()
	if modelID == "" {
		return nil
	}
	caps := core.NewCapabilities([]core.Capability{core.Capability_BYOC}, nil)
	caps.SetPerCapabilityConstraints(core.PerCapabilityConstraints{
		core.Capability_BYOC: &core.CapabilityConstraints{
			Models: map[string]*core.ModelConstraint{
				modelID: {},
			},
		},
	})
	return caps
}

func byocCapsBlob(t *testing.T, modelID string) []byte {
	t.Helper()
	caps := byocCaps(t, modelID)
	if caps == nil {
		return nil
	}
	blob, err := proto.Marshal(caps.ToNetCapabilities())
	require.NoError(t, err)
	return blob
}

func TestResolveByocPrice(t *testing.T) {
	require := require.New(t)

	byocCap := uint32(core.Capability_BYOC)
	oInfo := &net.OrchestratorInfo{
		PriceInfo: &net.PriceInfo{PricePerUnit: 100, PixelsPerUnit: 1},
		CapabilitiesPrices: []*net.PriceInfo{
			{Capability: byocCap, Constraint: "nano-banana", PricePerUnit: 10, PixelsPerUnit: 1},
			{Capability: byocCap, Constraint: "recraft-v4", PricePerUnit: 20, PixelsPerUnit: 1},
			{Capability: uint32(core.Capability_LiveVideoToVideo), Constraint: "nano-banana", PricePerUnit: 999, PixelsPerUnit: 1},
			{Capability: byocCap, Constraint: "free-cap", PricePerUnit: 0, PixelsPerUnit: 1},
			{Capability: byocCap, Constraint: "dup-cap", PricePerUnit: 0, PixelsPerUnit: 1},
			{Capability: byocCap, Constraint: "dup-cap", PricePerUnit: 30, PixelsPerUnit: 1},
		},
	}

	tests := []struct {
		name       string
		capability string
		oInfo      *net.OrchestratorInfo
		want       *net.PriceInfo
	}{
		{
			name:       "resolves per capability",
			capability: "recraft-v4",
			oInfo:      oInfo,
			want:       &net.PriceInfo{PricePerUnit: 20, PixelsPerUnit: 1},
		},
		{
			name:       "resolves the other capability",
			capability: "nano-banana",
			oInfo:      oInfo,
			want:       &net.PriceInfo{PricePerUnit: 10, PixelsPerUnit: 1},
		},
		{
			name:       "unknown capability falls back (nil)",
			capability: "does-not-exist",
			oInfo:      oInfo,
			want:       nil,
		},
		{
			name:       "empty capability falls back (nil)",
			capability: "",
			oInfo:      oInfo,
			want:       nil,
		},
		{
			name:       "zero/invalid matched rate falls back (nil)",
			capability: "free-cap",
			oInfo:      oInfo,
			want:       nil,
		},
		{
			name:       "skips invalid duplicate, honors later valid entry",
			capability: "dup-cap",
			oInfo:      oInfo,
			want:       &net.PriceInfo{PricePerUnit: 30, PixelsPerUnit: 1},
		},
		{
			name:       "no capabilities prices falls back (nil)",
			capability: "nano-banana",
			oInfo:      &net.OrchestratorInfo{PriceInfo: &net.PriceInfo{PricePerUnit: 100, PixelsPerUnit: 1}},
			want:       nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveByocPrice(byocCaps(t, tc.capability), tc.oInfo)
			if tc.want == nil {
				require.Nil(got)
				return
			}
			require.NotNil(got)
			require.Equal(tc.want.PricePerUnit, got.PricePerUnit, "PricePerUnit")
			require.Equal(tc.want.PixelsPerUnit, got.PixelsPerUnit, "PixelsPerUnit")
		})
	}
}

func TestGenerateLivePayment_ByocCapabilityPricing(t *testing.T) {
	require := require.New(t)

	ethClient := newTestEthClient(t)
	node, _ := core.NewLivepeerNode(ethClient, "", nil)
	node.Balances = core.NewAddressBalances(1 * time.Minute)

	// Large EV keeps the lv2v fee under the 100-ticket cap.
	ev := big.NewRat(10_000_000_000, 1)
	var totalTickets uint32
	sender := newMockSender(mockSenderConfig{
		ev: ev,
		createTicketBatchFn: func(args mock.Arguments, batch *pm.TicketBatch) {
			size := args.Int(1)
			*batch = *defaultTicketBatch()
			var baseSig []byte
			if len(batch.SenderParams) > 0 && batch.SenderParams[0] != nil {
				baseSig = batch.SenderParams[0].Sig
			}
			batch.SenderParams = make([]*pm.TicketSenderParams, size)
			for i := 0; i < size; i++ {
				totalTickets++
				batch.SenderParams[i] = &pm.TicketSenderParams{SenderNonce: totalTickets, Sig: baseSig}
			}
		},
	})
	node.Sender = sender
	ls := &LivepeerServer{LivepeerNode: node}

	autoPrice, err := core.NewAutoConvertedPrice("", big.NewRat(1_000, 1), nil)
	require.NoError(err)
	BroadcastCfg.SetMaxPrice(autoPrice)
	defer BroadcastCfg.SetMaxPrice(nil)

	// base >> 2x caps so leaving oInfo.PriceInfo at base would trip the doubling guard.
	const basePPU, capNanoPPU, capRecraftPPU = 100, 10, 20
	oInfo := &net.OrchestratorInfo{
		Address:   ethClient.addr.Bytes(),
		PriceInfo: &net.PriceInfo{PricePerUnit: basePPU, PixelsPerUnit: 1},
		TicketParams: &net.TicketParams{
			Recipient: pm.RandAddress().Bytes(),
		},
		AuthToken: stubAuthToken,
		CapabilitiesPrices: []*net.PriceInfo{
			{Capability: uint32(core.Capability_BYOC), Constraint: "nano-banana", PricePerUnit: capNanoPPU, PixelsPerUnit: 1},
			{Capability: uint32(core.Capability_BYOC), Constraint: "recraft-v4", PricePerUnit: capRecraftPPU, PixelsPerUnit: 1},
		},
	}
	orchBlob, err := proto.Marshal(oInfo)
	require.NoError(err)

	const preloadSecs = 60
	lv2vPixels := int64(defaultSegInfo.Height) * int64(defaultSegInfo.Width) * int64(defaultSegInfo.FPS) * preloadSecs

	doPayment := func(capability string) RemotePaymentState {
		reqBody, err := json.Marshal(RemotePaymentRequest{
			Orchestrator: orchBlob,
			Type:         RemoteType_LiveVideoToVideo,
			Capabilities: byocCapsBlob(t, capability),
		})
		require.NoError(err)
		req := httptest.NewRequest(http.MethodPost, "/generate-live-payment", bytes.NewReader(reqBody))
		rr := httptest.NewRecorder()
		ls.GenerateLivePayment(rr, req)
		require.Equal(http.StatusOK, rr.Code, "body=%s", rr.Body.String())
		var resp RemotePaymentResponse
		require.NoError(json.NewDecoder(rr.Body).Decode(&resp))
		var state RemotePaymentState
		require.NoError(json.Unmarshal(resp.State.State, &state))
		return state
	}

	feeFromState := func(state RemotePaymentState) *big.Rat {
		bal := new(big.Rat)
		_, ok := bal.SetString(state.Balance)
		require.True(ok, "parse balance %q", state.Balance)
		price := big.NewRat(state.InitialPricePerUnit, state.InitialPixelsPerUnit)
		var pixels int64
		if state.InitialPricePerUnit == basePPU && state.InitialPixelsPerUnit == 1 {
			pixels = lv2vPixels
		} else {
			pixels = preloadSecs
		}
		return new(big.Rat).Mul(price, big.NewRat(pixels, 1))
	}

	assertBalanceConsistent := func(state RemotePaymentState, fee *big.Rat) {
		smc := fee
		if ev.Cmp(smc) > 0 {
			smc = ev
		}
		q := new(big.Rat).Quo(smc, ev)
		nt := new(big.Int).Quo(q.Num(), q.Denom())
		if new(big.Int).Rem(q.Num(), q.Denom()).Sign() != 0 {
			nt.Add(nt, big.NewInt(1))
		}
		wantBal := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).SetInt(nt), ev), fee)
		gotBal := new(big.Rat)
		_, ok := gotBal.SetString(state.Balance)
		require.True(ok)
		require.Zero(gotBal.Cmp(wantBal), "balance got=%s want=%s fee=%s", gotBal.RatString(), wantBal.RatString(), fee.RatString())
	}

	nanoState := doPayment("nano-banana")
	require.EqualValues(capNanoPPU, nanoState.InitialPricePerUnit, "must lock the resolved cap price")
	require.EqualValues(1, nanoState.InitialPixelsPerUnit)
	nanoFee := big.NewRat(capNanoPPU*preloadSecs, 1)
	require.Zero(feeFromState(nanoState).Cmp(nanoFee), "nano fee")
	assertBalanceConsistent(nanoState, nanoFee)

	recraftState := doPayment("recraft-v4")
	require.EqualValues(capRecraftPPU, recraftState.InitialPricePerUnit)
	recraftFee := big.NewRat(capRecraftPPU*preloadSecs, 1)
	require.Zero(feeFromState(recraftState).Cmp(recraftFee), "recraft fee")
	assertBalanceConsistent(recraftState, recraftFee)
	require.Zero(recraftFee.Cmp(new(big.Rat).Mul(nanoFee, big.NewRat(2, 1))), "recraft fee must be 2x nano fee")

	unknownState := doPayment("totally-unknown-cap")
	require.EqualValues(basePPU, unknownState.InitialPricePerUnit, "unknown cap must fall back to base")
	fallbackFee := new(big.Rat).Mul(big.NewRat(basePPU, 1), big.NewRat(lv2vPixels, 1))
	require.Zero(feeFromState(unknownState).Cmp(fallbackFee))
	assertBalanceConsistent(unknownState, fallbackFee)
}
