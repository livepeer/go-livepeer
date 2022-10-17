package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Necessary to avoid our flags being overridden by test flags
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestMistJSON(t *testing.T) {

	// monkeypatch stdout and os.Args
	oldArgs := os.Args
	os.Args = []string{"livepeer", "-j"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	defer func() {
		os.Args = oldArgs
		os.Stdout = oldStdout
	}()

	c := make(chan []byte)
	go func() {
		out, err := ioutil.ReadAll(r)
		require.Nil(t, err)
		c <- out
	}()

	main()
	w.Close()
	out := <-c

	// Just check for now that we can unmarshal the JSON
	var input interface{}
	err := json.Unmarshal(out, &input)
	require.Nil(t, err)
}
