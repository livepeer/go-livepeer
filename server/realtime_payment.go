package server

import "time"

type StreamInfo struct {
	Dur      time.Duration
	StreamID string
	// TODO: Some other stream params needed to payment
}

// RealtimePaymentSender is used im Gateway to send payment to Orchestrator
type RealtimePaymentSender interface {
	// SendPayment process the streamInfo and sends a payment to Orchestrator if needed
	SendPayment(streamInfo StreamInfo) error
}

// RealtimePaymentValidator is used in Orchestrator to validate if the stream is paid
type RealtimePaymentValidator interface {
	// ValidatePayment checks if the stream is paid and if not it returns error, so that stream can be stopped
	ValidatePayment(streamInfo StreamInfo) error
}
