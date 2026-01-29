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

func newMockSender(t *testing.T, validateErr error) *pm.MockSender {
	t.Helper()

	sender := &pm.MockSender{}
	sender.On("StartSessionWithNonce", mock.Anything, mock.Anything).Return("pmSession")
	sender.On("CleanupSession", mock.Anything).Return()
	sender.On("ValidateTicketParams", mock.Anything).Return(validateErr)
	sender.On("EV", mock.Anything).Return(big.NewRat(1, 1), nil)
	sender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(defaultTicketBatch(), nil)
	sender.On("Nonce", mock.Anything).Return(7, nil)
	return sender
}

func TestGenerateLivePayment_RequestValidationErrors(t *testing.T) {
	require := require.New(t)

	baseOrchInfo := &net.OrchestratorInfo{
		Address:   ethcommon.HexToAddress("0x0000000000000000000000000000000000000001").Bytes(),
		PriceInfo: &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1},
		TicketParams: &net.TicketParams{
			Recipient: pm.RandAddress().Bytes(),
		},
		AuthToken: stubAuthToken,
	}

	makeOrchBlob := func(oInfo *net.OrchestratorInfo) []byte {
		b, err := proto.Marshal(&net.PaymentResult{Info: oInfo})
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
			name: "payment result missing info",
			req: func() RemotePaymentRequest {
				// Unknown field to keep non-empty payload while Info stays nil.
				return RemotePaymentRequest{Orchestrator: []byte{0x10, 0x01}, InPixels: 1}
			}(),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "Missing orch info",
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
			sender:     newMockSender(t, pm.ErrTicketParamsExpired),
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, _ := core.NewLivepeerNode(nil, "", nil)
			node.Balances = core.NewAddressBalances(1 * time.Minute)
			if tt.sender != nil {
				node.Sender = tt.sender
			} else {
				node.Sender = newMockSender(t, nil)
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
	node.Sender = newMockSender(t, nil)
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

	tests := []struct {
		name       string
		stateBytes []byte
		stateSig   []byte
		orchAddr   []byte
		wantMsg    string
	}{
		{
			name:       "invalid state signature",
			stateBytes: []byte(`{"stateID":"state","orchestratorAddress":"0x0000000000000000000000000000000000000001"}`),
			stateSig:   []byte("bad"),
			orchAddr:   orchInfo.Address,
			wantMsg:    "invalid sig",
		},
		{
			name:       "invalid state json",
			stateBytes: []byte("not-json"),
			stateSig:   sign([]byte("not-json")),
			orchAddr:   orchInfo.Address,
			wantMsg:    "invalid state",
		},
		{
			name: "orchestrator address mismatch",
			stateBytes: func() []byte {
				state, err := json.Marshal(RemotePaymentState{
					StateID:             "state",
					OrchestratorAddress: ethcommon.HexToAddress("0x0000000000000000000000000000000000000002"),
				})
				require.NoError(err)
				return state
			}(),
			stateSig: sign(func() []byte {
				state, err := json.Marshal(RemotePaymentState{
					StateID:             "state",
					OrchestratorAddress: ethcommon.HexToAddress("0x0000000000000000000000000000000000000002"),
				})
				require.NoError(err)
				return state
			}()),
			orchAddr: orchInfo.Address,
			wantMsg:  "orchestratorAddress mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oInfo := proto.Clone(orchInfo).(*net.OrchestratorInfo)
			oInfo.Address = tt.orchAddr
			orchBlob, err := proto.Marshal(&net.PaymentResult{Info: oInfo})
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

			require.Equal(http.StatusBadRequest, rr.Code)
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
	node.Sender = newMockSender(t, nil)
	ls := &LivepeerServer{LivepeerNode: node}

	oInfo := &net.OrchestratorInfo{
		Address:   ethClient.addr.Bytes(),
		PriceInfo: &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1},
		TicketParams: &net.TicketParams{
			Recipient: pm.RandAddress().Bytes(),
		},
		AuthToken: stubAuthToken,
	}
	orchBlob, err := proto.Marshal(&net.PaymentResult{Info: oInfo})
	require.NoError(err)

	reqBody, err := json.Marshal(RemotePaymentRequest{
		Orchestrator: orchBlob,
		Type:         RemoteType_LiveVideoToVideo,
	})
	require.NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/generate-live-payment", bytes.NewReader(reqBody))
	rr := httptest.NewRecorder()

	ls.GenerateLivePayment(rr, req)

	require.Equal(http.StatusOK, rr.Code)
	var resp RemotePaymentResponse
	require.NoError(json.NewDecoder(rr.Body).Decode(&resp))
	require.NotEmpty(resp.Payment)
	require.NotEmpty(resp.SegCreds)
	require.NotEmpty(resp.State.State)
	require.NotEmpty(resp.State.Sig)

	var state RemotePaymentState
	require.NoError(json.Unmarshal(resp.State.State, &state))
	require.NotEmpty(state.StateID)
	require.Equal(ethClient.addr, state.OrchestratorAddress)
	require.Equal("pmSession", state.PMSessionID)
	require.Equal(uint32(7), state.SenderNonce)
	require.NotEmpty(state.Balance)
	require.False(state.LastUpdate.IsZero())

	_, err = base64.StdEncoding.DecodeString(resp.Payment)
	require.NoError(err)
	_, err = base64.StdEncoding.DecodeString(resp.SegCreds)
	require.NoError(err)
}
