package record

import (
	"testing"
)

func TestBestRecord(t *testing.T) {
	sel := Selector{}
	sel["pk"] = PublicKeySelector

	i, err := sel.BestRecord("/pk/thing", [][]byte{[]byte("first"), []byte("second")})
	if err != nil {
		t.Fatal(err)
	}
	if i != 0 {
		t.Error("expected to select first record")
	}

	_, err = sel.BestRecord("/pk/thing", nil)
	if err == nil {
		t.Fatal("expected error for no records")
	}

	_, err = sel.BestRecord("/other/thing", [][]byte{[]byte("first"), []byte("second")})
	if err == nil {
		t.Fatal("expected error for unregistered ns")
	}

	_, err = sel.BestRecord("bad", [][]byte{[]byte("first"), []byte("second")})
	if err == nil {
		t.Fatal("expected error for bad key")
	}
}
