package byoc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"math/rand/v2"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/pkg/errors"
)

// Gateway handler for job request
func (bsg *BYOCGatewayServer) SubmitJob() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Gateway node
		bsg.submitJob(ctx, w, r)
	})
}

func (bsg *BYOCGatewayServer) submitJob(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	gatewayJob, err := bsg.setupGatewayJob(ctx, r.Header.Get(jobRequestHdr), r.Header.Get(jobOrchSearchTimeoutHdr), r.Header.Get(jobOrchSearchRespTimeoutHdr), false)
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
		workerResourceRoute := r.URL.Path

		err := gatewayJob.sign()
		if err != nil {
			clog.Errorf(ctx, "Error signing job, exiting stream processing request: %v", err)
			return
		}

		start := time.Now()
		resp, code, err := bsg.sendJobToOrch(ctx, r, gatewayJob.Job.Req, gatewayJob.SignedJobReq, orchToken, workerResourceRoute, body)
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

			gatewayBalance := updateGatewayBalance(bsg.node, orchToken, gatewayJob.Job.Req.Capability, time.Since(start))
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

			gatewayBalance := updateGatewayBalance(bsg.node, orchToken, gatewayJob.Job.Req.Capability, time.Since(start))

			clog.V(common.SHORT).Infof(ctx, "Job processed successfully took=%v balance=%v balance_from_orch=%v", time.Since(start), gatewayBalance.FloatString(0), orchBalance.FloatString(0))
		}
	}
}

func (bsg *BYOCGatewayServer) sendJobToOrch(ctx context.Context, r *http.Request, jobReq *JobRequest, signedReqHdr string, orchToken JobToken, route string, body []byte) (*http.Response, int, error) {
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
		paymentHdr, err := bsg.createPayment(ctx, jobReq, &orchToken)
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

func (bs *BYOCGatewayServer) sendPayment(ctx context.Context, orchPmtUrl, capability, jobReq, payment string) (int, error) {
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
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	return resp.StatusCode, nil
}

func (bsg *BYOCGatewayServer) setupGatewayJob(ctx context.Context, jobReqHdr string, orchSearchTimeoutHdr string, orchSearchRespTimeoutHdr string, skipOrchSearch bool) (*gatewayJob, error) {

	var orchs []JobToken

	clog.Infof(ctx, "processing job request req=%v", jobReqHdr)
	jobReq, err := bsg.verifyJobCreds(ctx, jobReqHdr, true)
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
		searchTimeout, respTimeout := getOrchSearchTimeouts(ctx, orchSearchTimeoutHdr, orchSearchRespTimeoutHdr)
		jobReq.OrchSearchTimeout = searchTimeout
		jobReq.OrchSearchRespTimeout = respTimeout

		//get pool of Orchestrators that can do the job
		orchs, err = getJobOrchestrators(ctx, bsg.node, jobReq.Capability, jobParams, jobReq.OrchSearchTimeout, jobReq.OrchSearchRespTimeout)
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

	return &gatewayJob{Job: &job, Orchs: orchs, node: bsg.node}, nil
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

func (bsg *BYOCGatewayServer) verifyJobCreds(ctx context.Context, jobCreds string, reserveCapacity bool) (*JobRequest, error) {
	//Gateway needs JobRequest parsed and verification of required fields
	jobData, err := parseJobRequest(jobCreds)
	if err != nil {
		glog.Error("Unable to unmarshal ", err)
		return nil, err
	}

	if jobData.Timeout == 0 {
		return nil, errNoTimeoutSet
	}

	return jobData, nil
}

func getJobOrchestrators(ctx context.Context, node *core.LivepeerNode, capability string, params JobParameters, timeout time.Duration, respTimeout time.Duration) ([]JobToken, error) {
	orchs := node.OrchestratorPool.GetInfos()
	//setup the GET request to get the Orchestrator tokens
	reqSender, err := getJobSender(ctx, node)
	if err != nil {
		clog.Errorf(ctx, "Failed to get job sender err=%v", err)
		return nil, err
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

func getJobSender(ctx context.Context, node *core.LivepeerNode) (*JobSender, error) {
	gateway := node.OrchestratorPool.Broadcaster()
	orchReq, err := genOrchestratorReq(gateway)
	if err != nil {
		clog.Errorf(ctx, "Failed to generate request for Orchestrator to verify to request job token err=%v", err)
		return nil, err
	}
	addr := ethcommon.BytesToAddress(orchReq.Address)
	jobSender := &JobSender{
		Addr: addr.Hex(),
		Sig:  "0x" + hex.EncodeToString(orchReq.Sig),
	}

	return jobSender, nil
}

func genOrchestratorReq(b common.Broadcaster) (*net.OrchestratorRequest, error) {
	sig, err := b.Sign([]byte(fmt.Sprintf("%v", b.Address().Hex())))
	if err != nil {
		return nil, err
	}
	return &net.OrchestratorRequest{Address: b.Address().Bytes(), Sig: sig}, nil
}

func getToken(ctx context.Context, respTimeout time.Duration, orchUrl, capability, sender, senderSig string) (*JobToken, error) {
	start := time.Now()
	tokenReq, err := http.NewRequestWithContext(ctx, "GET", orchUrl+"/process/token", nil)
	jobSender := JobSender{Addr: sender, Sig: senderSig}

	reqSenderStr, _ := json.Marshal(jobSender)
	tokenReq.Header.Set(jobEthAddressHdr, base64.StdEncoding.EncodeToString(reqSenderStr))
	tokenReq.Header.Set(jobCapabilityHdr, capability)
	if err != nil {
		clog.Errorf(ctx, "Failed to create request for Orchestrator to verify job token request err=%v", err)
		return nil, err
	}

	var resp *http.Response
	var jobToken JobToken
	var attempt int
	var backoff time.Duration = 100 * time.Millisecond
	deadline := time.Now().Add(respTimeout)

	for attempt = 0; attempt < 3; attempt++ {
		resp, err = sendJobReqWithTimeout(tokenReq, respTimeout)
		if err != nil {
			clog.Errorf(ctx, "failed to get token from Orchestrator (attempt %d) err=%v", attempt+1, err)
			continue
		}
		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			clog.Errorf(ctx, "Failed to read token response from Orchestrator %v err=%v", orchUrl, err)
		}

		if resp.StatusCode != http.StatusOK {
			clog.Errorf(ctx, "Failed to get token from Orchestrator %v status=%v (attempt %d)", orchUrl, resp.StatusCode, attempt+1)
		} else {
			latency := time.Since(start)
			clog.V(common.DEBUG).Infof(ctx, "Received job token from uri=%v, latency=%v", orchUrl, latency)
			err = json.Unmarshal(respBody, &jobToken)
			if err != nil {
				clog.Errorf(ctx, "Failed to unmarshal token from Orchestrator %v err=%v", orchUrl, err)
			} else {
				return &jobToken, nil
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
