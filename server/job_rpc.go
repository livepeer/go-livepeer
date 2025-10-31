package server

//based on segment_rpc.go

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"math/rand"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/net"

	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
)

const jobRequestHdr = "Livepeer"
const jobEthAddressHdr = "Livepeer-Eth-Address"
const jobCapabilityHdr = "Livepeer-Capability"
const jobPaymentHeaderHdr = "Livepeer-Payment"
const jobPaymentBalanceHdr = "Livepeer-Balance"
const jobOrchSearchTimeoutHdr = "Livepeer-Orch-Search-Timeout"
const jobOrchSearchRespTimeoutHdr = "Livepeer-Orch-Search-Resp-Timeout"
const jobOrchSearchTimeoutDefault = 1 * time.Second
const jobOrchSearchRespTimeoutDefault = 500 * time.Millisecond

var errNoTimeoutSet = errors.New("no timeout_seconds set with request, timeout_seconds is required")
var errNoCapabilityCapacity = errors.New("No capacity available for capability")
var errNoJobCreds = errors.New("Could not verify job creds")
var errPaymentError = errors.New("Could not parse payment")
var errInsufficientBalance = errors.New("Insufficient balance for request")

var sendJobReqWithTimeout = sendReqWithTimeout

type JobRequest struct {
	ID            string `json:"id"`
	Request       string `json:"request"`
	Parameters    string `json:"parameters"` //additional information for the Gateway to use to select orchestrators or to send to the worker
	Capability    string `json:"capability"`
	CapabilityUrl string `json:"capability_url"` //this is set when verified orch as capability
	Sender        string `json:"sender"`
	Sig           string `json:"sig"`
	Timeout       int    `json:"timeout_seconds"`

	OrchSearchTimeout     time.Duration
	OrchSearchRespTimeout time.Duration
}
type JobRequestDetails struct {
	StreamId string `json:"stream_id"`
}
type JobParameters struct {
	//Gateway
	Orchestrators JobOrchestratorsFilter `json:"orchestrators,omitempty"` //list of orchestrators to use for the job

	//Orchestrator
	EnableVideoIngress bool `json:"enable_video_ingress,omitempty"`
	EnableVideoEgress  bool `json:"enable_video_egress,omitempty"`
	EnableDataOutput   bool `json:"enable_data_output,omitempty"`
}
type JobOrchestratorsFilter struct {
	Exclude []string `json:"exclude,omitempty"`
	Include []string `json:"include,omitempty"`
}

type orchJob struct {
	Req     *JobRequest
	Details *JobRequestDetails
	Params  *JobParameters

	//Orchestrator fields
	Sender   ethcommon.Address
	JobPrice *net.PriceInfo
}
type gatewayJob struct {
	Job          *orchJob
	Orchs        []core.JobToken
	SignedJobReq string

	node *core.LivepeerNode
}

func (g *gatewayJob) sign() error {
	//sign the request
	gateway := g.node.OrchestratorPool.Broadcaster()
	sig, err := gateway.Sign([]byte(g.Job.Req.Request + g.Job.Req.Parameters))
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to sign request err=%v", err))
	}
	g.Job.Req.Sender = gateway.Address().Hex()
	g.Job.Req.Sig = "0x" + hex.EncodeToString(sig)

	//create the job request header with the signature
	jobReqEncoded, err := json.Marshal(g.Job.Req)
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to encode job request err=%v", err))
	}
	g.SignedJobReq = base64.StdEncoding.EncodeToString(jobReqEncoded)

	return nil
}

// worker registers to Orchestrator
func (h *lphttp) RegisterCapability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orch := h.orchestrator
	auth := r.Header.Get("Authorization")
	if auth != orch.TranscoderSecret() {
		http.Error(w, "invalid authorization", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	extCapSettings := string(body)
	remoteAddr := getRemoteAddr(r)

	cap, err := orch.RegisterExternalCapability(extCapSettings)

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		clog.Errorf(context.TODO(), "Error registering capability: %v", err)
		w.Write([]byte(fmt.Sprintf("Error registering capability: %v", err)))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
	clog.Infof(context.TODO(), "registered capability remoteAddr=%v capability=%v url=%v price=%v", remoteAddr, cap.Name, cap.Url, big.NewRat(cap.PricePerUnit, cap.PriceScaling))
}

func (h *lphttp) UnregisterCapability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	orch := h.orchestrator
	auth := r.Header.Get("Authorization")
	if auth != orch.TranscoderSecret() {
		http.Error(w, "invalid authorization", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	extCapName := string(body)
	remoteAddr := getRemoteAddr(r)

	err = orch.RemoveExternalCapability(extCapName)
	if err != nil {
		clog.Errorf(context.TODO(), "Error removing capability: %v", err)
		http.Error(w, fmt.Sprintf("Error removing capability: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("Error removing capability: %v", err)))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
	clog.Infof(context.TODO(), "removed capability remoteAddr=%v capability=%v", remoteAddr, extCapName)
}

func (h *lphttp) GetJobToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.node.NodeType != core.OrchestratorNode {
		http.Error(w, "request not allowed", http.StatusBadRequest)
	}

	remoteAddr := getRemoteAddr(r)

	orch := h.orchestrator

	jobEthAddrHdr := r.Header.Get(jobEthAddressHdr)
	if jobEthAddrHdr == "" {
		glog.Infof("generate token failed, invalid request remoteAddr=%v", remoteAddr)
		http.Error(w, fmt.Sprintf("Must have eth address and signature on address in Livepeer-Eth-Address header"), http.StatusBadRequest)
		return
	}
	jobSenderAddr, err := verifyTokenCreds(r.Context(), orch, jobEthAddrHdr)
	if err != nil {
		glog.Infof("generate token failed, invalid request with bad eth address header remoteAddr=%v", remoteAddr)
		http.Error(w, fmt.Sprintf("Invalid eth address header "), http.StatusBadRequest)
		return
	}

	jobCapsHdr := r.Header.Get(jobCapabilityHdr)
	if jobCapsHdr == "" {
		glog.Infof("generate token failed, invalid request, no capabilities included remoteAddr=%v", remoteAddr)
		http.Error(w, fmt.Sprintf("Job capabilities not provided, must provide comma separated capabilities in Livepeer-Capability header"), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	jobToken := core.JobToken{SenderAddress: nil, TicketParams: nil, Balance: 0, Price: nil}

	if !orch.CheckExternalCapabilityCapacity(jobCapsHdr) {
		//send response indicating no capacity available
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		senderAddr := ethcommon.HexToAddress(jobSenderAddr.Addr)

		jobPrice, err := orch.JobPriceInfo(senderAddr, jobCapsHdr)
		if err != nil {
			statusCode := http.StatusBadRequest
			if err.Error() == "insufficient sender reserve" {
				statusCode = http.StatusServiceUnavailable
			}
			glog.Errorf("could not get price err=%v", err.Error())
			http.Error(w, fmt.Sprintf("Could not get price err=%v", err.Error()), statusCode)
			return
		}
		ticketParams, err := orch.TicketParams(senderAddr, jobPrice)
		if err != nil {
			glog.Errorf("could not get ticket params err=%v", err.Error())
			http.Error(w, fmt.Sprintf("Could not get ticket params err=%v", err.Error()), http.StatusBadRequest)
			return
		}

		capBal := orch.Balance(senderAddr, core.ManifestID(jobCapsHdr))
		if capBal != nil {
			capBal, err = common.PriceToInt64(capBal)
			if err != nil {
				clog.Errorf(context.TODO(), "could not convert balance to int64 sender=%v capability=%v err=%v", senderAddr.Hex(), jobCapsHdr, err.Error())
				capBal = big.NewRat(0, 1)
			}
		} else {
			capBal = big.NewRat(0, 1)
		}
		//convert to int64. Note: returns with 000 more digits to allow for precision of 3 decimal places.
		capBalInt, err := common.PriceToFixed(capBal)
		if err != nil {
			glog.Errorf("could not convert balance to int64 sender=%v capability=%v err=%v", senderAddr.Hex(), jobCapsHdr, err.Error())
			capBalInt = 0
		} else {
			// Remove the last three digits from capBalInt
			capBalInt = capBalInt / 1000
		}

		jobToken = core.JobToken{
			SenderAddress: jobSenderAddr,
			TicketParams:  ticketParams,
			Balance:       capBalInt,
			Price:         jobPrice,
			ServiceAddr:   orch.ServiceURI().String(),
		}

		//send response indicating compatible
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(jobToken)
}

func (ls *LivepeerServer) setupGatewayJob(ctx context.Context, r *http.Request, skipOrchSearch bool) (*gatewayJob, error) {

	var orchs []core.JobToken

	jobReqHdr := r.Header.Get(jobRequestHdr)
	clog.Infof(ctx, "processing job request req=%v", jobReqHdr)
	jobReq, err := verifyJobCreds(ctx, nil, jobReqHdr, true)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to parse job request, err=%v", err))
	}

	var jobDetails JobRequestDetails
	if err := json.Unmarshal([]byte(jobReq.Request), &jobDetails); err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to unmarshal job request err=%v", err))
	}

	var jobParams JobParameters
	if err := json.Unmarshal([]byte(jobReq.Parameters), &jobParams); err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to unmarshal job parameters err=%v", err))
	}

	// get list of Orchestrators that can do the job if needed
	// (e.g. stop requests don't need new list of orchestrators)
	if !skipOrchSearch {
		searchTimeout, respTimeout := getOrchSearchTimeouts(ctx, r.Header.Get(jobOrchSearchTimeoutHdr), r.Header.Get(jobOrchSearchRespTimeoutHdr))
		jobReq.OrchSearchTimeout = searchTimeout
		jobReq.OrchSearchRespTimeout = respTimeout

		//get pool of Orchestrators that can do the job
		orchs, err = getJobOrchestrators(ctx, ls.LivepeerNode, jobReq.Capability, jobParams, jobReq.OrchSearchTimeout, jobReq.OrchSearchRespTimeout)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Unable to find orchestrators for capability %v err=%v", jobReq.Capability, err))
		}

		if len(orchs) == 0 {
			return nil, errors.New(fmt.Sprintf("No orchestrators found for capability %v", jobReq.Capability))
		}
	}

	job := orchJob{Req: jobReq,
		Details: &jobDetails,
		Params:  &jobParams,
	}

	return &gatewayJob{Job: &job, Orchs: orchs, node: ls.LivepeerNode}, nil
}

func (h *lphttp) ProcessJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Orchestrator node
	processJob(ctx, h, w, r)
}

// Gateway handler for job request
func (ls *LivepeerServer) SubmitJob() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Gateway node
		ls.submitJob(ctx, w, r)
	})
}

func (ls *LivepeerServer) submitJob(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	gatewayJob, err := ls.setupGatewayJob(ctx, r, false)
	if err != nil {
		clog.Errorf(ctx, "Error setting up job: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	clog.Infof(ctx, "Job request setup complete details=%v params=%v", gatewayJob.Job.Details, gatewayJob.Job.Params)

	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to setup job err=%v", err), http.StatusBadRequest)
		return
	}
	ctx = clog.AddVal(ctx, "job_id", gatewayJob.Job.Req.ID)
	ctx = clog.AddVal(ctx, "capability", gatewayJob.Job.Req.Capability)
	// Read the original request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	//send the request to the Orchestrator(s)
	//the loop ends on Gateway error and bad request errors
	for _, orchToken := range gatewayJob.Orchs {

		// Extract the worker resource route from the URL path
		// The prefix is "/process/request/"
		// if the request does not include the last / of the prefix no additional url path is added
		workerRoute := orchToken.ServiceAddr + "/process/request"
		prefix := "/process/request/"
		workerResourceRoute := r.URL.Path
		if strings.HasPrefix(workerResourceRoute, prefix) {
			workerResourceRoute = workerResourceRoute[len(prefix):]
		}
		if workerResourceRoute != "" {
			workerRoute = workerRoute + "/" + workerResourceRoute
		}

		err := gatewayJob.sign()
		if err != nil {
			clog.Errorf(ctx, "Error signing job, exiting stream processing request: %v", err)
			return
		}

		start := time.Now()
		resp, code, err := ls.sendJobToOrch(ctx, r, gatewayJob.Job.Req, gatewayJob.SignedJobReq, orchToken, workerResourceRoute, body)
		if err != nil {
			clog.Errorf(ctx, "job not able to be processed by Orchestrator %v err=%v ", orchToken.ServiceAddr, err.Error())
			continue
		}

		//error response from Orchestrator
		if code > 399 {
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				clog.Errorf(ctx, "Unable to read response err=%v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				continue
			}
			clog.Errorf(ctx, "error processing request err=%v ", string(data))
			//nonretryable error
			if code < 500 {
				//assume non retryable bad request
				//return error response from the worker
				http.Error(w, string(data), code)
				return
			}
			//retryable error, continue to next orchestrator
			continue
		}

		//Orchestrator returns Livepeer-Balance header for streaming and non-streaming responses
		// for streaming responses: the balance is the balance before deducting cost to finish the request
		//                          the ending balance is sent as last line before [DONE] in the SSE stream
		// for non-streaming: the balance is the balance after deducting the cost of the request
		orchBalance := resp.Header.Get(jobPaymentBalanceHdr)
		w.Header().Set(jobPaymentBalanceHdr, orchBalance)
		w.Header().Set("X-Metadata", resp.Header.Get("X-Metadata"))
		w.Header().Set("X-Orchestrator-Url", orchToken.ServiceAddr)

		if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
			//non streaming response
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				clog.Errorf(ctx, "Unable to read response err=%v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				continue
			}

			gatewayBalance := updateGatewayBalance(ls.LivepeerNode, orchToken, gatewayJob.Job.Req.Capability, time.Since(start))
			clog.V(common.SHORT).Infof(ctx, "Job processed successfully took=%v balance=%v balance_from_orch=%v", time.Since(start), gatewayBalance.FloatString(0), orchBalance)
			w.Write(data)
			return
		} else {
			// Handle streaming response (SSE)
			clog.Infof(ctx, "received streaming response")

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			// Flush to ensure data is sent immediately
			flusher, ok := w.(http.Flusher)
			if !ok {
				clog.Errorf(ctx, "streaming not supported")
				http.Error(w, "Streaming not supported", http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			// Read from upstream and forward to client
			respChan := make(chan string, 100)
			respCtx, _ := context.WithTimeout(ctx, time.Duration(gatewayJob.Job.Req.Timeout+10)*time.Second) //include a small buffer to let Orchestrator close the connection on the timeout

			go func() {
				defer resp.Body.Close()
				defer close(respChan)
				scanner := bufio.NewScanner(resp.Body)

				for scanner.Scan() {
					select {
					case <-respCtx.Done():
						respChan <- "data: [DONE]\n\n"
						return
					default:
						line := scanner.Text()
						respChan <- line
						if strings.Contains(line, "[DONE]") {
							break
						}
					}
				}
			}()

			orchBalance := big.NewRat(0, 1)
		proxyResp:
			for {
				select {
				case line := <-respChan:
					w.Write([]byte(line + "\n"))
					flusher.Flush()
					if strings.Contains(line, "balance:") {
						orchBalance = parseBalance(line)
					}
					if strings.Contains(line, "[DONE]") {
						break proxyResp
					}

				case <-respCtx.Done():
					break proxyResp
				}
			}

			gatewayBalance := updateGatewayBalance(ls.LivepeerNode, orchToken, gatewayJob.Job.Req.Capability, time.Since(start))

			clog.V(common.SHORT).Infof(ctx, "Job processed successfully took=%v balance=%v balance_from_orch=%v", time.Since(start), gatewayBalance.FloatString(0), orchBalance.FloatString(0))
		}
	}
}

func (ls *LivepeerServer) sendJobToOrch(ctx context.Context, r *http.Request, jobReq *JobRequest, signedReqHdr string, orchToken core.JobToken, route string, body []byte) (*http.Response, int, error) {
	orchUrl := orchToken.ServiceAddr + route
	req, err := http.NewRequestWithContext(ctx, "POST", orchUrl, bytes.NewBuffer(body))
	if err != nil {
		clog.Errorf(ctx, "Unable to create request err=%v", err)
		return nil, http.StatusInternalServerError, err
	}

	// set the headers
	if r != nil {
		req.Header.Add("Content-Length", r.Header.Get("Content-Length"))
		req.Header.Add("Content-Type", r.Header.Get("Content-Type"))
	} else {
		//this is for live requests which will be json to start stream
		// update requests should include the content type/length
		req.Header.Add("Content-Type", "application/json")
	}

	req.Header.Add(jobRequestHdr, signedReqHdr)
	if orchToken.Price.PricePerUnit > 0 {
		paymentHdr, err := createPayment(ctx, jobReq, &orchToken, ls.LivepeerNode)
		if err != nil {
			clog.Errorf(ctx, "Unable to create payment err=%v", err)
			return nil, http.StatusInternalServerError, fmt.Errorf("Unable to create payment err=%v", err)
		}
		if paymentHdr != "" {
			req.Header.Add(jobPaymentHeaderHdr, paymentHdr)
		}
	}

	resp, err := sendJobReqWithTimeout(req, time.Duration(jobReq.Timeout+5)*time.Second) //include 5 second buffer
	if err != nil {
		clog.Errorf(ctx, "job not able to be processed by Orchestrator %v err=%v ", orchToken.ServiceAddr, err.Error())
		return nil, http.StatusBadRequest, err
	}

	return resp, resp.StatusCode, nil
}

func (ls *LivepeerServer) sendPayment(ctx context.Context, orchPmtUrl, capability, jobReq, payment string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", orchPmtUrl, nil)
	if err != nil {
		clog.Errorf(ctx, "Unable to create request err=%v", err)
		return http.StatusBadRequest, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add(jobRequestHdr, jobReq)
	req.Header.Add(jobPaymentHeaderHdr, payment)

	resp, err := sendJobReqWithTimeout(req, 10*time.Second)
	if err != nil {
		clog.Errorf(ctx, "job payment not able to be processed by Orchestrator %v err=%v ", orchPmtUrl, err.Error())
		return http.StatusBadRequest, err
	}

	return resp.StatusCode, nil
}

func processJob(ctx context.Context, h *lphttp, w http.ResponseWriter, r *http.Request) {
	remoteAddr := getRemoteAddr(r)
	ctx = clog.AddVal(ctx, "client_ip", remoteAddr)
	orch := h.orchestrator
	// check the prompt sig from the request
	// confirms capacity available before processing payment info
	orchJob, err := h.setupOrchJob(ctx, r, true)
	if err != nil {
		if err == errNoCapabilityCapacity {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}
	taskId := core.RandomManifestID()
	ctx = clog.AddVal(ctx, "job_id", orchJob.Req.ID)
	ctx = clog.AddVal(ctx, "worker_task_id", string(taskId))
	ctx = clog.AddVal(ctx, "capability", orchJob.Req.Capability)
	ctx = clog.AddVal(ctx, "sender", orchJob.Req.Sender)
	clog.V(common.SHORT).Infof(ctx, "Received job, sending for processing")

	// Read the original body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	// Extract the worker resource route from the URL path
	// The prefix is "/process/request/"
	// if the request does not include the last / of the prefix no additional url path is added
	prefix := "/process/request/"
	workerResourceRoute := r.URL.Path
	if strings.HasPrefix(workerResourceRoute, prefix) {
		workerResourceRoute = workerResourceRoute[len(prefix):]
	}

	workerRoute := orchJob.Req.CapabilityUrl
	if workerResourceRoute != "" {
		workerRoute = workerRoute + "/" + workerResourceRoute
	}

	req, err := http.NewRequestWithContext(ctx, "POST", workerRoute, bytes.NewBuffer(body))
	if err != nil {
		clog.Errorf(ctx, "Unable to create request err=%v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	// set the headers
	req.Header.Add("Content-Length", r.Header.Get("Content-Length"))
	req.Header.Add("Content-Type", r.Header.Get("Content-Type"))

	start := time.Now()
	resp, err := sendReqWithTimeout(req, time.Duration(orchJob.Req.Timeout)*time.Second)
	if err != nil {
		clog.Errorf(ctx, "job not able to be processed err=%v ", err.Error())
		//if the request failed with connection error, remove the capability
		//exclude deadline exceeded or context canceled errors does not indicate a fatal error all the time
		if err != context.DeadlineExceeded && !strings.Contains(err.Error(), "context canceled") {
			clog.Errorf(ctx, "removing capability %v due to error %v", orchJob.Req.Capability, err.Error())
			h.orchestrator.RemoveExternalCapability(orchJob.Req.Capability)
		}

		chargeForCompute(start, orchJob.JobPrice, orch, orchJob.Sender, orchJob.Req.Capability)
		w.Header().Set(jobPaymentBalanceHdr, getPaymentBalance(orch, orchJob.Sender, orchJob.Req.Capability).FloatString(0))
		http.Error(w, fmt.Sprintf("job not able to be processed, removing capability err=%v", err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("X-Metadata", resp.Header.Get("X-Metadata"))

	//release capacity for another request
	// if requester closes the connection need to release capacity
	defer orch.FreeExternalCapabilityCapacity(orchJob.Req.Capability)

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		//non streaming response

		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			clog.Errorf(ctx, "Unable to read response err=%v", err)

			chargeForCompute(start, orchJob.JobPrice, orch, orchJob.Sender, orchJob.Req.Capability)
			w.Header().Set(jobPaymentBalanceHdr, getPaymentBalance(orch, orchJob.Sender, orchJob.Req.Capability).FloatString(0))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		//error response from worker but assume can retry and pass along error response and status code
		if resp.StatusCode > 399 {
			clog.Errorf(ctx, "error processing request err=%v ", string(data))

			chargeForCompute(start, orchJob.JobPrice, orch, orchJob.Sender, orchJob.Req.Capability)
			w.Header().Set(jobPaymentBalanceHdr, getPaymentBalance(orch, orchJob.Sender, orchJob.Req.Capability).FloatString(0))
			//return error response from the worker
			http.Error(w, string(data), resp.StatusCode)
			return
		}

		chargeForCompute(start, orchJob.JobPrice, orch, orchJob.Sender, orchJob.Req.Capability)
		w.Header().Set(jobPaymentBalanceHdr, getPaymentBalance(orch, orchJob.Sender, orchJob.Req.Capability).FloatString(0))
		clog.V(common.SHORT).Infof(ctx, "Job processed successfully took=%v balance=%v", time.Since(start), getPaymentBalance(orch, orchJob.Sender, orchJob.Req.Capability).FloatString(0))
		w.Write(data)
		//request completed and returned a response

		return
	} else {
		// Handle streaming response (SSE)
		clog.Infof(ctx, "received streaming response")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		//send payment balance back so client can determine if payment is needed
		addPaymentBalanceHeader(w, orch, orchJob.Sender, orchJob.Req.Capability)

		// Flush to ensure data is sent immediately
		flusher, ok := w.(http.Flusher)
		if !ok {
			clog.Errorf(ctx, "streaming not supported")

			chargeForCompute(start, orchJob.JobPrice, orch, orchJob.Sender, orchJob.Req.Capability)
			w.Header().Set(jobPaymentBalanceHdr, getPaymentBalance(orch, orchJob.Sender, orchJob.Req.Capability).FloatString(0))
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Read from upstream and forward to client
		respChan := make(chan string, 100)
		respCtx, _ := context.WithTimeout(ctx, time.Duration(orchJob.Req.Timeout)*time.Second)

		go func() {
			defer resp.Body.Close()
			defer close(respChan)
			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				select {
				case <-respCtx.Done():
					orchBal := orch.Balance(orchJob.Sender, core.ManifestID(orchJob.Req.Capability))
					if orchBal == nil {
						orchBal = big.NewRat(0, 1)
					}
					respChan <- fmt.Sprintf("data: {\"balance\": %v}\n\n", orchBal.FloatString(3))
					respChan <- "data: [DONE]\n\n"
					return
				default:
					line := scanner.Text()
					if strings.Contains(line, "[DONE]") {
						orchBal := orch.Balance(orchJob.Sender, core.ManifestID(orchJob.Req.Capability))
						if orchBal == nil {
							orchBal = big.NewRat(0, 1)
						}
						respChan <- fmt.Sprintf("data: {\"balance\": %v}\n\n", orchBal.FloatString(3))
						respChan <- scanner.Text()
						break
					}
					respChan <- scanner.Text()
				}
			}
		}()

		//check for payment balance
		pmtWatcher := time.NewTicker(5 * time.Second)
		defer pmtWatcher.Stop()
	proxyResp:
		for {
			select {
			case <-pmtWatcher.C:
				//check balance and end response if out of funds
				//skips if price is 0
				jobPriceRat := big.NewRat(orchJob.JobPrice.PricePerUnit, orchJob.JobPrice.PixelsPerUnit)
				if jobPriceRat.Cmp(big.NewRat(0, 1)) > 0 {
					h.orchestrator.DebitFees(orchJob.Sender, core.ManifestID(orchJob.Req.Capability), orchJob.JobPrice, 5)
					senderBalance := getPaymentBalance(orch, orchJob.Sender, orchJob.Req.Capability)
					if senderBalance != nil {
						if senderBalance.Cmp(big.NewRat(0, 1)) < 0 {
							w.Write([]byte("event: insufficient balance\n"))
							w.Write([]byte("data: {\"balance\": 0}\n\n"))
							w.Write([]byte("data: [DONE]\n\n"))
							flusher.Flush()
							break proxyResp
						}
					}
				}
			case <-respCtx.Done():
				break proxyResp
			case line := <-respChan:
				w.Write([]byte(line + "\n"))
				flusher.Flush()
			}
		}

		//capacity released with defer stmt above
		clog.V(common.SHORT).Infof(ctx, "Job processed successfully took=%v balance=%v", time.Since(start), getPaymentBalance(orch, orchJob.Sender, orchJob.Req.Capability).FloatString(0))
	}
}

// SetupOrchJob prepares the orchestrator job by extracting and validating the job request from the HTTP headers.
// Payment is applied if applicable.
func (h *lphttp) setupOrchJob(ctx context.Context, r *http.Request, reserveCapacity bool) (*orchJob, error) {
	job := r.Header.Get(jobRequestHdr)
	orch := h.orchestrator
	jobReq, err := verifyJobCreds(ctx, orch, job, reserveCapacity)
	if err != nil {
		if err == errZeroCapacity && reserveCapacity {
			return nil, errNoCapabilityCapacity
		} else if err == errNoTimeoutSet {
			return nil, errNoTimeoutSet
		} else {
			clog.Errorf(ctx, "job failed verification: %v", err)
			return nil, errNoJobCreds
		}
	}

	sender := ethcommon.HexToAddress(jobReq.Sender)

	jobPrice, err := orch.JobPriceInfo(sender, jobReq.Capability)
	if err != nil {
		return nil, errors.New("Could not get job price")
	}
	clog.V(common.DEBUG).Infof(ctx, "job price=%v units=%v", jobPrice.PricePerUnit, jobPrice.PixelsPerUnit)

	//no payment included, confirm if balance remains
	jobPriceRat := big.NewRat(jobPrice.PricePerUnit, jobPrice.PixelsPerUnit)
	orchBal := big.NewRat(0, 1)
	// if price is 0, no payment required
	if jobPriceRat.Cmp(big.NewRat(0, 1)) > 0 {
		minBal := new(big.Rat).Mul(jobPriceRat, big.NewRat(60, 1)) //minimum 1 minute balance
		//process payment if included
		orchBal, pmtErr := processPayment(ctx, orch, sender, jobReq.Capability, r.Header.Get(jobPaymentHeaderHdr))
		if pmtErr != nil {
			//log if there are payment errors but continue, balance will runout and clean up
			clog.Infof(ctx, "job payment error: %v", pmtErr)
		}

		if orchBal.Cmp(minBal) < 0 {
			orch.FreeExternalCapabilityCapacity(jobReq.Capability)
			return nil, errInsufficientBalance
		}
	}

	var jobDetails JobRequestDetails
	err = json.Unmarshal([]byte(jobReq.Request), &jobDetails)
	if err != nil {
		return nil, fmt.Errorf("Unable to unmarshal job request details err=%v", err)
	}

	clog.Infof(ctx, "job request verified id=%v sender=%v capability=%v timeout=%v price=%v balance=%v", jobReq.ID, jobReq.Sender, jobReq.Capability, jobReq.Timeout, jobPriceRat.FloatString(0), orchBal.FloatString(0))

	return &orchJob{Req: jobReq, Sender: sender, JobPrice: jobPrice, Details: &jobDetails}, nil
}

// process payment and return balance
func processPayment(ctx context.Context, orch Orchestrator, sender ethcommon.Address, capability string, paymentHdr string) (*big.Rat, error) {
	if paymentHdr != "" {
		payment, err := getPayment(paymentHdr)
		if err != nil {
			clog.Errorf(ctx, "job payment invalid: %v", err)
			return nil, errPaymentError
		}

		if err := orch.ProcessPayment(ctx, payment, core.ManifestID(capability)); err != nil {
			orch.FreeExternalCapabilityCapacity(capability)
			clog.Errorf(ctx, "Error processing payment: %v", err)
			return nil, errPaymentError
		}
	}
	orchBal := getPaymentBalance(orch, sender, capability)

	return orchBal, nil

}

func createPayment(ctx context.Context, jobReq *JobRequest, orchToken *core.JobToken, node *core.LivepeerNode) (string, error) {
	if orchToken == nil {
		return "", errors.New("orchestrator token is nil, cannot create payment")
	}
	//if no sender or ticket params, no payment
	if node.Sender == nil {
		return "", errors.New("no ticket sender available, cannot create payment")
	}
	if orchToken.TicketParams == nil {
		return "", errors.New("no ticket params available, cannot create payment")
	}

	var payment *net.Payment
	createTickets := true
	clog.Infof(ctx, "creating payment for job request %s", jobReq.Capability)
	sender := ethcommon.HexToAddress(jobReq.Sender)

	orchAddr := ethcommon.BytesToAddress(orchToken.TicketParams.Recipient)
	sessionID := node.Sender.StartSession(*pmTicketParams(orchToken.TicketParams))

	//setup balances and update Gateway balance to Orchestrator balance, log differences
	//Orchestrator tracks balance paid and will not perform work if the balance it
	//has is not sufficient
	orchBal := big.NewRat(orchToken.Balance, 1)
	price := big.NewRat(orchToken.Price.PricePerUnit, orchToken.Price.PixelsPerUnit)
	cost := new(big.Rat).Mul(price, big.NewRat(int64(jobReq.Timeout), 1))
	minBal := new(big.Rat).Mul(price, big.NewRat(60, 1)) //minimum 1 minute balance
	balance, diffToOrch, minBalCovered, resetToZero := node.Balances.CompareAndUpdateBalance(orchAddr, core.ManifestID(jobReq.Capability), orchBal, minBal)

	if diffToOrch.Sign() != 0 {
		clog.Infof(ctx, "Updated balance for sender=%v capability=%v by %v to match Orchestrator reported balance %v", sender.Hex(), jobReq.Capability, diffToOrch.FloatString(3), orchBal.FloatString(3))
	}
	if resetToZero {
		clog.Infof(ctx, "Reset balance to zero for to match Orchestrator reported balance sender=%v capability=%v", sender.Hex(), jobReq.Capability)
	}
	if minBalCovered {
		createTickets = false
		payment = &net.Payment{
			Sender:        sender.Bytes(),
			ExpectedPrice: orchToken.Price,
		}
	}
	clog.V(common.DEBUG).Infof(ctx, "current balance for sender=%v capability=%v is %v, cost=%v price=%v", sender.Hex(), jobReq.Capability, balance.FloatString(3), cost.FloatString(3), price.FloatString(3))

	if !createTickets {
		clog.V(common.DEBUG).Infof(ctx, "No payment required, using balance=%v", balance.FloatString(3))
		return "", nil
	} else {
		//calc ticket count
		ticketCnt := math.Ceil(float64(jobReq.Timeout))
		tickets, err := node.Sender.CreateTicketBatch(sessionID, int(ticketCnt))
		if err != nil {
			clog.Errorf(ctx, "Unable to create ticket batch err=%v", err)
			return "", err
		}

		//record the payment
		winProb := tickets.WinProbRat()
		fv := big.NewRat(tickets.FaceValue.Int64(), 1)
		pmtTotal := new(big.Rat).Mul(fv, winProb)
		pmtTotal = new(big.Rat).Mul(pmtTotal, big.NewRat(int64(ticketCnt), 1))
		node.Balances.Credit(orchAddr, core.ManifestID(jobReq.Capability), pmtTotal)
		//create the payment
		payment = &net.Payment{
			Sender:        sender.Bytes(),
			ExpectedPrice: orchToken.Price,
			TicketParams:  orchToken.TicketParams,
			ExpirationParams: &net.TicketExpirationParams{
				CreationRound:          tickets.CreationRound,
				CreationRoundBlockHash: tickets.CreationRoundBlockHash.Bytes(),
			},
		}

		totalEV := big.NewRat(0, 1)
		senderParams := make([]*net.TicketSenderParams, len(tickets.SenderParams))
		for i := 0; i < len(tickets.SenderParams); i++ {
			senderParams[i] = &net.TicketSenderParams{
				SenderNonce: orchToken.LastNonce + tickets.SenderParams[i].SenderNonce,
				Sig:         tickets.SenderParams[i].Sig,
			}
			totalEV = totalEV.Add(totalEV, tickets.WinProbRat())
		}
		orchToken.LastNonce = tickets.SenderParams[len(tickets.SenderParams)-1].SenderNonce + 1
		payment.TicketSenderParams = senderParams

		ratPrice, _ := common.RatPriceInfo(payment.ExpectedPrice)
		balanceForOrch := node.Balances.Balance(orchAddr, core.ManifestID(jobReq.Capability))
		balanceForOrchStr := ""
		if balanceForOrch != nil {
			balanceForOrchStr = balanceForOrch.FloatString(3)
		}

		clog.V(common.DEBUG).Infof(ctx, "Created new payment - capability=%v recipient=%v faceValue=%v winProb=%v price=%v numTickets=%v balance=%v",
			jobReq.Capability,
			tickets.Recipient.Hex(),
			eth.FormatUnits(tickets.FaceValue, "ETH"),
			tickets.WinProbRat().FloatString(10),
			ratPrice.FloatString(3)+" wei/unit",
			ticketCnt,
			balanceForOrchStr,
		)
	}

	data, err := proto.Marshal(payment)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

func updateGatewayBalance(node *core.LivepeerNode, orchToken core.JobToken, capability string, took time.Duration) *big.Rat {
	orchAddr := ethcommon.BytesToAddress(orchToken.TicketParams.Recipient)
	// update for usage of compute
	orchPrice := big.NewRat(orchToken.Price.PricePerUnit, orchToken.Price.PixelsPerUnit)
	cost := new(big.Rat).Mul(orchPrice, big.NewRat(int64(math.Ceil(took.Seconds())), 1))
	node.Balances.Debit(orchAddr, core.ManifestID(capability), cost)

	//get the updated balance
	balance := node.Balances.Balance(orchAddr, core.ManifestID(capability))
	if balance == nil {
		return big.NewRat(0, 1)
	}

	return balance
}

func parseBalance(line string) *big.Rat {
	balanceStr := strings.Replace(line, "balance:", "", 1)
	balanceStr = strings.Replace(line, "data:", "", 1)
	balanceStr = strings.TrimSpace(balanceStr)

	balance, ok := new(big.Rat).SetString(balanceStr)
	if ok {
		return balance
	} else {
		return big.NewRat(0, 1)
	}
}

func chargeForCompute(start time.Time, price *net.PriceInfo, orch Orchestrator, sender ethcommon.Address, jobId string) {
	// Debit the fee for the total time processed
	took := time.Since(start)
	orch.DebitFees(sender, core.ManifestID(jobId), price, int64(math.Ceil(took.Seconds())))
}

func addPaymentBalanceHeader(w http.ResponseWriter, orch Orchestrator, sender ethcommon.Address, jobId string) {
	//check balance and return remaning balance in header of response
	senderBalance := getPaymentBalance(orch, sender, jobId)
	if senderBalance != nil {
		w.Header().Set("Livepeer-Payment-Balance", senderBalance.FloatString(0))
		return
	}
	glog.Infof("sender balance is nil")
	//no balance
	w.Header().Set("Livepeer-Payment-Balance", "0")
}

func getPaymentBalance(orch Orchestrator, sender ethcommon.Address, jobId string) *big.Rat {
	//check balance and return remaning balance in header of response
	senderBalance := orch.Balance(sender, core.ManifestID(jobId))
	if senderBalance == nil {
		senderBalance = big.NewRat(0, 1)
	}

	return senderBalance
}

func verifyTokenCreds(ctx context.Context, orch Orchestrator, tokenCreds string) (*core.JobSender, error) {
	buf, err := base64.StdEncoding.DecodeString(tokenCreds)
	if err != nil {
		glog.Error("Unable to base64-decode ", err)
		return nil, errSegEncoding
	}

	var jobSender core.JobSender
	err = json.Unmarshal(buf, &jobSender)
	if err != nil {
		clog.Errorf(ctx, "Unable to parse the header text: ", err)
		return nil, err
	}

	sigHex := jobSender.Sig
	if len(jobSender.Sig) > 130 {
		sigHex = jobSender.Sig[2:]
	}
	sigByte, err := hex.DecodeString(sigHex)
	if err != nil {
		clog.Errorf(ctx, "Unable to hex-decode signature", err)
		return nil, errSegSig
	}

	if !orch.VerifySig(ethcommon.HexToAddress(jobSender.Addr), jobSender.Addr, sigByte) {
		clog.Errorf(ctx, "Sig check failed")
		return nil, errSegSig
	}

	//signature confirmed
	return &jobSender, nil
}

func parseJobRequest(jobReq string) (*JobRequest, error) {
	buf, err := base64.StdEncoding.DecodeString(jobReq)
	if err != nil {
		glog.Error("Unable to base64-decode ", err)
		return nil, errSegEncoding
	}

	var jobData JobRequest
	err = json.Unmarshal(buf, &jobData)
	if err != nil {
		glog.Error("Unable to unmarshal ", err)
		return nil, err
	}

	if jobData.Timeout == 0 {
		return nil, errNoTimeoutSet
	}

	return &jobData, nil
}

func verifyJobCreds(ctx context.Context, orch Orchestrator, jobCreds string, reserveCapacity bool) (*JobRequest, error) {
	//Gateway needs JobRequest parsed and verification of required fields
	jobData, err := parseJobRequest(jobCreds)
	if err != nil {
		glog.Error("Unable to unmarshal ", err)
		return nil, err
	}

	if jobData.Timeout == 0 {
		return nil, errNoTimeoutSet
	}

	//Orchestrator also verifies the signature of the request to confirm
	//jobReq.Sender actually sent the request
	if orch != nil {
		sigHex := jobData.Sig
		if len(jobData.Sig) > 130 {
			sigHex = jobData.Sig[2:]
		}
		sigByte, err := hex.DecodeString(sigHex)
		if err != nil {
			clog.Errorf(ctx, "Unable to hex-decode signature", err)
			return nil, errSegSig
		}

		if !orch.VerifySig(ethcommon.HexToAddress(jobData.Sender), jobData.Request+jobData.Parameters, sigByte) {
			clog.Errorf(ctx, "Sig check failed sender=%v", jobData.Sender)
			return nil, errSegSig
		}

		if reserveCapacity && orch.ReserveExternalCapabilityCapacity(jobData.Capability) != nil {
			return nil, errZeroCapacity
		}

		jobData.CapabilityUrl = orch.GetUrlForCapability(jobData.Capability)
	}

	return jobData, nil
}

func getOrchSearchTimeouts(ctx context.Context, searchTimeoutHdr, respTimeoutHdr string) (time.Duration, time.Duration) {
	timeout := jobOrchSearchTimeoutDefault
	if searchTimeoutHdr != "" {
		timeout, err := time.ParseDuration(searchTimeoutHdr)
		if err != nil || timeout < 0 {
			timeout = jobOrchSearchTimeoutDefault

		}
	}
	respTimeout := jobOrchSearchRespTimeoutDefault
	if respTimeoutHdr != "" {
		respTimeout, err := time.ParseDuration(respTimeoutHdr)
		if err != nil || respTimeout < 0 {
			respTimeout = jobOrchSearchRespTimeoutDefault
		}
	}

	return timeout, respTimeout
}

func getJobOrchestrators(ctx context.Context, node *core.LivepeerNode, capability string, params JobParameters, timeout time.Duration, respTimeout time.Duration) ([]core.JobToken, error) {
	orchs := node.OrchestratorPool.GetInfos()
	//setup the GET request to get the Orchestrator tokens
	reqSender, err := getJobSender(ctx, node)
	if err != nil {
		clog.Errorf(ctx, "Failed to get job sender err=%v", err)
		return nil, err
	}

	getOrchJobToken := func(ctx context.Context, orchUrl *url.URL, reqSender core.JobSender, respTimeout time.Duration, tokenCh chan core.JobToken, errCh chan error) {
		start := time.Now()
		tokenReq, err := http.NewRequestWithContext(ctx, "GET", orchUrl.String()+"/process/token", nil)
		reqSenderStr, _ := json.Marshal(reqSender)
		tokenReq.Header.Set(jobEthAddressHdr, base64.StdEncoding.EncodeToString(reqSenderStr))
		tokenReq.Header.Set(jobCapabilityHdr, capability)
		if err != nil {
			clog.Errorf(ctx, "Failed to create request for Orchestrator to verify job token request err=%v", err)
			return
		}

		resp, err := sendJobReqWithTimeout(tokenReq, respTimeout)
		if err != nil {
			clog.Errorf(ctx, "failed to get token from Orchestrator err=%v", err)
			errCh <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			clog.Errorf(ctx, "Failed to get token from Orchestrator %v err=%v", orchUrl.String(), err)
			errCh <- fmt.Errorf("failed to get token from Orchestrator")
			return
		}

		latency := time.Since(start)
		clog.V(common.DEBUG).Infof(ctx, "Received job token from uri=%v, latency=%v", orchUrl, latency)

		token, err := io.ReadAll(resp.Body)
		if err != nil {
			clog.Errorf(ctx, "Failed to read token from Orchestrator %v err=%v", orchUrl.String(), err)
			errCh <- err
			return
		}
		var jobToken core.JobToken
		err = json.Unmarshal(token, &jobToken)
		if err != nil {
			clog.Errorf(ctx, "Failed to unmarshal token from Orchestrator %v err=%v", orchUrl.String(), err)
			errCh <- err
			return
		}

		tokenCh <- jobToken
	}

	var jobTokens []core.JobToken
	timedOut := false
	nbResp := 0
	numAvailableOrchs := node.OrchestratorPool.Size()
	tokenCh := make(chan core.JobToken, numAvailableOrchs)
	errCh := make(chan error, numAvailableOrchs)

	tokensCtx, cancel := context.WithTimeout(clog.Clone(context.Background(), ctx), timeout)
	// Shuffle and get job tokens
	for _, i := range rand.Perm(len(orchs)) {
		//do not send to excluded Orchestrators
		if slices.Contains(params.Orchestrators.Exclude, orchs[i].URL.String()) {
			numAvailableOrchs--
			continue
		}
		//if include is set, only send to those Orchestrators
		if len(params.Orchestrators.Include) > 0 && !slices.Contains(params.Orchestrators.Include, orchs[i].URL.String()) {
			numAvailableOrchs--
			continue
		}

		go getOrchJobToken(ctx, orchs[i].URL, *reqSender, 500*time.Millisecond, tokenCh, errCh)
	}

	for nbResp < numAvailableOrchs && len(jobTokens) < numAvailableOrchs && !timedOut {
		select {
		case token := <-tokenCh:
			jobTokens = append(jobTokens, token)
			nbResp++
		case <-errCh:
			nbResp++
		case <-tokensCtx.Done():
			break
		}
	}
	cancel()

	return jobTokens, nil
}

func getJobSender(ctx context.Context, node *core.LivepeerNode) (*core.JobSender, error) {
	gateway := node.OrchestratorPool.Broadcaster()
	orchReq, err := genOrchestratorReq(gateway, GetOrchestratorInfoParams{})
	if err != nil {
		clog.Errorf(ctx, "Failed to generate request for Orchestrator to verify to request job token err=%v", err)
		return nil, err
	}
	addr := ethcommon.BytesToAddress(orchReq.Address)
	jobSender := &core.JobSender{
		Addr: addr.Hex(),
		Sig:  "0x" + hex.EncodeToString(orchReq.Sig),
	}

	return jobSender, nil
}
func getToken(ctx context.Context, respTimeout time.Duration, orchUrl, capability, sender, senderSig string) (*core.JobToken, error) {
	start := time.Now()
	tokenReq, err := http.NewRequestWithContext(ctx, "GET", orchUrl+"/process/token", nil)
	jobSender := core.JobSender{Addr: sender, Sig: senderSig}

	reqSenderStr, _ := json.Marshal(jobSender)
	tokenReq.Header.Set(jobEthAddressHdr, base64.StdEncoding.EncodeToString(reqSenderStr))
	tokenReq.Header.Set(jobCapabilityHdr, capability)
	if err != nil {
		clog.Errorf(ctx, "Failed to create request for Orchestrator to verify job token request err=%v", err)
		return nil, err
	}

	var resp *http.Response
	var token []byte
	var jobToken core.JobToken
	var attempt int
	var backoff time.Duration = 100 * time.Millisecond
	deadline := time.Now().Add(respTimeout)

	for attempt = 0; attempt < 3; attempt++ {
		resp, err = sendJobReqWithTimeout(tokenReq, respTimeout)
		if err != nil {
			clog.Errorf(ctx, "failed to get token from Orchestrator (attempt %d) err=%v", attempt+1, err)
		} else if resp.StatusCode != http.StatusOK {
			clog.Errorf(ctx, "Failed to get token from Orchestrator %v status=%v (attempt %d)", orchUrl, resp.StatusCode, attempt+1)
		} else {
			defer resp.Body.Close()
			latency := time.Since(start)
			clog.V(common.DEBUG).Infof(ctx, "Received job token from uri=%v, latency=%v", orchUrl, latency)
			token, err = io.ReadAll(resp.Body)
			if err != nil {
				clog.Errorf(ctx, "Failed to read token from Orchestrator %v err=%v", orchUrl, err)
			} else {
				err = json.Unmarshal(token, &jobToken)
				if err != nil {
					clog.Errorf(ctx, "Failed to unmarshal token from Orchestrator %v err=%v", orchUrl, err)
				} else {
					return &jobToken, nil
				}
			}
		}
		// If not last attempt and time remains, backoff
		if time.Now().Add(backoff).Before(deadline) && attempt < 2 {
			time.Sleep(backoff)
			backoff *= 2
		} else {
			break
		}
	}
	// All attempts failed
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("failed to get token from Orchestrator after %d attempts", attempt)
}
