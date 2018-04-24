package dht

import (
	"bytes"
	"testing"

	recpb "gx/ipfs/QmUpttFinNDmNPgFwKN8sZK6BUtBmA68Y4KdSBDXa8t9sJ/go-libp2p-record/pb"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
)

func TestCleanRecordSigned(t *testing.T) {
	actual := new(recpb.Record)
	actual.TimeReceived = proto.String("time")
	actual.XXX_unrecognized = []byte("extra data")
	actual.Signature = []byte("signature")
	actual.Author = proto.String("author")
	actual.Value = []byte("value")
	actual.Key = proto.String("key")

	cleanRecord(actual)
	actualBytes, err := proto.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}

	expected := new(recpb.Record)
	expected.Signature = []byte("signature")
	expected.Author = proto.String("author")
	expected.Value = []byte("value")
	expected.Key = proto.String("key")
	expectedBytes, err := proto.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(actualBytes, expectedBytes) {
		t.Error("failed to clean record")
	}
}

func TestCleanRecord(t *testing.T) {
	actual := new(recpb.Record)
	actual.TimeReceived = proto.String("time")
	actual.XXX_unrecognized = []byte("extra data")
	actual.Key = proto.String("key")
	actual.Value = []byte("value")

	cleanRecord(actual)
	actualBytes, err := proto.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}

	expected := new(recpb.Record)
	expected.Key = proto.String("key")
	expected.Value = []byte("value")
	expectedBytes, err := proto.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(actualBytes, expectedBytes) {
		t.Error("failed to clean record")
	}
}
