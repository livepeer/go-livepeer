package cmds

import (
	"bytes"
	"context"
	"io"
	"testing"
)

var root = &Command{
	Subcommands: map[string]*Command{
		"test": &Command{
			Run: func(req *Request, re ResponseEmitter, env Environment) {
				re.Emit(env)
			},
		},
	},
}

type wc struct {
	io.Writer
	io.Closer
}

type env int

func (e *env) Context() context.Context {
	return context.Background()
}

func TestExecutor(t *testing.T) {
	env := env(42)
	req, err := NewRequest(context.Background(), []string{"test"}, nil, nil, nil, root)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	re := NewWriterResponseEmitter(wc{&buf, nopCloser{}}, req, Encoders[Text])

	x := NewExecutor(root)
	x.Execute(req, re, &env)

	if out := buf.String(); out != "42\n" {
		t.Errorf("expected output \"42\" but got %q", out)
	}
}
