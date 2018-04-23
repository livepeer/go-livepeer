package cmds

import (
	"context"
	"io"
	"testing"
	"time"

	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

// nopClose implements io.Close and does nothing
type nopCloser struct{}

func (c nopCloser) Close() error { return nil }

type testEmitter testing.T

func (s *testEmitter) Close() error {
	return nil
}

func (s *testEmitter) SetLength(_ uint64) {}
func (s *testEmitter) SetError(err interface{}, code cmdkit.ErrorType) {
	(*testing.T)(s).Error(err)
}
func (s *testEmitter) Emit(value interface{}) error {
	return nil
}

// newTestEmitter fails the test if it receives an error.
func newTestEmitter(t *testing.T) *testEmitter {
	return (*testEmitter)(t)
}

// noop does nothing and can be used as a noop Run function
func noop(req *Request, re ResponseEmitter, env Environment) {}

// writecloser implements io.WriteCloser by embedding
// an io.Writer and an io.Closer
type writecloser struct {
	io.Writer
	io.Closer
}

// TestOptionValidation tests whether option type validation works
func TestOptionValidation(t *testing.T) {
	cmd := &Command{
		Options: []cmdkit.Option{
			cmdkit.IntOption("b", "beep", "enables beeper"),
			cmdkit.StringOption("B", "boop", "password for booper"),
		},
		Run: noop,
	}

	re := newTestEmitter(t)
	req, err := NewRequest(context.Background(), nil, map[string]interface{}{
		"beep": true,
	}, nil, nil, cmd)
	if err == nil {
		t.Error("Should have failed (incorrect type)")
	}

	re = newTestEmitter(t)
	req, err = NewRequest(context.Background(), nil, map[string]interface{}{
		"beep": 5,
	}, nil, nil, cmd)
	if err != nil {
		t.Error(err, "Should have passed")
	}
	cmd.Call(req, re, nil)

	re = newTestEmitter(t)
	req, err = NewRequest(context.Background(), nil, map[string]interface{}{
		"beep": 5,
		"boop": "test",
	}, nil, nil, cmd)
	if err != nil {
		t.Error("Should have passed")
	}

	cmd.Call(req, re, nil)

	re = newTestEmitter(t)
	req, err = NewRequest(context.Background(), nil, map[string]interface{}{
		"b": 5,
		"B": "test",
	}, nil, nil, cmd)
	if err != nil {
		t.Error("Should have passed")
	}

	cmd.Call(req, re, nil)

	re = newTestEmitter(t)
	req, err = NewRequest(context.Background(), nil, map[string]interface{}{
		"foo": 5,
	}, nil, nil, cmd)
	if err != nil {
		t.Error("Should have passed")
	}

	cmd.Call(req, re, nil)

	re = newTestEmitter(t)
	req, err = NewRequest(context.Background(), nil, map[string]interface{}{
		EncLong: "json",
	}, nil, nil, cmd)
	if err != nil {
		t.Error("Should have passed")
	}

	cmd.Call(req, re, nil)

	re = newTestEmitter(t)
	req, err = NewRequest(context.Background(), nil, map[string]interface{}{
		"b": "100",
	}, nil, nil, cmd)
	if err != nil {
		t.Error("Should have passed")
	}

	cmd.Call(req, re, nil)

	re = newTestEmitter(t)
	req, err = NewRequest(context.Background(), nil, map[string]interface{}{
		"b": ":)",
	}, nil, nil, cmd)
	if err == nil {
		t.Error("Should have failed (string value not convertible to int)")
	}
}

func TestRegistration(t *testing.T) {
	cmdA := &Command{
		Options: []cmdkit.Option{
			cmdkit.IntOption("beep", "number of beeps"),
		},
		Run: noop,
	}

	cmdB := &Command{
		Options: []cmdkit.Option{
			cmdkit.IntOption("beep", "number of beeps"),
		},
		Run: noop,
		Subcommands: map[string]*Command{
			"a": cmdA,
		},
	}

	path := []string{"a"}
	_, err := cmdB.GetOptions(path)
	if err == nil {
		t.Error("Should have failed (option name collision)")
	}
}

func TestResolving(t *testing.T) {
	cmdC := &Command{}
	cmdB := &Command{
		Subcommands: map[string]*Command{
			"c": cmdC,
		},
	}
	cmdB2 := &Command{}
	cmdA := &Command{
		Subcommands: map[string]*Command{
			"b": cmdB,
			"B": cmdB2,
		},
	}
	cmd := &Command{
		Subcommands: map[string]*Command{
			"a": cmdA,
		},
	}

	cmds, err := cmd.Resolve([]string{"a", "b", "c"})
	if err != nil {
		t.Error(err)
	}
	if len(cmds) != 4 || cmds[0] != cmd || cmds[1] != cmdA || cmds[2] != cmdB || cmds[3] != cmdC {
		t.Error("Returned command path is different than expected", cmds)
	}
}

func TestWalking(t *testing.T) {
	cmdA := &Command{
		Subcommands: map[string]*Command{
			"b": &Command{},
			"B": &Command{},
		},
	}
	i := 0
	cmdA.Walk(func(c *Command) {
		i = i + 1
	})
	if i != 3 {
		t.Error("Command tree walk didn't work, expected 3 got:", i)
	}
}

func TestHelpProcessing(t *testing.T) {
	cmdB := &Command{
		Helptext: cmdkit.HelpText{
			ShortDescription: "This is other short",
		},
	}
	cmdA := &Command{
		Helptext: cmdkit.HelpText{
			ShortDescription: "This is short",
		},
		Subcommands: map[string]*Command{
			"a": cmdB,
		},
	}
	cmdA.ProcessHelp()
	if len(cmdA.Helptext.LongDescription) == 0 {
		t.Error("LongDescription was not set on basis of ShortDescription")
	}
	if len(cmdB.Helptext.LongDescription) == 0 {
		t.Error("LongDescription was not set on basis of ShortDescription")
	}
}

type postRunTestCase struct {
	length      uint64
	err         *cmdkit.Error
	emit        []interface{}
	postRun     func(*Request, ResponseEmitter) ResponseEmitter
	next        []interface{}
	finalLength uint64
}

// TestPostRun tests whether commands with PostRun return the intended result
func TestPostRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var testcases = []postRunTestCase{
		postRunTestCase{
			length:      3,
			err:         nil,
			emit:        []interface{}{7},
			finalLength: 4,
			next:        []interface{}{14},
			postRun: func(req *Request, re ResponseEmitter) ResponseEmitter {
				re_, res := NewChanResponsePair(req)

				go func() {
					defer re.Close()
					l := res.Length()
					re.SetLength(l + 1)

					for {
						v, err := res.Next()
						if err == io.EOF {
							return
						}
						if err != nil {
							re.SetError(err, cmdkit.ErrNormal)
							t.Fatal(err)
							return
						}

						i := v.(int)

						err = re.Emit(2 * i)
						if err != nil {
							re.SetError(err, cmdkit.ErrNormal)
							return
						}
					}
				}()

				return re_
			},
		},
	}

	for _, tc := range testcases {
		cmd := &Command{
			Run: func(req *Request, re ResponseEmitter, env Environment) {
				re.SetLength(tc.length)

				for _, v := range tc.emit {
					err := re.Emit(v)
					if err != nil {
						t.Fatal(err)
					}
				}
				err := re.Close()
				if err != nil {
					t.Fatal(err)
				}
			},
			PostRun: PostRunMap{
				CLI: tc.postRun,
			},
		}

		req, err := NewRequest(ctx, nil, map[string]interface{}{
			EncLong: CLI,
		}, nil, nil, cmd)
		if err != nil {
			t.Fatal(err)
		}

		opts := req.Options
		if opts == nil {
			t.Fatal("req.Options() is nil")
		}

		encTypeIface := opts[EncLong]
		if encTypeIface == nil {
			t.Fatal("req.Options()[EncLong] is nil")
		}

		encType := EncodingType(encTypeIface.(string))
		if encType == "" {
			t.Fatal("no encoding type")
		}

		if encType != CLI {
			t.Fatal("wrong encoding type")
		}

		re, res := NewChanResponsePair(req)
		re = cmd.PostRun[PostRunType(encType)](req, re)

		cmd.Call(req, re, nil)

		l := res.Length()
		if l != tc.finalLength {
			t.Fatal("wrong final length")
		}

		for _, x := range tc.next {
			ch := make(chan interface{})

			go func() {
				v, err := res.Next()
				if err != nil {
					close(ch)
					t.Fatal(err)
				}

				ch <- v
			}()

			select {
			case v, ok := <-ch:
				if !ok {
					t.Fatal("error checking all next values - channel closed")
				}
				if x != v {
					t.Fatalf("final check of emitted values failed. got %v but expected %v", v, x)
				}
			case <-time.After(50 * time.Millisecond):
				t.Fatal("too few values in next")
			}
		}

		_, err = res.Next()
		if err != io.EOF {
			t.Fatal("expected EOF, got", err)
		}
	}
}

func TestCancel(t *testing.T) {
	wait := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())

	req, err := NewRequest(ctx, nil, nil, nil, nil, &Command{})
	if err != nil {
		t.Fatal(err)
	}

	re, res := NewChanResponsePair(req)

	go func() {
		err := re.Emit("abc")
		if err != context.Canceled {
			t.Errorf("re:  expected context.Canceled but got %v", err)
		} else {
			t.Log("re.Emit err:", err)
		}
		re.Close()
		close(wait)
	}()

	cancel()

	_, err = res.Next()
	if err != context.Canceled {
		t.Errorf("res: expected context.Canceled but got %v", err)
	} else {
		t.Log("res.Emit err:", err)
	}
	<-wait
}
