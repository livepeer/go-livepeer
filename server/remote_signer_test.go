package server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/golang/protobuf/proto"
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
		Address:   ethcommon.HexToAddress("0x1").Bytes(),
		PriceInfo: &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1},
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

	priceIncreaseStateBytes, priceIncreaseStateSig := func() ([]byte, []byte) {
		stateBytes, err := json.Marshal(RemotePaymentState{
			StateID:              "state",
			OrchestratorAddress:  ethcommon.BytesToAddress(orchInfo.Address),
			InitialPricePerUnit:  100,
			InitialPixelsPerUnit: 1,
		})
		require.NoError(err)
		return stateBytes, sign(stateBytes)
	}()

	tests := []struct {
		name       string
		stateBytes []byte
		stateSig   []byte
		orchInfo   *net.OrchestratorInfo
		wantStatus int
		wantMsg    string
	}{
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
			stateSig:   sign([]byte("not-json")),
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
			stateSig: sign(func() []byte {
				state, err := json.Marshal(RemotePaymentState{
					StateID:              "state",
					OrchestratorAddress:  ethcommon.HexToAddress("0x1"),
					InitialPixelsPerUnit: 1,
				})
				require.NoError(err)
				return state
			}()),
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
			stateSig:   priceIncreaseStateSig,
			orchInfo: func() *net.OrchestratorInfo {
				oInfo := proto.Clone(orchInfo).(*net.OrchestratorInfo)
				oInfo.PriceInfo = &net.PriceInfo{PricePerUnit: 250, PixelsPerUnit: 1}
				return oInfo
			}(),
			wantStatus: HTTPStatusPriceExceeded,
			wantMsg:    "Orchestrator price has more than doubled",
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

			reqBody, err := json.Marshal(RemotePaymentRequest{
				Orchestrator: orchBlob,
				InPixels:     1,
				State:        RemotePaymentStateSig{State: tt.stateBytes, Sig: tt.stateSig},
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

	resp, payment := doPayment(RemotePaymentRequest{
		Orchestrator: orchBlob,
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
		InPixels:     inPixelsUpdated,
		State:        resp.State,
	})

	var stateFee2 RemotePaymentState
	require.NoError(json.Unmarshal(resp2.State.State, &stateFee2))
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
