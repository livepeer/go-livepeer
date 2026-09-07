package byoc

import (
	"context"
	"encoding/base64"
	"math"
	"math/big"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/pkg/errors"
)

type sharedBalanceNode interface {
	Node() *core.LivepeerNode
	SharedBalanceLock() *sync.Mutex
}

// compares expected balance with current balance and updates accordingly with the expected balance being the target
// returns the difference and if minimum balance was covered
// also returns if balance was reset to zero because expected was zero
func compareAndUpdateBalance(node sharedBalanceNode, addr ethcommon.Address, id string, expected *big.Rat, minimumBal *big.Rat) (*big.Rat, *big.Rat, bool, bool) {
	lpNode := node.Node()
	sharedBalMtx := node.SharedBalanceLock()
	sharedBalMtx.Lock()
	defer sharedBalMtx.Unlock()

	current := lpNode.Balances.Balance(addr, core.ManifestID(id))
	if current == nil {
		//create a balance of 1 to start tracking
		lpNode.Balances.Debit(addr, core.ManifestID(id), big.NewRat(0, 1))
		current = lpNode.Balances.Balance(addr, core.ManifestID(id))
	}
	if expected == nil {
		expected = big.NewRat(0, 1)
	}
	diff := new(big.Rat).Sub(expected, current)

	if diff.Sign() > 0 {
		lpNode.Balances.Credit(addr, core.ManifestID(id), diff)
	} else {
		lpNode.Balances.Debit(addr, core.ManifestID(id), new(big.Rat).Abs(diff))
	}

	var resetToZero bool
	if expected.Sign() == 0 {
		lpNode.Balances.Debit(addr, core.ManifestID(id), current)

		resetToZero = true
	}

	//get updated balance after changes
	current = lpNode.Balances.Balance(addr, core.ManifestID(id))

	var minimumBalCovered bool
	if current.Cmp(minimumBal) >= 0 {
		minimumBalCovered = true
	}

	return current, diff, minimumBalCovered, resetToZero
}

func (bsg *BYOCGatewayServer) createPayment(ctx context.Context, jobReq *JobRequest, orchToken *JobToken) (string, error) {
	if orchToken == nil {
		return "", errors.New("orchestrator token is nil, cannot create payment")
	}
	//if no sender or ticket params, no payment
	if bsg.node.Sender == nil {
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
	sessionID := bsg.node.Sender.StartSession(*pmTicketParams(orchToken.TicketParams))

	//setup balances and update Gateway balance to Orchestrator balance, log differences
	//Orchestrator tracks balance paid and will not perform work if the balance it
	//has is not sufficient
	orchBal := big.NewRat(orchToken.Balance, 1)
	price := big.NewRat(orchToken.Price.PricePerUnit, orchToken.Price.PixelsPerUnit)
	cost := new(big.Rat).Mul(price, big.NewRat(int64(jobReq.Timeout), 1))
	minBal := new(big.Rat).Mul(price, big.NewRat(120, 1)) //minimum 2 minute balance, Orchestrator requires 1 minute.  Use 2 to have a buffer.
	balance, diffToOrch, minBalCovered, resetToZero := compareAndUpdateBalance(bsg, orchAddr, jobReq.Capability, orchBal, minBal)

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
		ticketParams := pmTicketParams(orchToken.TicketParams)
		//create a ticket to calc num tickets needed. If ticketEv is not the same as price then ticket count should reflect
		// ev to cover the cost for the time requested
		ticket := pm.NewTicket(ticketParams, ticketParams.ExpirationParams, sender, 1)
		ticketEv := ticket.EV()

		ticketCnt := ticketCountForCost(cost, ticketEv, int64(jobReq.Timeout))
		tickets, err := bsg.node.Sender.CreateTicketBatch(sessionID, int(ticketCnt))
		if err != nil {
			clog.Errorf(ctx, "Unable to create ticket batch err=%v", err)
			return "", err
		}

		//record the payment
		winProb := tickets.WinProbRat()
		fv := big.NewRat(tickets.FaceValue.Int64(), 1)
		pmtTotal := new(big.Rat).Mul(fv, winProb)
		pmtTotal = new(big.Rat).Mul(pmtTotal, big.NewRat(int64(ticketCnt), 1))
		bsg.node.Balances.Credit(orchAddr, core.ManifestID(jobReq.Capability), pmtTotal)
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

		senderParams := make([]*net.TicketSenderParams, len(tickets.SenderParams))
		for i := 0; i < len(tickets.SenderParams); i++ {
			senderParams[i] = &net.TicketSenderParams{
				SenderNonce: orchToken.LastNonce + tickets.SenderParams[i].SenderNonce,
				Sig:         tickets.SenderParams[i].Sig,
			}
		}
		orchToken.LastNonce = tickets.SenderParams[len(tickets.SenderParams)-1].SenderNonce + 1
		payment.TicketSenderParams = senderParams

		ratPrice, _ := common.RatPriceInfo(payment.ExpectedPrice)
		balanceForOrch := bsg.node.Balances.Balance(orchAddr, core.ManifestID(jobReq.Capability))
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

func ticketCountForCost(cost *big.Rat, ticketEv *big.Rat, timeoutSeconds int64) int64 {
	if ticketEv.Cmp(big.NewRat(0, 1)) == 0 {
		return 0
	}
	ticketCntRat := new(big.Rat).Quo(cost, ticketEv)
	ticketCnt, _ := ticketCntRat.Float64()
	if math.IsInf(ticketCnt, 0) {
		//return base of 1 ticket per second
		return timeoutSeconds
	}
	// pm.maxSenderNonces is 600
	if ticketCnt > 600 {
		//return best effort max
		return 600
	}

	//return ticket count to cover the cost for the cost based on ticket ev
	return int64(math.Max(0, math.Ceil(ticketCnt)))
}

func updateGatewayBalance(node *core.LivepeerNode, orchToken JobToken, capability string, took time.Duration) *big.Rat {
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
