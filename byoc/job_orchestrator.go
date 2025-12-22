package byoc

//based on segment_rpc.go

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"

	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

// worker registers to Orchestrator
func (bs *BYOCOrchestratorServer) RegisterCapability() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		orch := bs.orch
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
	})
}

func (bs *BYOCOrchestratorServer) UnregisterCapability() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		orch := bs.orch
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
	})
}

func (bso *BYOCOrchestratorServer) GetJobToken() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if bso.node.NodeType != core.OrchestratorNode {
			http.Error(w, "request not allowed", http.StatusBadRequest)
			return
		}

		remoteAddr := getRemoteAddr(r)

		orch := bso.orch

		jobEthAddrHdr := r.Header.Get(jobEthAddressHdr)
		if jobEthAddrHdr == "" {
			glog.Infof("generate token failed, invalid request remoteAddr=%v", remoteAddr)
			http.Error(w, fmt.Sprintf("Must have eth address and signature on address in Livepeer-Eth-Address header"), http.StatusBadRequest)
			return
		}
		jobSenderAddr, err := bso.verifyTokenCreds(r.Context(), jobEthAddrHdr)
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
	})
}

func (bso *BYOCOrchestratorServer) ProcessJob() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Orchestrator node
		bso.processJob(ctx, w, r)
	})
}

func (bso *BYOCOrchestratorServer) processJob(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	remoteAddr := getRemoteAddr(r)
	ctx = clog.AddVal(ctx, "client_ip", remoteAddr)
	orch := bso.orch
	// check the prompt sig from the request
	// confirms capacity available before processing payment info
	orchJob, err := bso.setupOrchJob(ctx, r, true)
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
		return
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
			bso.orch.RemoveExternalCapability(orchJob.Req.Capability)
		}

		bso.chargeForCompute(start, orchJob.JobPrice, orchJob.Sender, orchJob.Req.Capability)
		w.Header().Set(jobPaymentBalanceHdr, bso.getPaymentBalance(orchJob.Sender, orchJob.Req.Capability).FloatString(0))
		http.Error(w, fmt.Sprintf("job not able to be processed, removing capability err=%v", err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("X-Metadata", resp.Header.Get("X-Metadata"))

	//release capacity for another request
	// if requester closes the connection need to release capacity
	defer bso.orch.FreeExternalCapabilityCapacity(orchJob.Req.Capability)

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		//non streaming response

		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			clog.Errorf(ctx, "Unable to read response err=%v", err)

			bso.chargeForCompute(start, orchJob.JobPrice, orchJob.Sender, orchJob.Req.Capability)
			w.Header().Set(jobPaymentBalanceHdr, bso.getPaymentBalance(orchJob.Sender, orchJob.Req.Capability).FloatString(0))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		//error response from worker but assume can retry and pass along error response and status code
		if resp.StatusCode > 399 {
			clog.Errorf(ctx, "error processing request err=%v ", string(data))

			bso.chargeForCompute(start, orchJob.JobPrice, orchJob.Sender, orchJob.Req.Capability)
			w.Header().Set(jobPaymentBalanceHdr, bso.getPaymentBalance(orchJob.Sender, orchJob.Req.Capability).FloatString(0))
			//return error response from the worker
			http.Error(w, string(data), resp.StatusCode)
			return
		}

		bso.chargeForCompute(start, orchJob.JobPrice, orchJob.Sender, orchJob.Req.Capability)
		w.Header().Set(jobPaymentBalanceHdr, bso.getPaymentBalance(orchJob.Sender, orchJob.Req.Capability).FloatString(0))
		clog.V(common.SHORT).Infof(ctx, "Job processed successfully took=%v balance=%v", time.Since(start), bso.getPaymentBalance(orchJob.Sender, orchJob.Req.Capability).FloatString(0))
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
		bso.addPaymentBalanceHeader(w, orchJob.Sender, orchJob.Req.Capability)

		// Flush to ensure data is sent immediately
		flusher, ok := w.(http.Flusher)
		if !ok {
			clog.Errorf(ctx, "streaming not supported")

			bso.chargeForCompute(start, orchJob.JobPrice, orchJob.Sender, orchJob.Req.Capability)
			w.Header().Set(jobPaymentBalanceHdr, bso.getPaymentBalance(orchJob.Sender, orchJob.Req.Capability).FloatString(0))
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
					bso.orch.DebitFees(orchJob.Sender, core.ManifestID(orchJob.Req.Capability), orchJob.JobPrice, 5)
					senderBalance := bso.getPaymentBalance(orchJob.Sender, orchJob.Req.Capability)
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
		clog.V(common.SHORT).Infof(ctx, "Job processed successfully took=%v balance=%v", time.Since(start), bso.getPaymentBalance(orchJob.Sender, orchJob.Req.Capability).FloatString(0))
	}
}

// SetupOrchJob prepares the orchestrator job by extracting and validating the job request from the HTTP headers.
// Payment is applied if applicable.
func (bso *BYOCOrchestratorServer) setupOrchJob(ctx context.Context, r *http.Request, reserveCapacity bool) (*orchJob, error) {
	job := r.Header.Get(jobRequestHdr)
	orch := bso.orch
	jobReq, err := bso.verifyJobCreds(ctx, job, reserveCapacity)
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

	pmtErr := bso.confirmPayment(ctx, sender, jobReq.Capability, jobPrice, r.Header.Get(jobPaymentHeaderHdr))
	if pmtErr != nil {
		orch.FreeExternalCapabilityCapacity(jobReq.Capability)
		return nil, pmtErr
	}

	var jobDetails JobRequestDetails
	err = json.Unmarshal([]byte(jobReq.Request), &jobDetails)
	if err != nil {
		return nil, fmt.Errorf("Unable to unmarshal job request details err=%v", err)
	}

	clog.Infof(ctx, "job request verified id=%v sender=%v capability=%v timeout=%v", jobReq.ID, jobReq.Sender, jobReq.Capability, jobReq.Timeout)

	return &orchJob{Req: jobReq, Sender: sender, JobPrice: jobPrice, Details: &jobDetails}, nil
}

func (bso *BYOCOrchestratorServer) confirmPayment(ctx context.Context, sender ethcommon.Address, capability string, jobPrice *net.PriceInfo, paymentHdr string) error {

	clog.V(common.DEBUG).Infof(ctx, "job price=%v units=%v", jobPrice.PricePerUnit, jobPrice.PixelsPerUnit)

	//no payment included, confirm if balance remains
	jobPriceRat := big.NewRat(jobPrice.PricePerUnit, jobPrice.PixelsPerUnit)
	// if price is 0, no payment required
	if jobPriceRat.Cmp(big.NewRat(0, 1)) > 0 {
		minBal := new(big.Rat).Mul(jobPriceRat, big.NewRat(60, 1)) //minimum 1 minute balance
		//process payment if included
		orchBal, pmtErr := bso.processPayment(ctx, sender, capability, paymentHdr)
		if pmtErr != nil {
			//log if there are payment errors but continue, balance will runout and clean up
			clog.Infof(ctx, "job payment error: %v", pmtErr)
		}

		if orchBal.Cmp(minBal) < 0 {
			return errInsufficientBalance
		}
	}

	return nil
}

// process payment and return balance
func (bso *BYOCOrchestratorServer) processPayment(ctx context.Context, sender ethcommon.Address, capability string, paymentHdr string) (*big.Rat, error) {
	if paymentHdr != "" {
		payment, err := getPayment(paymentHdr)
		if err != nil {
			clog.Errorf(ctx, "job payment invalid: %v", err)
			return nil, errPaymentError
		}

		if err := bso.orch.ProcessPayment(ctx, payment, core.ManifestID(capability)); err != nil {
			bso.orch.FreeExternalCapabilityCapacity(capability)
			clog.Errorf(ctx, "Error processing payment: %v", err)
			return nil, errPaymentError
		}
	}
	orchBal := bso.getPaymentBalance(sender, capability)

	return orchBal, nil

}

func (bso *BYOCOrchestratorServer) chargeForCompute(start time.Time, price *net.PriceInfo, sender ethcommon.Address, jobId string) {
	// Debit the fee for the total time processed
	took := time.Since(start)
	bso.orch.DebitFees(sender, core.ManifestID(jobId), price, int64(math.Ceil(took.Seconds())))
}

func (bso *BYOCOrchestratorServer) addPaymentBalanceHeader(w http.ResponseWriter, sender ethcommon.Address, jobId string) {
	//check balance and return remaning balance in header of response
	senderBalance := bso.getPaymentBalance(sender, jobId)
	w.Header().Set("Livepeer-Payment-Balance", senderBalance.FloatString(0))
}

func (bso *BYOCOrchestratorServer) getPaymentBalance(sender ethcommon.Address, jobId string) *big.Rat {
	//check balance and return remaning balance in header of response
	senderBalance := bso.orch.Balance(sender, core.ManifestID(jobId))
	if senderBalance == nil {
		senderBalance = big.NewRat(0, 1)
	}

	return senderBalance
}

func (bso *BYOCOrchestratorServer) verifyJobCreds(ctx context.Context, jobCreds string, reserveCapacity bool) (*JobRequest, error) {
	jobData, err := parseJobRequest(jobCreds)
	if err != nil {
		glog.Error("Unable to unmarshal ", err)
		return nil, err
	}

	if jobData.Timeout == 0 {
		return nil, errNoTimeoutSet
	}

	sigHex := jobData.Sig
	if len(jobData.Sig) > 130 {
		sigHex = jobData.Sig[2:]
	}
	sigByte, err := hex.DecodeString(sigHex)
	if err != nil {
		clog.Errorf(ctx, "Unable to hex-decode signature", err)
		return nil, errSegSig
	}

	if !bso.orch.VerifySig(ethcommon.HexToAddress(jobData.Sender), jobData.Request+jobData.Parameters, sigByte) {
		clog.Errorf(ctx, "Sig check failed sender=%v", jobData.Sender)
		return nil, errSegSig
	}

	if reserveCapacity && bso.orch.ReserveExternalCapabilityCapacity(jobData.Capability) != nil {
		return nil, errZeroCapacity
	}

	jobData.CapabilityUrl = bso.orch.GetUrlForCapability(jobData.Capability)

	return jobData, nil
}

func (bso *BYOCOrchestratorServer) verifyTokenCreds(ctx context.Context, tokenCreds string) (*JobSender, error) {
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

	if !bso.orch.VerifySig(ethcommon.HexToAddress(jobSender.Addr), jobSender.Addr, sigByte) {
		clog.Errorf(ctx, "Sig check failed")
		return nil, errSegSig
	}

	//signature confirmed
	return &jobSender, nil
}
