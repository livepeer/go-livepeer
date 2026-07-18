package server

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpnet "github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/trickle"
	"github.com/livepeer/go-tools/drivers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIWorkerResults_ErrorsWhenAuthHeaderMissing(t *testing.T) {
	var l lphttp

	var w = httptest.NewRecorder()
	r, err := http.NewRequest(http.MethodPost, "/aiResults", nil)
	require.NoError(t, err)

	code, body := aiResultsTest(l, w, r)

	require.Equal(t, http.StatusUnauthorized, code)
	require.Contains(t, body, "Unauthorized")
}

func TestAIWorkerResults_ErrorsWhenCredentialsInvalid(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodPost, "/aiResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerAIWorker)
	r.Header.Set("Credentials", "BAD CREDENTIALS")

	code, body := aiResultsTest(l, w, r)
	require.Equal(t, http.StatusUnauthorized, code)
	require.Contains(t, body, "invalid secret")
}

func TestAIWorkerResults_ErrorsWhenContentTypeMissing(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodPost, "/aiResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerAIWorker)
	r.Header.Set("Credentials", "")

	code, body := aiResultsTest(l, w, r)

	require.Equal(t, http.StatusUnsupportedMediaType, code)
	require.Contains(t, body, "mime: no media type")
}

func TestAIWorkerResults_ErrorsWhenTaskIDMissing(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodPost, "/aiResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerAIWorker)
	r.Header.Set("Credentials", "")
	r.Header.Set("Content-Type", "application/json")

	code, body := aiResultsTest(l, w, r)

	require.Equal(t, http.StatusBadRequest, code)
	require.Contains(t, body, "Invalid Task ID")
}

func TestAIWorkerResults_BadRequestType(t *testing.T) {
	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}

	assert := assert.New(t)
	assert.Nil(nil)
	resultData := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		assert.NoError(err)
		w.Write([]byte("result binary data"))
	}))
	defer resultData.Close()
	// sending bad request
	notify := createAIJob(742, "text-to-image-invalid", "livepeer/model1", "")

	wkr := stubAIWorker{}
	node, _ := core.NewLivepeerNode(nil, "/tmp/thisdirisnotactuallyusedinthistest", nil)
	node.OrchSecret = "verbigsecret"
	node.AIWorker = &wkr
	node.Capabilities = createStubAIWorkerCapabilitiesForPipelineModelId("text-to-image", "livepeer/model1")

	var headers http.Header
	var body []byte
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, err := io.ReadAll(r.Body)
		assert.NoError(err)
		headers = r.Header
		body = out
		w.Write(nil)
	}))
	defer ts.Close()
	parsedURL, _ := url.Parse(ts.URL)
	// send empty request data
	runAIJob(node, parsedURL.Host, httpc, notify)
	time.Sleep(3 * time.Millisecond)

	assert.NotNil(body)
	assert.Equal("742", headers.Get("TaskId"))
	assert.Equal(aiWorkerErrorMimeType, headers.Get("Content-Type"))
	assert.Equal(node.OrchSecret, headers.Get("Credentials"))
	assert.Equal(protoVerAIWorker, headers.Get("Authorization"))
	assert.Equal("AI request validation failed for", string(body)[0:32])
}

func TestScopePaymentChallenge(t *testing.T) {
	lp := newScopeHTTP(t)

	challenge, oInfo := requestScopePaymentChallenge(t, lp)

	require.NotEmpty(t, challenge.PaymentParams)
	require.Equal(t, lp.orchestrator.ServiceURI().String(), challenge.Orchestrator)
	require.NotEmpty(t, challenge.ManifestID)
	require.Equal(t, challenge.ManifestID, oInfo.GetAuthToken().GetSessionId())
	require.NotNil(t, oInfo.GetTicketParams())
	require.NotNil(t, oInfo.GetPriceInfo())
	require.Equal(t, int64(4), oInfo.GetPriceInfo().GetPricePerUnit())
	require.Equal(t, int64(1), oInfo.GetPriceInfo().GetPixelsPerUnit())
}

func TestScopePaidRetryUsesChallengeManifestID(t *testing.T) {
	lp := newScopeHTTP(t)
	orch := lp.orchestrator.(*stubOrchestrator)
	orch.balances = make(map[ethcommon.Address]map[core.ManifestID]*big.Rat)
	orch.paymentCredit = big.NewRat(100, 1)
	challenge, oInfo := requestScopePaymentChallenge(t, lp)
	headers := scopePaymentHeaders(t, orch, oInfo.GetAuthToken(), challenge.ManifestID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(`{"model_id":"scope"}`))
	setRequestHeaders(req, headers)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp worker.LiveVideoToVideoResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.ManifestId)
	require.Equal(t, challenge.ManifestID, *resp.ManifestId)
	require.NotNil(t, resp.ControlUrl)
	require.Contains(t, *resp.ControlUrl, *resp.ManifestId+"-control")
	require.NotNil(t, resp.EventsUrl)
	require.Contains(t, *resp.EventsUrl, *resp.ManifestId+"-events")
	closeScopeEvents(t, lp, *resp.ManifestId)

	balance := orch.Balance(orch.Address(), core.ManifestID(challenge.ManifestID))
	require.NotNil(t, balance)
	require.Equal(t, "100", balance.FloatString(0))

	orch.balanceMu.Lock()
	balanceBuckets := make(map[core.ManifestID]*big.Rat, len(orch.balances[orch.Address()]))
	for manifestID, balance := range orch.balances[orch.Address()] {
		balanceBuckets[manifestID] = balance
	}
	orch.balanceMu.Unlock()
	require.Len(t, balanceBuckets, 1)
	require.Contains(t, balanceBuckets, core.ManifestID(challenge.ManifestID))
}

func TestScopePaidRetryRecurringAccountingUsesChallengeManifestID(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		lp := newScopeHTTP(t)
		lp.node.LivePaymentInterval = time.Second
		orch := lp.orchestrator.(*stubOrchestrator)
		orch.balances = make(map[ethcommon.Address]map[core.ManifestID]*big.Rat)
		orch.paymentCredit = big.NewRat(3, 1)

		challenge, oInfo := requestScopePaymentChallenge(t, lp)
		headers := scopePaymentHeadersWithPrice(t, orch, oInfo.GetAuthToken(), challenge.ManifestID, scopeTestPricePerSecond(1))

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(`{"model_id":"scope"}`))
		setRequestHeaders(req, headers)
		lp.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		var resp worker.LiveVideoToVideoResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.NotNil(t, resp.ManifestId)
		require.Equal(t, challenge.ManifestID, *resp.ManifestId)

		balance := orch.Balance(orch.Address(), core.ManifestID(challenge.ManifestID))
		require.NotNil(t, balance)
		require.Equal(t, "3", balance.FloatString(0))

		time.Sleep(time.Second)
		eventsCh := trickle.NewLocalPublisher(lp.trickleSrv, challenge.ManifestID+"-events", "application/json")
		require.NoError(t, eventsCh.Write(strings.NewReader(`{"event":"tick"}`)))
		synctest.Wait()

		balance = orch.Balance(orch.Address(), core.ManifestID(challenge.ManifestID))
		require.NotNil(t, balance)
		require.Equal(t, "2", balance.FloatString(0))

		orch.balanceMu.Lock()
		balanceBuckets := make(map[core.ManifestID]*big.Rat, len(orch.balances[orch.Address()]))
		for manifestID, balance := range orch.balances[orch.Address()] {
			balanceBuckets[manifestID] = balance
		}
		orch.balanceMu.Unlock()
		require.Len(t, balanceBuckets, 1)
		require.Contains(t, balanceBuckets, core.ManifestID(challenge.ManifestID))

		require.NoError(t, eventsCh.Close())
		synctest.Wait()
	})
}

func TestScopePaidRetryAllowsSegmentManifestToDifferFromAuthToken(t *testing.T) {
	lp := newScopeHTTP(t)
	orch := lp.orchestrator.(*stubOrchestrator)
	orch.balances = make(map[ethcommon.Address]map[core.ManifestID]*big.Rat)
	orch.paymentCredit = big.NewRat(100, 1)
	_, oInfo := requestScopePaymentChallenge(t, lp)
	legacyManifestID := "different-manifest"
	headers := scopePaymentHeaders(t, orch, oInfo.GetAuthToken(), legacyManifestID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(`{"model_id":"scope"}`))
	setRequestHeaders(req, headers)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp worker.LiveVideoToVideoResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.ManifestId)
	require.Equal(t, legacyManifestID, *resp.ManifestId)
	require.NotNil(t, resp.ControlUrl)
	require.Contains(t, *resp.ControlUrl, legacyManifestID+"-control")
	require.NotNil(t, resp.EventsUrl)
	require.Contains(t, *resp.EventsUrl, legacyManifestID+"-events")
	closeScopeEvents(t, lp, legacyManifestID)

	balance := orch.Balance(orch.Address(), core.ManifestID(legacyManifestID))
	require.NotNil(t, balance)
	require.Equal(t, "100", balance.FloatString(0))
	require.Nil(t, orch.Balance(orch.Address(), core.ManifestID(oInfo.GetAuthToken().GetSessionId())))
}

func TestScopeServerlessOffchainDoesNotRequirePayment(t *testing.T) {
	lp := newScopeOffchainHTTP(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(`{"model_id":"scope"}`))
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp worker.LiveVideoToVideoResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.ManifestId)
	require.NotEmpty(t, *resp.ManifestId)
	require.NotNil(t, resp.ControlUrl)
	require.Contains(t, *resp.ControlUrl, *resp.ManifestId+"-control")
	require.NotNil(t, resp.EventsUrl)
	require.Contains(t, *resp.EventsUrl, *resp.ManifestId+"-events")
	closeScopeEvents(t, lp, *resp.ManifestId)
}

func TestScopeRejectsOversizedPayload(t *testing.T) {
	lp := newScopeHTTP(t)

	oversizedBody := `{"model_id":"scope","gateway_request_id":"` + strings.Repeat("a", maxScopeRequestBodySize) + `"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(oversizedBody))
	req.Header.Set(scopePayerAddressHeader, lp.orchestrator.(*stubOrchestrator).Address().Hex())
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "http: request body too large")
}

func TestScopePaymentChallengeRejectsInvalidPayer(t *testing.T) {
	lp := newScopeHTTP(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(`{"model_id":"scope"}`))
	req.Header.Set(scopePayerAddressHeader, "not-an-address")
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusPaymentRequired, w.Code)
	require.Contains(t, w.Body.String(), "invalid live runner payment signer address")
}

func newScopeHTTP(t *testing.T) *lphttp {
	t.Helper()
	oldStorage := drivers.NodeStorage
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	t.Cleanup(func() { drivers.NodeStorage = oldStorage })
	node, err := core.NewLivepeerNode(nil, t.TempDir(), nil)
	require.NoError(t, err)
	node.Eth = &eth.StubClient{}
	node.Capabilities = createStubAIWorkerCapabilitiesForPipelineModelId("live-video-to-video", "scope")
	orch := newStubOrchestrator()
	orch.ticketParams = defaultTicketParams()
	lp := &lphttp{
		orchestrator: orch,
		node:         node,
		transRPC:     http.NewServeMux(),
	}
	require.NoError(t, startAIServer(lp))
	return lp
}

func newScopeOffchainHTTP(t *testing.T) *lphttp {
	t.Helper()
	lp := newScopeHTTP(t)
	lp.node.Eth = nil
	return lp
}

func requestScopePaymentChallenge(t *testing.T, lp *lphttp) (scopePaymentChallengeResponse, *lpnet.OrchestratorInfo) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(`{"model_id":"scope"}`))
	req.Header.Set(scopePayerAddressHeader, lp.orchestrator.(*stubOrchestrator).Address().Hex())
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusPaymentRequired, w.Code)
	return decodeScopePaymentChallenge(t, w.Body.Bytes())
}

func closeScopeEvents(t *testing.T, lp *lphttp, manifestID string) {
	t.Helper()
	eventsCh := trickle.NewLocalPublisher(lp.trickleSrv, manifestID+"-events", "application/json")
	require.NoError(t, eventsCh.Close())
}

func decodeScopePaymentChallenge(t *testing.T, body []byte) (scopePaymentChallengeResponse, *lpnet.OrchestratorInfo) {
	t.Helper()
	var challenge scopePaymentChallengeResponse
	require.NoError(t, json.Unmarshal(body, &challenge))
	require.NotEmpty(t, challenge.PaymentParams)
	raw, err := base64.StdEncoding.DecodeString(challenge.PaymentParams)
	require.NoError(t, err)
	var oInfo lpnet.OrchestratorInfo
	require.NoError(t, proto.Unmarshal(raw, &oInfo))
	return challenge, &oInfo
}

func setRequestHeaders(req *http.Request, headers http.Header) {
	for k, values := range headers {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}
}

func scopePaymentHeaders(t *testing.T, orch *stubOrchestrator, authToken *lpnet.AuthToken, manifestID string) http.Header {
	t.Helper()
	return scopePaymentHeadersWithPrice(t, orch, authToken, manifestID, &lpnet.PriceInfo{PricePerUnit: 10, PixelsPerUnit: 1})
}

func scopePaymentHeadersWithPrice(t *testing.T, orch *stubOrchestrator, authToken *lpnet.AuthToken, manifestID string, priceInfo *lpnet.PriceInfo) http.Header {
	t.Helper()
	md := &core.SegTranscodingMetadata{
		ManifestID: core.ManifestID(manifestID),
		Caps:       core.NewCapabilities(nil, nil),
		AuthToken:  authToken,
	}
	sig, err := orch.Sign(md.Flatten())
	require.NoError(t, err)
	segData, err := core.NetSegData(md)
	require.NoError(t, err)
	segData.Sig = sig
	segBytes, err := proto.Marshal(segData)
	require.NoError(t, err)

	payment := &lpnet.Payment{
		Sender:        orch.Address().Bytes(),
		ExpectedPrice: priceInfo,
	}
	paymentBytes, err := proto.Marshal(payment)
	require.NoError(t, err)

	headers := http.Header{}
	headers.Set(segmentHeader, base64.StdEncoding.EncodeToString(segBytes))
	headers.Set(paymentHeader, base64.StdEncoding.EncodeToString(paymentBytes))
	return headers
}

func scopeTestPricePerSecond(pricePerSecond int64) *lpnet.PriceInfo {
	return &lpnet.PriceInfo{
		PricePerUnit:  pricePerSecond,
		PixelsPerUnit: int64(defaultSegInfo.Height) * int64(defaultSegInfo.Width) * int64(defaultSegInfo.FPS),
	}
}
