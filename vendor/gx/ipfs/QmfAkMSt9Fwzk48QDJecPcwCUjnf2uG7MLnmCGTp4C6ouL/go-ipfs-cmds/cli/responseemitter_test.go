package cli

import (
	"bytes"
	//"io"
	"testing"

	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
	"gx/ipfs/QmfAkMSt9Fwzk48QDJecPcwCUjnf2uG7MLnmCGTp4C6ouL/go-ipfs-cmds"
)

type writeCloser struct {
	*bytes.Buffer
}

func (wc writeCloser) Close() error { return nil }

type tcSetError struct {
	stdout, stderr     *bytes.Buffer
	exStdout, exStderr string
	exExit             int
	f                  func(re ResponseEmitter, t *testing.T)
}

func (tc tcSetError) Run(t *testing.T) {
	req := &cmds.Request{}

	cmdsre, exitCh := NewResponseEmitter(tc.stdout, tc.stderr, nil, req)

	re := cmdsre.(ResponseEmitter)

	go tc.f(re, t)

	if exitCode := <-exitCh; exitCode != tc.exExit {
		t.Fatalf("expected exit code %d, got %d", tc.exExit, exitCode)
	}

	if tc.stdout.String() != tc.exStdout {
		t.Fatalf(`expected stdout string "%s" but got "%s"`, tc.exStdout, tc.stdout.String())
	}

	if tc.stderr.String() != tc.exStderr {
		t.Fatalf(`expected stderr string "%s" but got "%s"`, tc.exStderr, tc.stderr.String())
	}

	t.Logf("stdout:\n---\n%s---\n", tc.stdout.Bytes())
	t.Logf("stderr:\n---\n%s---\n", tc.stderr.Bytes())
}

func TestSetError(t *testing.T) {
	tcs := []tcSetError{
		tcSetError{
			stdout:   bytes.NewBuffer(nil),
			stderr:   bytes.NewBuffer(nil),
			exStdout: "a\n",
			exStderr: "Error: some error\n",
			exExit:   1,
			f: func(re ResponseEmitter, t *testing.T) {
				re.Emit("a")
				re.SetError("some error", cmdkit.ErrFatal)
				re.Emit("b")
			},
		},

		tcSetError{
			stdout:   bytes.NewBuffer(nil),
			stderr:   bytes.NewBuffer(nil),
			exStdout: "a\nb\n",
			exStderr: "Error: some error\n",
			exExit:   1,
			f: func(re ResponseEmitter, t *testing.T) {
				defer re.Close()
				re.Emit("a")
				re.SetError("some error", cmdkit.ErrNormal)
				re.Emit("b")
			},
		},

		tcSetError{
			stdout:   bytes.NewBuffer(nil),
			stderr:   bytes.NewBuffer(nil),
			exStdout: "a\nb\n",
			exStderr: "Error: some error\n",
			exExit:   3,
			f: func(re ResponseEmitter, t *testing.T) {
				re.Emit("a")
				re.SetError("some error", cmdkit.ErrNormal)
				re.Emit("b")
				re.Exit(3)
			},
		},
	}

	for i, tc := range tcs {
		t.Log(i)
		tc.Run(t)
	}
}
