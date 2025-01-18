package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/deepmap/oapi-codegen/v2/pkg/securityprovider"
)

type RunnerContainerType int

const (
	Managed RunnerContainerType = iota
	External
)

type RunnerContainer struct {
	RunnerContainerConfig
	Name     string
	Client   *ClientWithResponses
	Hardware *HardwareInformation
}

type RunnerEndpoint struct {
	URL   string
	Token string
}

type RunnerContainerConfig struct {
	Type             RunnerContainerType
	Pipeline         string
	ModelID          string
	Endpoint         RunnerEndpoint
	ContainerImageID string

	// For managed containers only
	ID               string
	GPU              string
	KeepWarm         bool
	containerTimeout time.Duration
}

// Create global references to functions to allow for mocking in tests.
var runnerWaitUntilReadyFunc = runnerWaitUntilReady

func NewRunnerContainer(ctx context.Context, cfg RunnerContainerConfig, name string) (*RunnerContainer, error) {
	// Ensure that timeout is set to a non-zero value.
	timeout := cfg.containerTimeout
	if timeout == 0 {
		timeout = containerTimeout
	}

	var opts []ClientOption
	if cfg.Endpoint.Token != "" {
		bearerTokenProvider, err := securityprovider.NewSecurityProviderBearerToken(cfg.Endpoint.Token)
		if err != nil {
			return nil, err
		}

		opts = append(opts, WithRequestEditorFn(bearerTokenProvider.Intercept))
	}

	client, err := NewClientWithResponses(cfg.Endpoint.URL, opts...)
	if err != nil {
		return nil, err
	}

	cctx, cancel := context.WithTimeout(ctx, cfg.containerTimeout)
	defer cancel()
	if err := runnerWaitUntilReadyFunc(cctx, client, pollingInterval); err != nil {
		return nil, err
	}

	var hardware *HardwareInformation
	hctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	hdw, err := getRunnerHardware(hctx, client)
	if err != nil {
		hardware = &HardwareInformation{Pipeline: cfg.Pipeline, ModelId: cfg.ModelID, GpuInfo: nil}
	} else {
		hardware = hdw
	}

	return &RunnerContainer{
		RunnerContainerConfig: cfg,
		Name:                  name,
		Client:                client,
		Hardware:              hardware,
	}, nil
}

func runnerWaitUntilReady(ctx context.Context, client *ClientWithResponses, pollingInterval time.Duration) error {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

tickerLoop:
	for range ticker.C {
		select {
		case <-ctx.Done():
			return errors.New("timed out waiting for runner")
		default:
			if _, err := client.HealthWithResponse(ctx); err == nil {
				break tickerLoop
			}
		}
	}

	return nil
}

func getRunnerHardware(ctx context.Context, client *ClientWithResponses) (*HardwareInformation, error) {
	resp, err := client.HardwareInfoWithResponse(ctx)
	if err != nil {
		slog.Error("Error getting hardware info for runner", slog.String("error", err.Error()))
		return nil, err
	}

	return resp.JSON200, nil
}
