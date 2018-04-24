package http

import (
	"context"
	"runtime"
	"testing"

	cmds "gx/ipfs/QmfAkMSt9Fwzk48QDJecPcwCUjnf2uG7MLnmCGTp4C6ouL/go-ipfs-cmds"
)

func TestHTTP(t *testing.T) {
	type testcase struct {
		path []string
		v    interface{}
	}

	tcs := []testcase{
		{
			path: []string{"version"},
			v: VersionOutput{ // handler_test:/^func getTestServer/
				Version: "0.1.2",
				Commit:  "c0mm17",
				Repo:    "4",
				System:  runtime.GOARCH + "/" + runtime.GOOS, //TODO: Precise version here
				Golang:  runtime.Version(),
			},
		},
	}

	for _, tc := range tcs {
		srv := getTestServer(t, nil)
		c := NewClient(srv.URL)
		req, err := cmds.NewRequest(context.Background(), tc.path, nil, nil, nil, cmdRoot)
		if err != nil {
			t.Fatal(err)
		}

		res, err := c.Send(req)
		if err != nil {
			t.Fatal(err)
		}

		iv, err := res.Next()
		if err != nil {
			t.Fatal(err)
		}

		v := iv.(*VersionOutput)

		if *v != tc.v {
			t.Errorf("expected value to be %v but got %v", tc.v, v)
		}
	}
}
