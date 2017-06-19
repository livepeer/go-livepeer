package net

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
)

type Opcode uint8

const (
	StreamDataID Opcode = iota
	FinishStreamID
	SubReqID
	CancelSubID
)

type Msg struct {
	Op   Opcode
	Data interface{}
}

type msgAux struct {
	Op   Opcode
	Data []byte
}

type SubReqMsg struct {
	StrmID      string
	SubNodeID   string
	SubNodeAddr string
}

type StreamDataMsg struct {
	SeqNo  uint64
	StrmID string
	Data   []byte
}

func (m Msg) MarshalJSON() ([]byte, error) {
	// Encode m.Data into a gob
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	switch m.Data.(type) {
	case SubReqMsg:
		gob.Register(SubReqMsg{})
		err := enc.Encode(m.Data.(SubReqMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal Handshake: %v", err)
		}
	case StreamDataMsg:
		gob.Register(StreamDataMsg{})
		err := enc.Encode(m.Data.(StreamDataMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal StreamDataMsg: %v", err)
		}
	default:
		return nil, errors.New("failed to marshal message data")
	}

	// build an aux and marshal using built-in json
	aux := msgAux{Op: m.Op, Data: b.Bytes()}
	return json.Marshal(aux)
}

func (m *Msg) UnmarshalJSON(b []byte) error {
	// Use builtin json to unmarshall into aux
	var aux msgAux
	json.Unmarshal(b, &aux)

	// The Op field in aux is already what we want for m.Op
	m.Op = aux.Op

	// decode the gob in aux.Data and put it in m.Data
	dec := gob.NewDecoder(bytes.NewBuffer(aux.Data))
	switch aux.Op {
	case SubReqID:
		var sr SubReqMsg
		err := dec.Decode(&sr)
		if err != nil {
			return errors.New("failed to decode handshake")
		}
		m.Data = sr
	case StreamDataID:
		var sd StreamDataMsg
		err := dec.Decode(&sd)
		if err != nil {
			return errors.New("failed to decode StreamDataMsg")
		}
		m.Data = sd
	default:
		return errors.New("failed to decode message data")
	}

	return nil
}
