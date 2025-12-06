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
var sendJobReqWithTimeout = sendReqWithTimeout

type JobSender struct {
	Addr string `json:"addr"`
	Sig  string `json:"sig"`
}

type JobToken struct {
	SenderAddress *JobSender        `json:"sender_address,omitempty"`
	TicketParams  *net.TicketParams `json:"ticket_params,omitempty"`
	Balance       int64             `json:"balance,omitempty"`
	Price         *net.PriceInfo    `json:"price,omitempty"`
	ServiceAddr   string            `json:"service_addr,omitempty"`
}

type JobRequest struct {
	ID            string `json:"id"`
	Request       string `json:"request"`
	Parameters    string `json:"parameters"` //additional information for the Gateway to use to select orchestrators or to send to the worker
	Capability    string `json:"capability"`
	CapabilityUrl string `json:"capability_url"` //this is set when verified orch as capability
	Sender        string `json:"sender"`
	Sig           string `json:"sig"`
	Timeout       int    `json:"timeout_seconds"`

	orchSearchTimeout     time.Duration
	orchSearchRespTimeout time.Duration
}

type JobParameters struct {
	Orchestrators JobOrchestratorsFilter `json:"orchestrators,omitempty"` //list of orchestrators to use for the job
}

type JobOrchestratorsFilter struct {
	Exclude []string `json:"exclude,omitempty"`
	Include []string `json:"include,omitempty"`
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
	jobToken := JobToken{SenderAddress: nil, TicketParams: nil, Balance: 0, Price: nil}

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

		jobToken = JobToken{
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
	jobReqHdr := r.Header.Get(jobRequestHdr)
	jobReq, err := verifyJobCreds(ctx, nil, jobReqHdr)
	if err != nil {
		clog.Errorf(ctx, "Unable to verify job creds err=%v", err)
		http.Error(w, fmt.Sprintf("Unable to parse job request, err=%v", err), http.StatusBadRequest)
		return
	}
	ctx = clog.AddVal(ctx, "job_id", jobReq.ID)
	ctx = clog.AddVal(ctx, "capability", jobReq.Capability)
	clog.Infof(ctx, "processing job request")

	searchTimeout, respTimeout := getOrchSearchTimeouts(ctx, r.Header.Get(jobOrchSearchTimeoutHdr), r.Header.Get(jobOrchSearchRespTimeoutHdr))
	jobReq.orchSearchTimeout = searchTimeout
	jobReq.orchSearchRespTimeout = respTimeout

	var params JobParameters
	if err := json.Unmarshal([]byte(jobReq.Parameters), &params); err != nil {
		clog.Errorf(ctx, "Unable to unmarshal job parameters err=%v", err)
		http.Error(w, fmt.Sprintf("Unable to unmarshal job parameters err=%v", err), http.StatusBadRequest)
		return
	}

	//get pool of Orchestrators that can do the job
	orchs, err := getJobOrchestrators(ctx, ls.LivepeerNode, jobReq.Capability, params, jobReq.orchSearchTimeout, jobReq.orchSearchRespTimeout)
	if err != nil {
		clog.Errorf(ctx, "Unable to find orchestrators for capability %v err=%v", jobReq.Capability, err)
		http.Error(w, fmt.Sprintf("Unable to find orchestrators for capability %v err=%v", jobReq.Capability, err), http.StatusBadRequest)
		return
	}

	if len(orchs) == 0 {
		clog.Errorf(ctx, "No orchestrators found for capability %v", jobReq.Capability)
		http.Error(w, fmt.Sprintf("No orchestrators found for capability %v", jobReq.Capability), http.StatusServiceUnavailable)
		return
	}

	// Read the original request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	r.Body.Close()
	//sign the request
	gateway := ls.LivepeerNode.OrchestratorPool.Broadcaster()
	sig, err := gateway.Sign([]byte(jobReq.Request + jobReq.Parameters))
	if err != nil {
		clog.Errorf(ctx, "Unable to sign request err=%v", err)
		http.Error(w, fmt.Sprintf("Unable to sign request err=%v", err), http.StatusInternalServerError)
		return
	}
	jobReq.Sender = gateway.Address().Hex()
	jobReq.Sig = "0x" + hex.EncodeToString(sig)

	//create the job request header with the signature
	jobReqEncoded, err := json.Marshal(jobReq)
	if err != nil {
		clog.Errorf(ctx, "Unable to encode job request err=%v", err)
		http.Error(w, fmt.Sprintf("Unable to encode job request err=%v", err), http.StatusInternalServerError)
		return
	}
	jobReqHdr = base64.StdEncoding.EncodeToString(jobReqEncoded)

	//send the request to the Orchestrator(s)
	//the loop ends on Gateway error and bad request errors
	for _, orchToken := range orchs {

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

		req, err := http.NewRequestWithContext(ctx, "POST", workerRoute, bytes.NewBuffer(body))
		if err != nil {
			clog.Errorf(ctx, "Unable to create request err=%v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// set the headers
		req.Header.Add("Content-Length", r.Header.Get("Content-Length"))
		req.Header.Add("Content-Type", r.Header.Get("Content-Type"))

		req.Header.Add(jobRequestHdr, jobReqHdr)
		if orchToken.Price.PricePerUnit > 0 {
			paymentHdr, err := createPayment(ctx, jobReq, orchToken, ls.LivepeerNode)
			if err != nil {
				clog.Errorf(ctx, "Unable to create payment err=%v", err)
				http.Error(w, fmt.Sprintf("Unable to create payment err=%v", err), http.StatusBadRequest)
				return
			}
			req.Header.Add(jobPaymentHeaderHdr, paymentHdr)
		}

		start := time.Now()
		resp, err := sendJobReqWithTimeout(req, time.Duration(jobReq.Timeout+5)*time.Second) //include 5 second buffer
		if err != nil {
			clog.Errorf(ctx, "job not able to be processed by Orchestrator %v err=%v ", orchToken.ServiceAddr, err.Error())
			continue
		}
		//error response from Orchestrator
		if resp.StatusCode > 399 {
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				clog.Errorf(ctx, "Unable to read response err=%v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				continue
			}
			clog.Errorf(ctx, "error processing request err=%v ", string(data))
			//nonretryable error
			if resp.StatusCode < 500 {
				//assume non retryable bad request
				//return error response from the worker
				http.Error(w, string(data), resp.StatusCode)
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

			gatewayBalance := updateGatewayBalance(ls.LivepeerNode, orchToken, jobReq.Capability, time.Since(start))
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
			respCtx, _ := context.WithTimeout(ctx, time.Duration(jobReq.Timeout+10)*time.Second) //include a small buffer to let Orchestrator close the connection on the timeout

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

			gatewayBalance := updateGatewayBalance(ls.LivepeerNode, orchToken, jobReq.Capability, time.Since(start))

			clog.V(common.SHORT).Infof(ctx, "Job processed successfully took=%v balance=%v balance_from_orch=%v", time.Since(start), gatewayBalance.FloatString(0), orchBalance.FloatString(0))
		}

	}
}

func processJob(ctx context.Context, h *lphttp, w http.ResponseWriter, r *http.Request) {
	remoteAddr := getRemoteAddr(r)
	ctx = clog.AddVal(ctx, "client_ip", remoteAddr)
	orch := h.orchestrator
	// check the prompt sig from the request
	// confirms capacity available before processing payment info
	job := r.Header.Get(jobRequestHdr)
	jobReq, err := verifyJobCreds(ctx, orch, job)
	if err != nil {
		if err == errZeroCapacity {
			clog.Errorf(ctx, "No capacity available for capability err=%q", err)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		} else if err == errNoTimeoutSet {
			clog.Errorf(ctx, "Timeout not set in request err=%q", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			clog.Errorf(ctx, "Could not verify job creds err=%q", err)
			http.Error(w, err.Error(), http.StatusForbidden)
		}

		return
	}

	sender := ethcommon.HexToAddress(jobReq.Sender)
	jobPrice, err := orch.JobPriceInfo(sender, jobReq.Capability)
	if err != nil {
		clog.Errorf(ctx, "could not get price err=%v", err.Error())
		http.Error(w, fmt.Sprintf("Could not get price err=%v", err.Error()), http.StatusBadRequest)
		return
	}
	clog.V(common.DEBUG).Infof(ctx, "job price=%v units=%v", jobPrice.PricePerUnit, jobPrice.PixelsPerUnit)
	taskId := core.RandomManifestID()
	jobId := jobReq.Capability
	ctx = clog.AddVal(ctx, "job_id", jobReq.ID)
	ctx = clog.AddVal(ctx, "worker_task_id", string(taskId))
	ctx = clog.AddVal(ctx, "capability", jobReq.Capability)
	ctx = clog.AddVal(ctx, "sender", jobReq.Sender)

	//no payment included, confirm if balance remains
	jobPriceRat := big.NewRat(jobPrice.PricePerUnit, jobPrice.PixelsPerUnit)
	var payment net.Payment
	// if price is 0, no payment required
	if jobPriceRat.Cmp(big.NewRat(0, 1)) > 0 {
		// get payment information
		payment, err = getPayment(r.Header.Get(jobPaymentHeaderHdr))
		if err != nil {
			clog.Errorf(r.Context(), "Could not parse payment: %v", err)
			http.Error(w, err.Error(), http.StatusPaymentRequired)
			return
		}

		if payment.TicketParams == nil {

			//if price is not 0, confirm balance
			if jobPriceRat.Cmp(big.NewRat(0, 1)) > 0 {
				minBal := jobPriceRat.Mul(jobPriceRat, big.NewRat(60, 1)) //minimum 1 minute balance
				orchBal := getPaymentBalance(orch, sender, jobId)

				if orchBal.Cmp(minBal) < 0 {
					clog.Errorf(ctx, "Insufficient balance for request")
					http.Error(w, "Insufficient balance", http.StatusPaymentRequired)
					orch.FreeExternalCapabilityCapacity(jobReq.Capability)
					return
				}
			}
		} else {
			if err := orch.ProcessPayment(ctx, payment, core.ManifestID(jobId)); err != nil {
				clog.Errorf(ctx, "error processing payment err=%q", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				orch.FreeExternalCapabilityCapacity(jobReq.Capability)
				return
			}
		}

		clog.Infof(ctx, "balance after payment is %v", getPaymentBalance(orch, sender, jobId).FloatString(0))
	}

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

	workerRoute := jobReq.CapabilityUrl
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
	resp, err := sendReqWithTimeout(req, time.Duration(jobReq.Timeout)*time.Second)
	if err != nil {
		clog.Errorf(ctx, "job not able to be processed err=%v ", err.Error())
		//if the request failed with connection error, remove the capability
		//exclude deadline exceeded or context canceled errors does not indicate a fatal error all the time
		if err != context.DeadlineExceeded && !strings.Contains(err.Error(), "context canceled") {
			clog.Errorf(ctx, "removing capability %v due to error %v", jobReq.Capability, err.Error())
			h.orchestrator.RemoveExternalCapability(jobReq.Capability)
		}

		chargeForCompute(start, jobPrice, orch, sender, jobId)
		w.Header().Set(jobPaymentBalanceHdr, getPaymentBalance(orch, sender, jobId).FloatString(0))
		http.Error(w, fmt.Sprintf("job not able to be processed, removing capability err=%v", err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("X-Metadata", resp.Header.Get("X-Metadata"))

	//release capacity for another request
	// if requester closes the connection need to release capacity
	defer orch.FreeExternalCapabilityCapacity(jobReq.Capability)

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		//non streaming response

		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			clog.Errorf(ctx, "Unable to read response err=%v", err)

			chargeForCompute(start, jobPrice, orch, sender, jobId)
			w.Header().Set(jobPaymentBalanceHdr, getPaymentBalance(orch, sender, jobId).FloatString(0))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		//error response from worker but assume can retry and pass along error response and status code
		if resp.StatusCode > 399 {
			clog.Errorf(ctx, "error processing request err=%v ", string(data))

			chargeForCompute(start, jobPrice, orch, sender, jobId)
			w.Header().Set(jobPaymentBalanceHdr, getPaymentBalance(orch, sender, jobId).FloatString(0))
			//return error response from the worker
			http.Error(w, string(data), resp.StatusCode)
			return
		}

		chargeForCompute(start, jobPrice, orch, sender, jobId)
		w.Header().Set(jobPaymentBalanceHdr, getPaymentBalance(orch, sender, jobId).FloatString(0))
		clog.V(common.SHORT).Infof(ctx, "Job processed successfully took=%v balance=%v", time.Since(start), getPaymentBalance(orch, sender, jobId).FloatString(0))
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
		addPaymentBalanceHeader(w, orch, sender, jobId)

		// Flush to ensure data is sent immediately
		flusher, ok := w.(http.Flusher)
		if !ok {
			clog.Errorf(ctx, "streaming not supported")

			chargeForCompute(start, jobPrice, orch, sender, jobId)
			w.Header().Set(jobPaymentBalanceHdr, getPaymentBalance(orch, sender, jobId).FloatString(0))
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Read from upstream and forward to client
		respChan := make(chan string, 100)
		respCtx, _ := context.WithTimeout(ctx, time.Duration(jobReq.Timeout)*time.Second)

		go func() {
			defer resp.Body.Close()
			defer close(respChan)
			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				select {
				case <-respCtx.Done():
					orchBal := orch.Balance(sender, core.ManifestID(jobId))
					if orchBal == nil {
						orchBal = big.NewRat(0, 1)
					}
					respChan <- fmt.Sprintf("data: {\"balance\": %v}\n\n", orchBal.FloatString(3))
					respChan <- "data: [DONE]\n\n"
					return
				default:
					line := scanner.Text()
					if strings.Contains(line, "[DONE]") {
						orchBal := orch.Balance(sender, core.ManifestID(jobId))
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
				if jobPriceRat.Cmp(big.NewRat(0, 1)) > 0 {
					h.orchestrator.DebitFees(sender, core.ManifestID(jobId), jobPrice, 5)
					senderBalance := getPaymentBalance(orch, sender, jobId)
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
		clog.V(common.SHORT).Infof(ctx, "Job processed successfully took=%v balance=%v", time.Since(start), getPaymentBalance(orch, sender, jobId).FloatString(0))
	}
}

func createPayment(ctx context.Context, jobReq *JobRequest, orchToken JobToken, node *core.LivepeerNode) (string, error) {
	var payment *net.Payment
	sender := ethcommon.HexToAddress(jobReq.Sender)
	orchAddr := ethcommon.BytesToAddress(orchToken.TicketParams.Recipient)
	balance := node.Balances.Balance(orchAddr, core.ManifestID(jobReq.Capability))
	sessionID := node.Sender.StartSession(*pmTicketParams(orchToken.TicketParams))
	createTickets := true
	if balance == nil {
		//create a balance of 0
		node.Balances.Debit(orchAddr, core.ManifestID(jobReq.Capability), big.NewRat(0, 1))
		balance = node.Balances.Balance(orchAddr, core.ManifestID(jobReq.Capability))
	} else {
		price := big.NewRat(orchToken.Price.PricePerUnit, orchToken.Price.PixelsPerUnit)
		cost := price.Mul(price, big.NewRat(int64(jobReq.Timeout), 1))
		if balance.Cmp(cost) > 0 {
			createTickets = false
			payment = &net.Payment{
				Sender:        sender.Bytes(),
				ExpectedPrice: orchToken.Price,
			}
		}
	}

	if !createTickets {
		clog.V(common.DEBUG).Infof(ctx, "No payment required, using balance=%v", balance.FloatString(3))
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
				SenderNonce: tickets.SenderParams[i].SenderNonce,
				Sig:         tickets.SenderParams[i].Sig,
			}
			totalEV = totalEV.Add(totalEV, tickets.WinProbRat())
		}

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

func updateGatewayBalance(node *core.LivepeerNode, orchToken JobToken, capability string, took time.Duration) *big.Rat {
	orchAddr := ethcommon.BytesToAddress(orchToken.TicketParams.Recipient)
	// update for usage of compute
	orchPrice := big.NewRat(orchToken.Price.PricePerUnit, orchToken.Price.PixelsPerUnit)
	cost := orchPrice.Mul(orchPrice, big.NewRat(int64(math.Ceil(took.Seconds())), 1))
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
	//check balance and return remaining balance in header of response
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
	//check balance and return remaining balance in header of response
	senderBalance := orch.Balance(sender, core.ManifestID(jobId))
	if senderBalance == nil {
		senderBalance = big.NewRat(0, 1)
	}

	return senderBalance
}

func verifyTokenCreds(ctx context.Context, orch Orchestrator, tokenCreds string) (*JobSender, error) {
	buf, err := base64.StdEncoding.DecodeString(tokenCreds)
	if err != nil {
		glog.Error("Unable to base64-decode ", err)
		return nil, errSegEncoding
	}

	var jobSender JobSender
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

func verifyJobCreds(ctx context.Context, orch Orchestrator, jobCreds string) (*JobRequest, error) {
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

		if orch.ReserveExternalCapabilityCapacity(jobData.Capability) != nil {
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

func getJobOrchestrators(ctx context.Context, node *core.LivepeerNode, capability string, params JobParameters, timeout time.Duration, respTimeout time.Duration) ([]JobToken, error) {
	orchs := node.OrchestratorPool.GetInfos()
	gateway := node.OrchestratorPool.Broadcaster()

	//setup the GET request to get the Orchestrator tokens
	//get the address and sig for the sender
	gatewayReq, err := genOrchestratorReq(gateway, GetOrchestratorInfoParams{})
	if err != nil {
		clog.Errorf(ctx, "Failed to generate request for Orchestrator to verify to request job token err=%v", err)
		return nil, err
	}
	addr := ethcommon.BytesToAddress(gatewayReq.Address)
	reqSender := &JobSender{
		Addr: addr.Hex(),
		Sig:  "0x" + hex.EncodeToString(gatewayReq.Sig),
	}

	getOrchJobToken := func(ctx context.Context, orchUrl *url.URL, reqSender JobSender, respTimeout time.Duration, tokenCh chan JobToken, errCh chan error) {
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
		var jobToken JobToken
		err = json.Unmarshal(token, &jobToken)
		if err != nil {
			clog.Errorf(ctx, "Failed to unmarshal token from Orchestrator %v err=%v", orchUrl.String(), err)
			errCh <- err
			return
		}

		tokenCh <- jobToken
	}

	var jobTokens []JobToken
	timedOut := false
	nbResp := 0
	numAvailableOrchs := node.OrchestratorPool.Size()
	tokenCh := make(chan JobToken, numAvailableOrchs)
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
