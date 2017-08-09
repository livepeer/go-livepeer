package basicnet

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
	TranscodeResponseID
	GetMasterPlaylistReqID
	MasterPlaylistDataID
	SimpleString
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
	StrmID string
	// SubNodeID string
	//TODO: Add Signature
}

type CancelSubMsg struct {
	StrmID string
}

type FinishStreamMsg struct {
	StrmID string
}

type StreamDataMsg struct {
	SeqNo  uint64
	StrmID string
	Data   []byte
}

type TranscodeResponseMsg struct {
	//map of streamid -> video description
	StrmID string
	Result map[string]string
}

type GetMasterPlaylistReqMsg struct {
	StrmID string
}

type MasterPlaylistDataMsg struct {
	StrmID   string
	MPL      string
	NotFound bool
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
	case CancelSubMsg:
		gob.Register(CancelSubMsg{})
		err := enc.Encode(m.Data.(CancelSubMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal CancelSubMsg: %v", err)
		}
	case StreamDataMsg:
		gob.Register(StreamDataMsg{})
		err := enc.Encode(m.Data.(StreamDataMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal StreamDataMsg: %v", err)
		}
	case FinishStreamMsg:
		gob.Register(FinishStreamMsg{})
		err := enc.Encode(m.Data.(FinishStreamMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal FinishStreamMsg: %v", err)
		}
	case TranscodeResponseMsg:
		gob.Register(TranscodeResponseMsg{})
		err := enc.Encode(m.Data.(TranscodeResponseMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal TranscodeResponseMsg: %v", err)
		}
	case MasterPlaylistDataMsg:
		gob.Register(MasterPlaylistDataMsg{})
		err := enc.Encode(m.Data.(MasterPlaylistDataMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal MasterPlaylistDataMsg: %v", err)
		}
	case GetMasterPlaylistReqMsg:
		gob.Register(GetMasterPlaylistReqMsg{})
		err := enc.Encode(m.Data.(GetMasterPlaylistReqMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal GetMasterPlaylistReqMsg: %v", err)
		}
	case string:
		err := enc.Encode(m.Data)
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal string: %v", err)
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
	case CancelSubID:
		var cs CancelSubMsg
		err := dec.Decode(&cs)
		if err != nil {
			return errors.New("failed to decode CancelSubMsg")
		}
		m.Data = cs
	case StreamDataID:
		var sd StreamDataMsg
		err := dec.Decode(&sd)
		if err != nil {
			return errors.New("failed to decode StreamDataMsg")
		}
		m.Data = sd
	case FinishStreamID:
		var fs FinishStreamMsg
		err := dec.Decode(&fs)
		if err != nil {
			return errors.New("failed to decode FinishStreamMsg")
		}
		m.Data = fs
	case TranscodeResponseID:
		var tr TranscodeResponseMsg
		err := dec.Decode(&tr)
		if err != nil {
			return errors.New("failed to decode TranscodeResponseMsg")
		}
		m.Data = tr
	case MasterPlaylistDataID:
		var mpld MasterPlaylistDataMsg
		err := dec.Decode(&mpld)
		if err != nil {
			return errors.New("failed to decode MasterPlaylistDataMsg")
		}
		m.Data = mpld
	case GetMasterPlaylistReqID:
		var mplr GetMasterPlaylistReqMsg
		err := dec.Decode(&mplr)
		if err != nil {
			return errors.New("failed to decode GetMasterPlaylistReqMsg")
		}
		m.Data = mplr
	case SimpleString:
		var str string
		err := dec.Decode(&str)
		if err != nil {
			return errors.New("Failed to decode string msg")
		}
		m.Data = str

	default:
		return errors.New("failed to decode message data")
	}

	return nil
}
