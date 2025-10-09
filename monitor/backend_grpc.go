package monitor

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/metadata"
)

const grpcCodecName = "json"

type jsonCodec struct{}

func (jsonCodec) Name() string {
	return grpcCodecName
}

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	if v == nil {
		return nil
	}
	return json.Unmarshal(data, v)
}

type grpcBackend struct {
	conn    *grpc.ClientConn
	method  string
	headers map[string]string
	timeout time.Duration
}

func init() {
	encoding.RegisterCodec(jsonCodec{})
	RegisterBackendFactory("grpc", newGRPCBackend)
	RegisterBackendFactory("grpcs", newGRPCBackend)
}

func newGRPCBackend(u *url.URL, opts BackendOptions) (EventBackend, error) {
	method := strings.TrimSpace(u.Fragment)
	if method == "" {
		method = strings.TrimPrefix(u.Path, "/")
	}
	if method == "" {
		return nil, fmt.Errorf("grpc sink %q missing method (set path or fragment)", u.String())
	}
	if !strings.Contains(method, ".") && !strings.Contains(method, "/") {
		return nil, fmt.Errorf("grpc sink method %q must be fully qualified", method)
	}
	if !strings.HasPrefix(method, "/") {
		method = "/" + method
		method = strings.ReplaceAll(method, "//", "/")
	}

	timeout := 10 * time.Second
	if rawTimeout := u.Query().Get("timeout"); rawTimeout != "" {
		dur, err := time.ParseDuration(rawTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout %q for grpc sink %s: %w", rawTimeout, u.String(), err)
		}
		timeout = dur
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	dialOpts := []grpc.DialOption{grpc.WithBlock()}

	if strings.EqualFold(u.Scheme, "grpcs") {
		tlsCfg := &tls.Config{}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	targetHost := u.Host
	if targetHost == "" {
		return nil, fmt.Errorf("grpc sink %q missing host", u.String())
	}

	conn, err := grpc.DialContext(dialCtx, targetHost, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("dial grpc sink %s: %w", u.String(), err)
	}

	return &grpcBackend{
		conn:    conn,
		method:  method,
		headers: opts.Headers,
		timeout: timeout,
	}, nil
}

func (b *grpcBackend) Start(_ context.Context) error {
	return nil
}

func (b *grpcBackend) Publish(ctx context.Context, batch []EventEnvelope) error {
	if len(batch) == 0 {
		return nil
	}

	callCtx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	if len(b.headers) > 0 {
		pairs := make([]string, 0, len(b.headers)*2)
		for k, v := range b.headers {
			key := strings.ToLower(k)
			pairs = append(pairs, key, v)
		}
		md := metadata.Pairs(pairs...)
		callCtx = metadata.NewOutgoingContext(callCtx, md)
	}

	var resp json.RawMessage
	if err := b.conn.Invoke(callCtx, b.method, batch, &resp, grpc.CallContentSubtype(grpcCodecName)); err != nil {
		return err
	}
	return nil
}

func (b *grpcBackend) Stop(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := b.conn.Close(); err != nil {
			// connection close errors are logged but not fatal
		}
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
