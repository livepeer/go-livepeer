package server

//based on segment_rpc.go

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

type JobSender struct {
	Addr string `json:"addr"`
	Sig  string `json:"sig"`
}

type JobToken struct {
	Token         string            `json:"token"`
	JobId         string            `json:"job_id"`
	Expiration    int64             `json:"expiration"`
	SenderAddress *JobSender        `json:"sender_address,omitempty"`
	TicketParams  *net.TicketParams `json:"ticket_params,omitempty"`
	Balance       int64             `json:"balance,omitempty"`
}

type JobRequest struct {
	ID            string    `json:"id"`
	Request       string    `json:"request"`
	Parameters    string    `json:"parameters"`
	Capability    string    `json:"capability"`
	CapabilityUrl string    `json:"capability_url"` //this is set when verified orch as capability
	Token         *JobToken `json:"token"`
	Sig           string    `json:"sig"`
	Timeout       int       `json:"timeout_seconds"`
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

	clog.Infof(context.TODO(), "registered capability remoteAddr=%v capability=%v url=%v", remoteAddr, cap.Name, cap.Url)
}

func (h *lphttp) GetJobToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	remoteAddr := getRemoteAddr(r)
	var jobToken JobToken
	orch := h.orchestrator
	jobEthAddrHdr := r.Header.Get(jobEthAddressHdr)
	if jobEthAddrHdr == "" {
		glog.Infof("generate token failed, invalid request remoteAddr=%v", remoteAddr)
		http.Error(w, fmt.Sprintf("Must have eth address and signature on address in Livepeer-Job-Eth-Address header"), http.StatusBadRequest)
	}
	var jobSenderAddr *JobSender
	err := json.Unmarshal([]byte(jobEthAddressHdr), jobSenderAddr)
	if err != nil {
		glog.Infof("generate token failed, invalid request remoteAddr=%v", remoteAddr)
		http.Error(w, fmt.Sprintf("Invalid eth address header must include 'addr' and 'sig' fields"), http.StatusBadRequest)
	}

	if !ethcommon.IsHexAddress(jobSenderAddr.Addr) {
		glog.Infof("generate token failed, invalid eth address remoteAddr=%v", remoteAddr)
		http.Error(w, fmt.Sprintf("Eth address invalid, must have valid eth address in %v header", jobEthAddressHdr), http.StatusBadRequest)
	}

	if !orch.VerifySig(ethcommon.HexToAddress(jobSenderAddr.Addr), jobSenderAddr.Addr, []byte(jobSenderAddr.Sig)) {
		glog.Infof("generate token failed, eth address signature failed remoteAddr=%v", remoteAddr)
		http.Error(w, "eth address request signature could not be verified", http.StatusBadRequest)
	}

	jobCapsHdr := r.Header.Get(jobCapabilityHdr)
	if jobCapsHdr == "" {
		glog.Infof("generate token failed, invalid request, no capabilities included remoteAddr=%v", remoteAddr)
		http.Error(w, fmt.Sprintf("Job capabilities not provided, must provide comma separated capabilities in Livepeer-Job-Capability header"), http.StatusBadRequest)
	}

	w.Header().Set("Content-Type", "application/json")

	if orch.CheckExternalCapabilityCapacity(jobCapsHdr) {
		jobToken = JobToken{Token: "", JobId: "", Expiration: 0, SenderAddress: nil, TicketParams: nil, Balance: 0}
		//send response indicating no capacity available
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		senderAddr := ethcommon.HexToAddress(jobSenderAddr.Addr)
		jobId := newJobId()

		token := orch.AuthToken(string(jobId), time.Now().Add(authTokenValidPeriod).Unix())
		jobPrice, err := orch.JobPriceInfo(senderAddr, core.RandomManifestID(), jobCapsHdr)
		if err != nil {
			glog.Errorf("could not get price err=%v", err.Error())
			http.Error(w, fmt.Sprintf("Could not get price err=%v", err.Error()), http.StatusBadRequest)
		}
		ticketParams, err := orch.TicketParams(senderAddr, jobPrice)
		if err != nil {
			glog.Errorf("could not get ticket params err=%v", err.Error())
			http.Error(w, fmt.Sprintf("Could not get ticket params err=%v", err.Error()), http.StatusBadRequest)
		}

		capBal := orch.Balance(senderAddr, core.ManifestID(jobCapsHdr))
		if capBal != nil {
			capBal, err = common.PriceToInt64(capBal)
			if err != nil {
				clog.Errorf(context.TODO(), "could not convert balance to int64 sender=%v capability=%v err=%v", senderAddr.Hex(), jobCapsHdr, err.Error())
				capBal = big.NewRat(0, 0)
			}
		} else {
			capBal = big.NewRat(0, 0)
		}
		//convert to int64. Note: returns with 000 more digits to allow for precision of 3 decimal places.
		capBalInt, err := common.PriceToFixed(capBal)
		if err != nil {
			capBalInt = 0
		} else {
			// Remove the last three digits from capBalInt
			capBalInt = capBalInt / 1000
		}

		jobToken = JobToken{Token: base64.StdEncoding.EncodeToString(token.Token),
			JobId:         token.SessionId,
			Expiration:    token.Expiration,
			SenderAddress: jobSenderAddr,
			TicketParams:  ticketParams,
			Balance:       capBalInt,
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
	payment, err := getPayment(r.Header.Get(paymentHeader))
	if err != nil {
		clog.Errorf(ctx, "Could not parse payment")
		http.Error(w, err.Error(), http.StatusPaymentRequired)
		return
	}

	sender := getPaymentSender(payment)
	ctx = clog.AddVal(ctx, "sender", sender.Hex())

	// check the prompt sig from the request
	// returns error if no capacity
	job := r.Header.Get(jobRequestHdr)
	jobReq, ctx, err := verifyJobCreds(ctx, orch, job, sender)
	ctx = clog.AddVal(ctx, "job_id", jobReq.ID)
	ctx = clog.AddVal(ctx, "capability", jobReq.Capability)

	if err != nil {
		if err == errZeroCapacity {
			clog.Errorf(ctx, "No capacity available for capability err=%q", err)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		} else {
			clog.Errorf(ctx, "Could not verify job creds err=%q", err)
			http.Error(w, err.Error(), http.StatusForbidden)
		}

		return
	}

	if payment.TicketParams == nil {
		//no payment included, confirm if balance remains
		if !h.orchestrator.SufficientBalance(sender, core.ManifestID(jobReq.Capability)) {
			clog.Errorf(ctx, "Insufficient balance for request")
			http.Error(w, "Insufficient balance", http.StatusPaymentRequired)
			return
		}
	}

	if err := orch.ProcessPayment(ctx, payment, core.ManifestID(jobReq.ID)); err != nil {
		clog.Errorf(ctx, "error processing payment err=%q", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	clog.V(common.SHORT).Infof(ctx, "Received job, sending for processing", remoteAddr)
	newHeader := r.Header.Clone()
	for k, _ := range newHeader {
		if strings.Contains(k, "Livepeer") {
			newHeader.Del(k)
		}
	}
	req, err := http.NewRequestWithContext(ctx, "POST", jobReq.CapabilityUrl, r.Body)
	if err != nil {
		clog.Errorf(ctx, "Unable to create request err=%v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	req.Header = newHeader

	start := time.Now()
	resp, err := sendReqWithTimeout(req, time.Duration(jobReq.Timeout)*time.Second)
	if err != nil || resp.StatusCode > 399 {
		clog.Errorf(ctx, "job not able to be processed err=%v ", err.Error())

		if err == context.DeadlineExceeded {
			orch.RemoveExternalCapability(jobReq.Capability)
		}

		orch.FreeExternalCapabilityCapacity(jobReq.Capability)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	took := time.Since(start)
	// Debit the fee for the total time processed
	h.orchestrator.DebitFees(sender, core.ManifestID(jobReq.Capability), payment.GetExpectedPrice(), int64(took.Seconds()))

	//check balance and return remaning balance in header of response
	senderBalance := h.orchestrator.Balance(sender, core.ManifestID(jobReq.Capability))
	if senderBalance == nil {
		senderBalance = big.NewRat(0, 0)
	}
	senderBalAmt, _ := common.PriceToInt64(senderBalance)
	if senderBalAmt.Cmp(big.NewRat(0, 0)) < 0 {
		clog.Errorf(ctx, "Sender balance is negative, sender=%v capability=%v", sender.Hex(), jobReq.Capability)
		http.Error(w, "Sender balance is negative", http.StatusPaymentRequired)
		return
	}

	clog.V(common.SHORT).Infof(ctx, "Job processed successfully took=%v", took)

	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		clog.Errorf(ctx, "Unable to read response err=%v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Livepeer-Payment-Balance", senderBalAmt.FloatString(0))
	w.Write(data)

}

func verifyJobCreds(ctx context.Context, orch Orchestrator, jobCreds string, requestedBy ethcommon.Address) (*JobRequest, context.Context, error) {
	buf, err := base64.StdEncoding.DecodeString(jobCreds)
	if err != nil {
		glog.Error("Unable to base64-decode ", err)
		return nil, ctx, errSegEncoding
	}

	var jobData JobRequest
	err = json.Unmarshal(buf, jobData)
	if err != nil {
		glog.Error("Unable to unmarshal ", err)
		return nil, ctx, err
	}

	ctx = clog.AddVal(ctx, "job_id", jobData.ID)

	if !orch.VerifySig(requestedBy, jobData.Request, []byte(jobData.Sig)) {
		clog.Errorf(ctx, "Sig check failed")
		return nil, ctx, errSegSig
	}

	// Check that auth token is valid and not expired
	if jobData.Token == nil {
		return nil, ctx, errors.New("missing auth token")
	}

	reqToken, err := base64.StdEncoding.DecodeString(jobData.Token.Token)
	if err != nil {
		return nil, ctx, errors.New("missing auth token")
	}
	verifyToken := orch.AuthToken(jobData.Token.JobId, jobData.Token.Expiration)
	if !bytes.Equal(verifyToken.Token, reqToken) {
		return nil, ctx, errors.New("invalid auth token")
	}
	ctx = clog.AddOrchSessionID(ctx, jobData.Token.JobId)

	expiration := time.Unix(jobData.Token.Expiration, 0)
	if time.Now().After(expiration) {
		return nil, ctx, errors.New("expired auth token")
	}

	if orch.ReserveExternalCapabilityCapacity(jobData.Capability) != nil {
		clog.Errorf(ctx, "Cannot process job err=%q", err)
		return nil, ctx, errZeroCapacity
	}

	jobData.CapabilityUrl = orch.GetUrlForCapability(jobData.Capability)

	return &jobData, ctx, nil
}

func newJobId() string {
	return string(core.RandomManifestID())
}
