package data

import (
	"testing"
)

func TestToD3Json(t *testing.T) {
	n1 := NewNode("A")
	n1.AddConn("A", "B")
	n1.SetBroadcast("strm1")
	n1.SetStream("strm1", 15, 100)

	n2 := NewNode("B")
	n2.AddConn("B", "A")
	n2.AddConn("B", "C")
	n2.SetRelay("strm1", "A")

	n3 := NewNode("C")
	n3.AddConn("C", "B")
	n3.SetSub("strm1")
	n3.SetStream("strm1", 10, 100)

	n := NewNetwork()
	n.SetNode(n1)
	n.SetNode(n2)
	n.SetNode(n3)

	json := n.ToD3Json().(map[string]interface{})
	if _, nok := json["nodes"]; !nok {
		t.Errorf("Wrong json: %v", json)
	}

	if _, lok := json["links"]; !lok {
		t.Errorf("Wrong json: %v", json)
	}

	if _, sok := json["streams"]; !sok {
		t.Errorf("Wrong json: %v", json)
	}
}
