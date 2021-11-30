package server

import (
	"context"
	"errors"
	"fmt"
	gonet "net"
	"net/url"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/net"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const getOrchestratorTimeout = 2 * time.Second

var errNoOrchestrators = errors.New("no orchestrators")

type Router struct {
	uris []*url.URL
	srv  *grpc.Server
}

func NewRouter(uris []*url.URL) *Router {
	return &Router{uris: uris}
}

func (r *Router) Start(uri *url.URL, serviceURI *url.URL, workDir string) error {
	listener, err := gonet.Listen("tcp", uri.Host)
	if err != nil {
		return err
	}
	defer listener.Close()

	certFile, keyFile, err := getCert(serviceURI, workDir)
	if err != nil {
		return err
	}

	creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		return err
	}

	s := grpc.NewServer(grpc.Creds(creds))
	r.srv = s

	net.RegisterOrchestratorServer(s, r)
	grpc_health_v1.RegisterHealthServer(s, health.NewServer())

	errCh := make(chan error)
	go func() {
		errCh <- s.Serve(listener)
	}()

	time.Sleep(1 * time.Second)

	if err := checkAvailability(context.Background(), serviceURI); err != nil {
		s.Stop()
		return err
	}

	glog.Infof("Started router server at %v", uri)

	return <-errCh
}

func (r *Router) Stop() {
	r.srv.Stop()
}

func (r *Router) GetOrchestrator(ctx context.Context, req *net.OrchestratorRequest) (*net.OrchestratorInfo, error) {
	return getOrchestratorInfo(ctx, r.uris, req)
}

func (r *Router) Ping(ctx context.Context, req *net.PingPong) (*net.PingPong, error) {
	return &net.PingPong{Value: []byte{}}, nil
}

func checkAvailability(logCtx context.Context, uri *url.URL) error {
	client, conn, err := startOrchestratorClient(logCtx, uri)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), GRPCConnectTimeout)
	defer cancel()

	_, err = client.Ping(ctx, &net.PingPong{Value: []byte{}})
	if err != nil {
		return err
	}

	return nil
}

func getOrchestratorInfo(ctx context.Context, uris []*url.URL, req *net.OrchestratorRequest) (*net.OrchestratorInfo, error) {
	if len(uris) == 0 {
		return nil, errNoOrchestrators
	}

	infoCh := make(chan *net.OrchestratorInfo)
	errCh := make(chan error, len(uris))

	cctx, cancel := context.WithTimeout(ctx, getOrchestratorTimeout)
	defer cancel()

	for _, uri := range uris {
		go func(uri *url.URL) {
			client, conn, err := startOrchestratorClient(ctx, uri)
			if err != nil {
				errCh <- fmt.Errorf("%v err=%v", uri, err)
				return
			}
			defer conn.Close()

			info, err := client.GetOrchestrator(cctx, req)
			if err != nil {
				errCh <- fmt.Errorf("%v err=%v", uri, err)
				return
			}

			select {
			case infoCh <- info:
			default:
			}
		}(uri)
	}

	errCtr := 0
	for {
		select {
		case info := <-infoCh:
			glog.Infof("Forwarding OrchestratorInfo orch=%v", info.Transcoder)
			return info, nil
		case err := <-errCh:
			glog.Error(err)
			errCtr++
			if errCtr >= len(uris) {
				return nil, errNoOrchestrators
			}
		case <-cctx.Done():
			return nil, errors.New("timed out")
		}
	}
}
