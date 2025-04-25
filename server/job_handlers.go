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
	"net/http"
	"strings"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"

	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/golang/glog"
)

const jobRequestHdr = "Livepeer-Job"
const jobEthAddressHdr = "Livepeer-Job-Eth-Address"
const jobCapabilityHdr = "Livepeer-Job-Capability"
const jobPaymentHeaderHdr = "Livepeer-Job-Payment"

var errNoTimeoutSet = errors.New("no timeout_seconds set with request, timeout_seconds is required")

type JobSender struct {
	Addr string `json:"addr"`
	Sig  string `json:"sig"`
}

type JobToken struct {
	SenderAddress *JobSender        `json:"sender_address,omitempty"`
	TicketParams  *net.TicketParams `json:"ticket_params,omitempty"`
	Balance       int64             `json:"balance,omitempty"`
	Price         *net.PriceInfo    `json:"price,omitempty"`
}

type JobRequest struct {
	ID            string `json:"id"`
	Request       string `json:"request"`
	Parameters    string `json:"parameters"`
	Capability    string `json:"capability"`
	CapabilityUrl string `json:"capability_url"` //this is set when verified orch as capability
	Sender        string `json:"sender"`
	Sig           string `json:"sig"`
	Timeout       int    `json:"timeout_seconds"`
}

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
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}

	clog.Infof(context.TODO(), "registered capability remoteAddr=%v capability=%v url=%v price=%v", remoteAddr, cap.Name, cap.Url, cap.GetPrice().FloatString(3))
}

func (h *lphttp) GetJobToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	remoteAddr := getRemoteAddr(r)

	orch := h.orchestrator
	jobEthAddrHdr := r.Header.Get(jobEthAddressHdr)
	if jobEthAddrHdr == "" {
		glog.Infof("generate token failed, invalid request remoteAddr=%v", remoteAddr)
		http.Error(w, fmt.Sprintf("Must have eth address and signature on address in Livepeer-Job-Eth-Address header"), http.StatusBadRequest)
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
		http.Error(w, fmt.Sprintf("Job capabilities not provided, must provide comma separated capabilities in Livepeer-Job-Capability header"), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	jobToken := JobToken{SenderAddress: nil, TicketParams: nil, Balance: 0, Price: nil}

	if !orch.CheckExternalCapabilityCapacity(jobCapsHdr) {
		//send response indicating no capacity available
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		senderAddr := ethcommon.HexToAddress(jobSenderAddr.Addr)

		jobPrice, err := orch.JobPriceInfo(senderAddr, core.RandomManifestID(), jobCapsHdr)
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
			// Remove the last three digits from capBalInt
			capBalInt = capBalInt / 1000
		} else {
			capBalInt = 0
		}

		jobToken = JobToken{
			SenderAddress: jobSenderAddr,
			TicketParams:  ticketParams,
			Balance:       capBalInt,
			Price:         jobPrice,
		}

		//send response indicating compatible
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(jobToken)
}

func (h *lphttp) ProcessJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	orch := h.orchestrator

	remoteAddr := getRemoteAddr(r)
	ctx := clog.AddVal(r.Context(), "client_ip", remoteAddr)

	// get payment information
	payment, err := getPayment(r.Header.Get(jobPaymentHeaderHdr))
	if err != nil {
		clog.Errorf(ctx, "Could not parse payment: %v", err)
		http.Error(w, err.Error(), http.StatusPaymentRequired)
		return
	}

	// check the prompt sig from the request
	// returns error if no capacity
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
	taskId := core.RandomManifestID()
	ctx = clog.AddVal(ctx, "job_id", jobReq.ID)
	ctx = clog.AddVal(ctx, "worker_task_id", string(taskId))
	ctx = clog.AddVal(ctx, "capability", jobReq.Capability)
	ctx = clog.AddVal(ctx, "sender", jobReq.Sender)
	sender := ethcommon.HexToAddress(jobReq.Sender)

	//jobId := jobReq.Capability + "-" + jobReq.ID
	jobId := jobReq.Capability
	jobPrice := payment.GetExpectedPrice()
	if payment.TicketParams == nil {
		//no payment included, confirm if balance remains
		if !h.orchestrator.SufficientBalance(sender, core.ManifestID(jobId)) {
			clog.Errorf(ctx, "Insufficient balance for request")
			http.Error(w, "Insufficient balance", http.StatusPaymentRequired)
			return
		}
	}

	if err := orch.ProcessPayment(ctx, payment, core.ManifestID(jobId)); err != nil {
		clog.Errorf(ctx, "error processing payment err=%q", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	clog.Infof(ctx, "balance after payment is %v", orch.Balance(sender, core.ManifestID(jobId)).FloatString(3))
	clog.V(common.SHORT).Infof(ctx, "Received job, sending for processing")

	// Read the original body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	workerRoute := jobReq.CapabilityUrl
	workerResourceRoute := r.PathValue("workerResourceRoute")
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

		if err == context.DeadlineExceeded {
			orch.RemoveExternalCapability(jobReq.Capability)
		}

		chargeForCompute(start, jobPrice, orch, sender, jobId)
		addPaymentBalanceHeader(w, orch, sender, jobId)
		http.Error(w, fmt.Sprintf("job not able to be processed err=%v", err.Error()), http.StatusInternalServerError)
		return
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		//non streaming response
		//release capacity for another request
		orch.FreeExternalCapabilityCapacity(jobReq.Capability)

		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			clog.Errorf(ctx, "Unable to read response err=%v", err)

			chargeForCompute(start, jobPrice, orch, sender, jobId)
			addPaymentBalanceHeader(w, orch, sender, jobId)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		//error response from worker but assume can retry and pass along error response and status code
		if resp.StatusCode > 399 {
			clog.Errorf(ctx, "error processing request err=%v ", string(data))

			chargeForCompute(start, jobPrice, orch, sender, jobId)
			addPaymentBalanceHeader(w, orch, sender, jobId)
			//return error response from the worker
			http.Error(w, string(data), resp.StatusCode)
			return
		}

		chargeForCompute(start, jobPrice, orch, sender, jobId)
		addPaymentBalanceHeader(w, orch, sender, jobId)
		clog.V(common.SHORT).Infof(ctx, "Job processed successfully took=%v", time.Since(start))
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
			addPaymentBalanceHeader(w, orch, sender, jobId)
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		// Read from upstream and forward to client
		respChan := make(chan string, 100)
		respCtx, cancelResp := context.WithTimeout(ctx, 40*12*time.Second) //paramsExpirationBlock (40) in pm package * 12 second block times
		defer cancelResp()
		go func() {
			defer resp.Body.Close()
			defer close(respChan)
			scanner := bufio.NewScanner(resp.Body)
			//set a reasonable timeout to stop waiting for results

			for scanner.Scan() {
				select {
				case <-respCtx.Done():
					return
				default:
					line := scanner.Text()
					if strings.Contains(line, "[DONE]") {
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
				h.orchestrator.DebitFees(sender, core.ManifestID(jobId), payment.GetExpectedPrice(), 5)
				senderBalance := getPaymentBalance(orch, sender, jobId)
				if senderBalance != nil {
					if senderBalance.Cmp(big.NewRat(0, 1)) < 0 {
						w.Write([]byte("event: insufficient balance\n"))
						w.Write([]byte("data: [DONE]\n\n"))
						flusher.Flush()
						break proxyResp
					}
				}
			case line := <-respChan:
				w.Write([]byte(line + "\n"))
				flusher.Flush()
			case <-respCtx.Done():
				break proxyResp
			}
		}

		//release capacity for another request
		orch.FreeExternalCapabilityCapacity(jobReq.Capability)
		clog.V(common.SHORT).Infof(ctx, "Job processed successfully took=%v", time.Since(start))
	}
}

func chargeForCompute(start time.Time, price *net.PriceInfo, orch Orchestrator, sender ethcommon.Address, jobId string) {
	// Debit the fee for the total time processed
	took := time.Since(start)
	orch.DebitFees(sender, core.ManifestID(jobId), price, int64(math.Ceil(took.Seconds())))
	clog.V(common.SHORT).Infof(context.TODO(), "Job processed successfully took=%v", took)
}

func addPaymentBalanceHeader(w http.ResponseWriter, orch Orchestrator, sender ethcommon.Address, jobId string) {
	//check balance and return remaning balance in header of response
	senderBalance := getPaymentBalance(orch, sender, jobId)
	if senderBalance != nil {
		senderBalAmt, err := common.PriceToInt64(senderBalance)
		if err != nil {
			glog.Errorf("could not convert balance to int64 sender=%v jobId=%v err=%v", sender.Hex(), jobId, err.Error())
			senderBalAmt = big.NewRat(0, 1)
		}
		w.Header().Set("Livepeer-Payment-Balance", senderBalAmt.FloatString(0))
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
	senderBalAmt, err := common.PriceToInt64(senderBalance)
	if err != nil {
		return big.NewRat(0, 1)
	}

	return senderBalAmt

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

func verifyJobCreds(ctx context.Context, orch Orchestrator, jobCreds string) (*JobRequest, error) {
	buf, err := base64.StdEncoding.DecodeString(jobCreds)
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
		clog.Errorf(ctx, "Sig check failed")
		return nil, errSegSig
	}

	if orch.ReserveExternalCapabilityCapacity(jobData.Capability) != nil {
		return nil, errZeroCapacity
	}

	jobData.CapabilityUrl = orch.GetUrlForCapability(jobData.Capability)

	return &jobData, nil
}
