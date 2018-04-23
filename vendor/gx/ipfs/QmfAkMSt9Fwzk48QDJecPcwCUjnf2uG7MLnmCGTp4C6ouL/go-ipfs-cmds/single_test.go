package cmds

import (
	"context"
	"io"
	"testing"
)

func TestSingle_1(t *testing.T) {
	req, err := NewRequest(context.Background(), nil, nil, nil, nil, &Command{})
	if err != nil {
		t.Fatal(err)
	}

	re, res := NewChanResponsePair(req)

	go func() {
		if err := EmitOnce(re, "test"); err != nil {
			t.Fatal(err)
		}
	}()

	v, err := res.Next()
	if err != nil {
		t.Fatal(err)
	}

	if str, ok := v.(string); !ok || str != "test" {
		t.Fatalf("expected %#v, got %#v", "foo", str)
	}

	if _, err = res.Next(); err != io.EOF {
		t.Fatalf("expected %#v, got %#v", io.EOF, err)
	}
}
