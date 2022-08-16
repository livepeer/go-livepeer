package main

import (
	"flag"
	"os"
	"testing"

	"github.com/peterbourgon/ff/v3"
	"github.com/stretchr/testify/require"
)

// This test exists because we wanted to make our CLI argument casing consistent without
// breaking backwards compatibility
func TestParseAcceptsEitherDatadirCasing(t *testing.T) {
	// Reset between tests
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// No Casing
	lpc := parseLivepeerConfig()

	err := ff.Parse(flag.CommandLine, []string{
		"-datadir", "/some/data/dir",
	})
	require.NoError(t, err)
	require.NotNil(t, lpc.Datadir)
	require.Equal(t, "/some/data/dir", *lpc.Datadir)

	// Reset between tests
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Camel Casing
	lpc = parseLivepeerConfig()

	err = ff.Parse(flag.CommandLine, []string{
		"-dataDir", "/some/data/dir",
	})
	require.NoError(t, err)
	require.NotNil(t, lpc.Datadir)
	require.Equal(t, "/some/data/dir", *lpc.Datadir)
}
