package worker

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const (
	gvisorRuntimeName = "runsc"
	runscInstallPath  = "/usr/local/bin/runsc"
	gvisorBaseURL     = "https://storage.googleapis.com/gvisor/releases/release/latest"
)

// SetupRuntime validates and prepares the container runtime. For "gvisor", it
// ensures the runsc binary is available (downloading it if necessary) and
// returns the Docker runtime name "runsc". An empty runtimeFlag returns ""
// which tells Docker to use its default runtime.
func SetupRuntime(runtimeFlag string) (string, error) {
	if runtimeFlag == "" {
		return "", nil
	}

	if runtimeFlag != "gvisor" {
		return "", fmt.Errorf("unsupported AI runtime: %q (supported values: \"gvisor\")", runtimeFlag)
	}

	path, err := exec.LookPath(gvisorRuntimeName)
	if err == nil {
		slog.Info("Found existing runsc binary", "path", path)
	} else {
		slog.Info("runsc binary not found on PATH, downloading...")
		if err := downloadRunsc(); err != nil {
			return "", fmt.Errorf("failed to download runsc: %w", err)
		}
		slog.Info("Successfully installed runsc", "path", runscInstallPath)
	}

	slog.Warn("Ensure the Docker daemon is configured with the runsc runtime for GPU support. " +
		"Add to /etc/docker/daemon.json: " +
		`{"runtimes": {"runsc": {"path": "/usr/local/bin/runsc", "runtimeArgs": ["--nvproxy"]}}} ` +
		"then restart Docker (systemctl restart docker).")

	return gvisorRuntimeName, nil
}

// downloadRunsc downloads the runsc binary from the official gVisor release
// URL, verifies its SHA512 checksum, and installs it to runscInstallPath.
func downloadRunsc() error {
	arch := runscArch()

	binaryURL := fmt.Sprintf("%s/%s/runsc", gvisorBaseURL, arch)
	checksumURL := binaryURL + ".sha512"

	// Download checksum first.
	checksumResp, err := http.Get(checksumURL)
	if err != nil {
		return fmt.Errorf("failed to download checksum: %w", err)
	}
	defer checksumResp.Body.Close()

	if checksumResp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download checksum: HTTP %d", checksumResp.StatusCode)
	}

	checksumBytes, err := io.ReadAll(checksumResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read checksum: %w", err)
	}
	expectedChecksum := strings.Fields(string(checksumBytes))[0]

	// Download the binary.
	binaryResp, err := http.Get(binaryURL)
	if err != nil {
		return fmt.Errorf("failed to download runsc: %w", err)
	}
	defer binaryResp.Body.Close()

	if binaryResp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download runsc: HTTP %d", binaryResp.StatusCode)
	}

	// Write to a temp file first, then rename for atomicity.
	tmpFile, err := os.CreateTemp("", "runsc-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	hasher := sha512.New()
	writer := io.MultiWriter(tmpFile, hasher)

	if _, err := io.Copy(writer, binaryResp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write runsc binary: %w", err)
	}
	tmpFile.Close()

	// Verify checksum.
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	// Make executable and move to install path.
	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		return fmt.Errorf("failed to chmod runsc: %w", err)
	}

	if err := os.Rename(tmpFile.Name(), runscInstallPath); err != nil {
		return fmt.Errorf("failed to install runsc to %s: %w", runscInstallPath, err)
	}

	return nil
}

// runscArch returns the architecture string used in gVisor download URLs.
func runscArch() string {
	switch runtime.GOARCH {
	case "arm64":
		return "aarch64"
	default:
		return "x86_64"
	}
}
