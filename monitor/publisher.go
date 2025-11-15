package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/google/uuid"
)

type EventEnvelope struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Gateway   string          `json:"gateway,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

type PublisherConfig struct {
	GatewayAddress string
	SinkURLs       []string
	Headers        map[string]string
	QueueSize      int
	BatchSize      int
	FlushInterval  time.Duration
}

type BackendOptions struct {
	Headers        map[string]string
	GatewayAddress string
}

type EventBackend interface {
	Start(ctx context.Context) error
	Publish(ctx context.Context, batch []EventEnvelope) error
	Stop(ctx context.Context) error
}

type BackendFactory func(u *url.URL, opts BackendOptions) (EventBackend, error)

type backendEntry struct {
	name    string
	backend EventBackend
}

type publisher struct {
	ctx           context.Context
	cancel        context.CancelFunc
	queue         chan EventEnvelope
	batchSize     int
	flushInterval time.Duration
	gateway       string
	backends      []backendEntry
	wg            sync.WaitGroup
}

var (
	publisherMu      sync.RWMutex
	activePublisher  *publisher
	backendFactories = make(map[string]BackendFactory)
	factoriesMu      sync.RWMutex
)

func RegisterBackendFactory(scheme string, factory BackendFactory) {
	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	backendFactories[strings.ToLower(scheme)] = factory
}

func InitEventPublisher(cfg PublisherConfig) error {
	if len(cfg.SinkURLs) == 0 {
		return fmt.Errorf("event publisher requires at least one sink URL")
	}

	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	if len(backendFactories) == 0 {
		return fmt.Errorf("no event backend factories registered")
	}

	pub, err := newPublisher(cfg)
	if err != nil {
		return err
	}

	publisherMu.Lock()
	if activePublisher != nil {
		go func(old *publisher) {
			if err := old.Stop(context.Background()); err != nil {
				glog.Errorf("event publisher shutdown error: %v", err)
			}
		}(activePublisher)
	}
	activePublisher = pub
	publisherMu.Unlock()

	return nil
}

func ShutdownEventPublisher(ctx context.Context) error {
	publisherMu.Lock()
	pub := activePublisher
	activePublisher = nil
	publisherMu.Unlock()
	if pub == nil {
		return nil
	}
	return pub.Stop(ctx)
}

func QueueEvent(eventType string, payload interface{}) {
	publisherMu.RLock()
	pub := activePublisher
	publisherMu.RUnlock()

	if pub == nil {
		return
	}

	envelope, err := pub.buildEnvelope(eventType, payload)
	if err != nil {
		glog.Errorf("event publisher failed to encode payload for %s: %v", eventType, err)
		return
	}

	select {
	case pub.queue <- envelope:
	default:
		glog.Warningf("event publisher queue full, dropping event %q", eventType)
	}
}

func newPublisher(cfg PublisherConfig) (*publisher, error) {
	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = 100
	}
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	flushInterval := cfg.FlushInterval
	if flushInterval <= 0 {
		flushInterval = time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	pub := &publisher{
		ctx:           ctx,
		cancel:        cancel,
		queue:         make(chan EventEnvelope, queueSize),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		gateway:       cfg.GatewayAddress,
	}

	opts := BackendOptions{
		Headers:        cfg.Headers,
		GatewayAddress: cfg.GatewayAddress,
	}

	for _, raw := range cfg.SinkURLs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("parse sink url %q: %w", raw, err)
		}
		factory, ok := backendFactories[strings.ToLower(u.Scheme)]
		if !ok {
			cancel()
			return nil, fmt.Errorf("no backend registered for scheme %q", u.Scheme)
		}
		backend, err := factory(u, opts)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("init backend %q: %w", raw, err)
		}
		if err := backend.Start(ctx); err != nil {
			cancel()
			backend.Stop(context.Background())
			return nil, fmt.Errorf("start backend %q: %w", raw, err)
		}
		pub.backends = append(pub.backends, backendEntry{name: raw, backend: backend})
	}

	if len(pub.backends) == 0 {
		cancel()
		return nil, fmt.Errorf("no valid event sinks configured")
	}

	pub.wg.Add(1)
	go pub.run()

	return pub, nil
}

func (p *publisher) Stop(ctx context.Context) error {
	p.cancel()
	close(p.queue)

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		p.wg.Wait()
	}()

	select {
	case <-stopped:
	case <-ctx.Done():
	}

	var errs []string
	for _, entry := range p.backends {
		if err := entry.backend.Stop(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", entry.name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("backend shutdown errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (p *publisher) run() {
	defer p.wg.Done()

	ticker := time.NewTimer(p.flushInterval)
	defer ticker.Stop()

	batch := make([]EventEnvelope, 0, p.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		copyBatch := make([]EventEnvelope, len(batch))
		copy(copyBatch, batch)
		for _, entry := range p.backends {
			if err := entry.backend.Publish(p.ctx, copyBatch); err != nil {
				glog.Errorf("event publisher backend %s publish error: %v", entry.name, err)
			}
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-p.ctx.Done():
			flush()
			return
		case evt, ok := <-p.queue:
			if !ok {
				flush()
				return
			}
			batch = append(batch, evt)
			if len(batch) >= p.batchSize {
				flush()
				if !ticker.Stop() {
					<-ticker.C
				}
				ticker.Reset(p.flushInterval)
			}
		case <-ticker.C:
			flush()
			ticker.Reset(p.flushInterval)
		}
	}
}

func (p *publisher) buildEnvelope(eventType string, payload interface{}) (EventEnvelope, error) {
	raw, err := normalizePayload(payload)
	if err != nil {
		return EventEnvelope{}, err
	}
	return EventEnvelope{
		ID:        uuid.NewString(),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Gateway:   p.gateway,
		Payload:   raw,
	}, nil
}

func normalizePayload(payload interface{}) (json.RawMessage, error) {
	if payload == nil {
		return json.RawMessage([]byte("null")), nil
	}

	switch v := payload.(type) {
	case json.RawMessage:
		clone := make([]byte, len(v))
		copy(clone, v)
		return json.RawMessage(clone), nil
	case []byte:
		if json.Valid(v) {
			clone := make([]byte, len(v))
			copy(clone, v)
			return json.RawMessage(clone), nil
		}
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(b), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(b), nil
	}
}
