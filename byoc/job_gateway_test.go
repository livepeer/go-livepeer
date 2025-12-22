package byoc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"

	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// stubJobOrchestratorPool is a stub implementation of the OrchestratorPool interface
type stubJobOrchestratorPool struct {
	uris  []*url.URL
	infos []common.OrchestratorLocalInfo
	node  *core.LivepeerNode
}

func newStubOrchestratorPool(node *core.LivepeerNode, uris []string) *stubJobOrchestratorPool {
	var urlList []*url.URL
	var infos []common.OrchestratorLocalInfo
	for _, uri := range uris {
		if u, err := url.Parse(uri); err == nil {
			urlList = append(urlList, u)
			infos = append(infos, common.OrchestratorLocalInfo{URL: u, Score: 1.0})
		}
	}
	return &stubJobOrchestratorPool{
		uris:  urlList,
		infos: infos,
		node:  mockJobLivepeerNode(),
	}
}

func (s *stubJobOrchestratorPool) GetInfos() []common.OrchestratorLocalInfo {
	var infos []common.OrchestratorLocalInfo
	for _, uri := range s.uris {
		infos = append(infos, common.OrchestratorLocalInfo{URL: uri})
	}
	return infos
}
func (s *stubJobOrchestratorPool) GetOrchestrators(ctx context.Context, max int, suspender common.Suspender, comparator common.CapabilityComparator, scorePred common.ScorePred) (common.OrchestratorDescriptors, error) {
	var ods common.OrchestratorDescriptors
	for _, uri := range s.uris {
		ods = append(ods, common.OrchestratorDescriptor{
			LocalInfo: &common.OrchestratorLocalInfo{URL: uri, Score: 1.0},
			RemoteInfo: &net.OrchestratorInfo{
				Transcoder: uri.String(),
			},
		})
	}
	return ods, nil
}
func (s *stubJobOrchestratorPool) Size() int {
	return len(s.uris)
}
func (s *stubJobOrchestratorPool) SizeWith(scorePred common.ScorePred) int {
	if scorePred == nil {
		return len(s.infos)
	}
	count := 0
	for _, info := range s.infos {
		if scorePred(info.Score) {
			count++
		}
	}
	return count
}

func (s *stubJobOrchestratorPool) Broadcaster() common.Broadcaster {
	return newMockJobOrchestrator()
}

func TestSubmitJob_MethodNotAllowed(t *testing.T) {
	bsg := &BYOCGatewayServer{
		node: mockJobLivepeerNode(),
	}

	req := httptest.NewRequest("GET", "/process/request/gg", nil)
	w := httptest.NewRecorder()

	handler := bsg.SubmitJob()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestCreatePayment(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.TODO()
		bsg := &BYOCGatewayServer{
			node:         mockJobLivepeerNode(),
			sharedBalMtx: &sync.Mutex{},
		}
		mockSender := pm.MockSender{}
		mockSender.On("StartSession", mock.Anything).Return("foo").Times(4)
		bsg.node.Sender = &mockSender

		bsg.node.Balances = core.NewAddressBalances(5 * time.Second)
		defer bsg.node.Balances.StopCleanup()

		jobReq := JobRequest{
			Capability: "test-payment-cap",
		}
		sender := JobSender{
			Addr: "0x1111111111111111111111111111111111111111",
			Sig:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		}

		//match to pm maxWinProb
		maxWinProb := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
		winProb10 := new(big.Int).Div(maxWinProb, big.NewInt(10))
		orchTocken := JobToken{
			TicketParams: &net.TicketParams{
				Recipient:         ethcommon.HexToAddress("0x1111111111111111111111111111111111111111").Bytes(),
				FaceValue:         big.NewInt(1000).Bytes(),
				WinProb:           winProb10.Bytes(),
				RecipientRandHash: []byte("hash"),
				Seed:              big.NewInt(1234).Bytes(),
				ExpirationBlock:   big.NewInt(100000).Bytes(),
			},
			SenderAddress: &sender,
			Balance:       0,
			Price: &net.PriceInfo{
				PricePerUnit:  100,
				PixelsPerUnit: 1,
			},
		}

		var pmTickets net.Payment

		//payment with one ticket
		mockSender.On("CreateTicketBatch", "foo", jobReq.Timeout).Return(mockTicketBatch(1), nil).Once()
		payment, err := bsg.createPayment(ctx, &jobReq, &orchTocken)
		assert.Nil(t, err)
		pmPayment, err := base64.StdEncoding.DecodeString(payment)
		assert.Nil(t, err)
		err = proto.Unmarshal(pmPayment, &pmTickets)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(pmTickets.TicketSenderParams))

		//test 2 tickets
		mockSender.On("CreateTicketBatch", "foo", jobReq.Timeout).Return(mockTicketBatch(2), nil).Once()
		payment, err = bsg.createPayment(ctx, &jobReq, &orchTocken)
		assert.Nil(t, err)
		pmPayment, err = base64.StdEncoding.DecodeString(payment)
		assert.Nil(t, err)
		err = proto.Unmarshal(pmPayment, &pmTickets)
		assert.Nil(t, err)
		assert.Equal(t, 2, len(pmTickets.TicketSenderParams))

		//test 600 tickets
		jobReq.Timeout = 600
		mockSender.On("CreateTicketBatch", "foo", jobReq.Timeout).Return(mockTicketBatch(600), nil).Once()
		orchTocken.Price.PricePerUnit = 6000
		payment, err = bsg.createPayment(ctx, &jobReq, &orchTocken)
		assert.Nil(t, err)
		pmPayment, err = base64.StdEncoding.DecodeString(payment)
		assert.Nil(t, err)
		err = proto.Unmarshal(pmPayment, &pmTickets)
		assert.Nil(t, err)
		assert.Equal(t, 600, len(pmTickets.TicketSenderParams))
	})
}

func createTestPayment(capability string) (string, error) {
	ctx := context.TODO()
	bsg := &BYOCGatewayServer{
		node:         mockJobLivepeerNode(),
		sharedBalMtx: &sync.Mutex{},
	}

	mockSender := pm.MockSender{}
	mockSender.On("StartSession", mock.Anything).Return("foo").Times(4)
	mockSender.On("CreateTicketBatch", "foo", 1).Return(mockTicketBatch(1), nil).Once()
	bsg.node.Sender = &mockSender

	bsg.node.Balances = core.NewAddressBalances(1 * time.Second)
	defer bsg.node.Balances.StopCleanup()

	jobReq := JobRequest{
		Capability: capability,
		Timeout:    1,
	}
	sender := JobSender{
		Addr: "0x1111111111111111111111111111111111111111",
		Sig:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
	}

	//match to pm maxWinProb
	maxWinProb := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	winProb10 := new(big.Int).Div(maxWinProb, big.NewInt(10))
	orchTocken := JobToken{
		TicketParams: &net.TicketParams{
			Recipient:         ethcommon.HexToAddress("0x1111111111111111111111111111111111111111").Bytes(),
			FaceValue:         big.NewInt(1000).Bytes(),
			WinProb:           winProb10.Bytes(),
			RecipientRandHash: []byte("hash"),
			Seed:              big.NewInt(1234).Bytes(),
			ExpirationBlock:   big.NewInt(100000).Bytes(),
		},
		SenderAddress: &sender,
		Balance:       0,
		Price: &net.PriceInfo{
			PricePerUnit:  100,
			PixelsPerUnit: 1,
		},
	}

	pmt, err := bsg.createPayment(ctx, &jobReq, &orchTocken)
	if err != nil {
		return "", err
	}

	return pmt, nil
}

func mockTicketBatch(count int) *pm.TicketBatch {
	senderParams := make([]*pm.TicketSenderParams, count)
	for i := 0; i < count; i++ {
		senderParams[i] = &pm.TicketSenderParams{
			SenderNonce: uint32(i + 1),
			Sig:         pm.RandBytes(42),
		}
	}

	return &pm.TicketBatch{
		TicketParams: &pm.TicketParams{
			Recipient:       pm.RandAddress(),
			FaceValue:       big.NewInt(1234),
			WinProb:         big.NewInt(5678),
			Seed:            big.NewInt(7777),
			ExpirationBlock: big.NewInt(1000),
		},
		TicketExpirationParams: &pm.TicketExpirationParams{},
		Sender:                 ethcommon.HexToAddress("0x1111111111111111111111111111111111111111"),
		SenderParams:           senderParams,
	}
}

func TestSubmitJob_OrchestratorSelectionParams(t *testing.T) {
	// Create mock HTTP servers for orchestrators
	mockServers := make([]*httptest.Server, 5)
	orchURLs := make([]string, 5)

	// Start HTTP test servers
	for i := 0; i < 5; i++ {
		server := httptest.NewServer(http.HandlerFunc(orchTokenHandler))
		mockServers[i] = server
		orchURLs[i] = server.URL
		t.Logf("Mock server %d started at %s", i, orchURLs[i])
	}

	// Clean up servers when test completes
	defer func() {
		for _, server := range mockServers {
			server.Close()
		}
	}()

	node := mockJobLivepeerNode()
	pool := newStubOrchestratorPool(node, orchURLs)
	node.OrchestratorPool = pool

	// Define test cases
	testCases := []struct {
		name          string
		include       []string
		exclude       []string
		expectedCount int
	}{
		{
			name:          "No filtering",
			include:       []string{},
			exclude:       []string{},
			expectedCount: 5, // All orchestrators
		},
		{
			name:          "Include specific orchestrators",
			include:       []string{orchURLs[0], orchURLs[2]}, // First and third servers
			exclude:       []string{},
			expectedCount: 2,
		},
		{
			name:          "Exclude specific orchestrators",
			include:       []string{},
			exclude:       []string{orchURLs[1], orchURLs[3]}, // Second and fourth servers
			expectedCount: 3,
		},
		{
			name:          "Both include and exclude",
			include:       []string{orchURLs[0], orchURLs[1], orchURLs[2]}, // First three servers
			exclude:       []string{orchURLs[1]},                           // Exclude second server
			expectedCount: 2,                                               // Should have first and third servers
		},
		{
			name:          "Include non-existent orchestrators",
			include:       []string{"http://nonexistent.example.com"},
			exclude:       []string{},
			expectedCount: 0,
		},
		{
			name:          "Exclude all orchestrators",
			include:       []string{},
			exclude:       orchURLs, // Exclude all servers
			expectedCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create JobParameters with the test case's filters
			params := JobParameters{
				Orchestrators: JobOrchestratorsFilter{
					Include: tc.include,
					Exclude: tc.exclude,
				},
			}

			// Call getJobOrchestrators
			tokens, err := getJobOrchestrators(
				context.Background(),
				node,
				"test-capability",
				params,
				100*time.Millisecond, // Short timeout for testing
				50*time.Millisecond,
			)

			if tc.expectedCount == 0 {
				// If we expect no orchestrators, we should still get a nil error
				// because the function should return an empty list, not an error
				assert.NoError(t, err)
				assert.Len(t, tokens, 0)
			} else {
				assert.NoError(t, err)
				assert.Len(t, tokens, tc.expectedCount)

				if len(tc.include) > 0 {
					for _, token := range tokens {
						assert.True(t, slices.Contains(tc.include, token.ServiceAddr))
					}
				}

				if len(tc.exclude) > 0 {
					for _, token := range tokens {
						assert.False(t, slices.Contains(tc.exclude, token.ServiceAddr))
					}
				}
			}
		})
	}

}

func TestSetupGatewayJob(t *testing.T) {
	// Prepare a JobRequest with valid fields
	jobDetails := JobRequestDetails{StreamId: "test-stream"}
	jobParams := JobParameters{
		Orchestrators:      JobOrchestratorsFilter{},
		EnableVideoIngress: true,
		EnableVideoEgress:  true,
		EnableDataOutput:   true,
	}
	jobReq := JobRequest{
		ID:         "job-1",
		Request:    marshalToString(t, jobDetails),
		Parameters: marshalToString(t, jobParams),
		Capability: "test-capability",
		Timeout:    10,
	}
	jobReqB, err := json.Marshal(jobReq)
	assert.NoError(t, err)
	jobReqB64 := base64.StdEncoding.EncodeToString(jobReqB)

	// Setup a minimal LivepeerServer with a stub OrchestratorPool
	server := httptest.NewServer(http.HandlerFunc(orchTokenHandler))
	defer server.Close()
	node := mockJobLivepeerNode()

	node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
	bsg := &BYOCGatewayServer{node: node}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(jobRequestHdr, jobReqB64)

	// Should succeed
	gatewayJob, err := bsg.setupGatewayJob(context.Background(), jobReqB64, "1s", "250ms", false)
	assert.NoError(t, err)
	assert.NotNil(t, gatewayJob)
	assert.Equal(t, "test-capability", gatewayJob.Job.Req.Capability)
	assert.Equal(t, "test-stream", gatewayJob.Job.Details.StreamId)
	assert.Equal(t, 10, gatewayJob.Job.Req.Timeout)
	assert.Equal(t, 1, len(gatewayJob.Orchs))

	//test signing request
	assert.Empty(t, gatewayJob.SignedJobReq)
	gatewayJob.sign()
	assert.NotEmpty(t, gatewayJob.SignedJobReq)

	// Should fail with invalid base64
	gatewayJob, err = bsg.setupGatewayJob(context.Background(), "not-base64", "1s", "250ms", false)
	assert.Error(t, err)
	assert.Nil(t, gatewayJob)

	// Should fail with missing orchestrators (simulate getJobOrchestrators returns empty)
	bsg.node.OrchestratorPool = newStubOrchestratorPool(node, []string{})
	gatewayJob, err = bsg.setupGatewayJob(context.Background(), jobReqB64, "1s", "250ms", false)
	assert.Error(t, err)
	assert.Nil(t, gatewayJob)
}
