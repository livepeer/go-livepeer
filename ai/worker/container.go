package worker

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
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
	Version  *Version

	BorrowCtx context.Context
	sync.RWMutex
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
	ID                string
	GPU               string
	KeepWarm          bool
	OptimizationFlags OptimizationFlags
	containerTimeout  time.Duration
}

// Create global references to functions to allow for mocking in tests.
var runnerWaitUntilReadyFunc = runnerWaitUntilReady

func NewRunnerContainer(ctx context.Context, cfg RunnerContainerConfig, name string) (rc *RunnerContainer, isLoading bool, err error) {
	// Ensure that timeout is set to a non-zero value.
	timeout := cfg.containerTimeout
	if timeout == 0 {
		timeout = containerTimeout
	}

	var opts []ClientOption
	if cfg.Endpoint.Token != "" {
		bearerTokenProvider, err := securityprovider.NewSecurityProviderBearerToken(cfg.Endpoint.Token)
		if err != nil {
			return nil, false, err
		}

		opts = append(opts, WithRequestEditorFn(bearerTokenProvider.Intercept))
	}

	client, err := NewClientWithResponses(cfg.Endpoint.URL, opts...)
	if err != nil {
		return nil, false, err
	}

	cctx, cancel := context.WithTimeout(ctx, cfg.containerTimeout)
	defer cancel()
	isLoading, err = runnerWaitUntilReadyFunc(cctx, client, pollingInterval)
	if err != nil {
		return nil, isLoading, err
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
	runnerVersion := &Version{Pipeline: cfg.Pipeline, ModelId: cfg.ModelID, Version: "0.0.0"}
	version, err := client.VersionWithResponse(ctx)
	if err != nil {
		glog.Error("Error getting runner version", err)
	} else if version.StatusCode() != http.StatusOK {
		glog.Error("Error getting runner version", version.StatusCode(), string(version.Body))
	} else {
		runnerVersion = version.JSON200
		glog.Info("Started runner with version", runnerVersion, " version=", runnerVersion.Version, " body=", string(version.Body))
	}

	return &RunnerContainer{
		RunnerContainerConfig: cfg,
		Name:                  name,
		Client:                client,
		Hardware:              hardware,
		Version:               runnerVersion,
	}, isLoading, nil
}

func runnerWaitUntilReady(ctx context.Context, client *ClientWithResponses, pollingInterval time.Duration) (isLoading bool, err error) {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("timed out waiting for runner: %w", lastErr)
		case <-ticker.C:
			reqCtx, cancel := context.WithTimeout(ctx, healthcheckTimeout)
			health, err := client.HealthWithResponse(reqCtx)
			cancel()
			if err != nil {
				lastErr = err
			} else if httpStatus := health.StatusCode(); httpStatus != http.StatusOK {
				lastErr = fmt.Errorf("health check failed with status code %d", httpStatus)
			} else if st := health.JSON200.Status; st == ERROR {
				return false, fmt.Errorf("runner is in error state")
			} else {
				// any other state means the container is ready
				isLoading = st == "LOADING" // TODO: Use enum when ai-runner SDK is updated
				return isLoading, nil
			}
		}
	}
}

func getRunnerHardware(ctx context.Context, client *ClientWithResponses) (*HardwareInformation, error) {
	resp, err := client.HardwareInfoWithResponse(ctx)
	if err != nil {
		slog.Error("Error getting hardware info for runner", slog.String("error", err.Error()))
		return nil, err
	}

	return resp.JSON200, nil
}
