package worker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetupRuntime_Empty(t *testing.T) {
	runtime, err := SetupRuntime("")
	require.NoError(t, err)
	require.Equal(t, "", runtime)
}

func TestSetupRuntime_Unsupported(t *testing.T) {
	_, err := SetupRuntime("kata")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported AI runtime")
}

func TestSetupRuntime_GvisorFound(t *testing.T) {
	// Create a fake runsc binary on a temp PATH.
	tmpDir := t.TempDir()
	fakeRunsc := filepath.Join(tmpDir, "runsc")
	require.NoError(t, os.WriteFile(fakeRunsc, []byte("#!/bin/sh\n"), 0755))

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+":"+origPath)

	runtime, err := SetupRuntime("gvisor")
	require.NoError(t, err)
	require.Equal(t, "runsc", runtime)
}
