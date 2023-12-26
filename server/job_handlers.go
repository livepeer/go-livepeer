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
	"net/http"
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
const jobRegisterCapabilityHdr = "Livepeer-Job-Register-Capability"

type JobSender struct {
	Addr string `json:"addr"`
	Sig  string `json:"sig"`
}

type JobToken struct {
	Token         string            `json:"token"`
	JobId         string            `json:"jobId"`
	Expiration    int64             `json:"expiration"`
	SenderAddress *JobSender        `json:"senderAddress,omitempty"`
	TicketParams  *net.TicketParams `json:"ticketParams,omitempty"`
}

type JobRequest struct {
	ID            string    `json:"id"`
	Request       string    `json:"request"`
	Parameters    string    `json:"parameters"`
	Capability    string    `json:"capability"`
	CapabilityUrl string    `json:"capabilityUrl"` //this i set when verified orch as capability
	Token         *JobToken `json:"token"`
	Sig           string    `json:"sig"`
	Timeout       int       `json:"timeoutSeconds"`
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
	}

	extCap := r.Header.Get(jobRegisterCapabilityHdr)
	remoteAddr := getRemoteAddr(r)

	cap, err := orch.ExternalCapabilities().RegisterCapability(extCap)

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}

	clog.Infof(context.TODO(), "registered capability remoteAddr=%v capability=%v url=%v", remoteAddr, cap.Name, cap.Url.RequestURI())
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

	if !orch.ExternalCapabilities().CompatibleWith(jobCapsHdr) {
		w.WriteHeader(http.StatusNoContent)
		jobToken = JobToken{Token: "", JobId: "", Expiration: 0, SenderAddress: nil, TicketParams: nil}
	} else {
		senderAddr := ethcommon.HexToAddress(jobSenderAddr.Addr)
		jobId := core.RandomManifestID()

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

		jobToken = JobToken{Token: base64.StdEncoding.EncodeToString(token.Token),
			JobId:         token.SessionId,
			Expiration:    token.Expiration,
			SenderAddress: jobSenderAddr,
			TicketParams:  ticketParams,
		}

		//send response
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
	ctx := clog.AddVal(r.Context(), clog.ClientIP, remoteAddr)

	payment, err := getPayment(r.Header.Get(paymentHeader))
	if err != nil {
		clog.Errorf(ctx, "Could not parse payment")
		http.Error(w, err.Error(), http.StatusPaymentRequired)
		return
	}

	sender := getPaymentSender(payment)
	ctx = clog.AddVal(ctx, "sender", sender.Hex())

	// check the prompt sig from the request
	job := r.Header.Get(jobRequestHdr)
	jobReq, ctx, err := verifyJobCreds(ctx, orch, job, sender)
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

	clog.V(common.SHORT).Infof(ctx, "Received job, sending for processing id=%v sender=%v ip=%v", jobReq.ID, sender.Hex(), remoteAddr)
	extReq := []byte(fmt.Sprintf(`{"request":%v, "parameters":%v}`, jobReq.Request, jobReq.Parameters))
	req, err := http.NewRequestWithContext(ctx, "POST", jobReq.CapabilityUrl, bytes.NewBuffer(extReq))

	resp, err := sendReqWithTimeout(req, time.Duration(jobReq.Timeout)*time.Second)
	if err != nil || resp.StatusCode > 399 {
		clog.Errorf(ctx, "job not able to be processed err=%v, external_capabilitites=%v", err.Error(), jobReq.Capability)

		if err == context.DeadlineExceeded {
			orch.ExternalCapabilities().RemoveCapability(jobReq.Capability)
		}

		orch.FreeExternalCapacity(jobReq.Capability)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)

		return
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)

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

	orchCaps := orch.ExternalCapabilities()
	if !orchCaps.CompatibleWith(jobData.Capability) {
		clog.Errorf(ctx, "Capability check failed")
		return nil, ctx, errCapCompat
	}

	if err := orch.CheckExternalCapacity(jobData.Capability); err != nil {
		clog.Errorf(ctx, "Cannot process job err=%q", err)
		return nil, ctx, errZeroCapacity
	}

	jobData.CapabilityUrl = orch.GetUrlForCapability(jobData.Capability)

	return &jobData, ctx, nil
}

func NewJobId() string {
	return string(core.RandomManifestID())
}
