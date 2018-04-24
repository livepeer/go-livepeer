package basicnet

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"

	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
)

type Opcode uint8

const (
	StreamDataID Opcode = iota
	FinishStreamID
	SubReqID
	CancelSubID
	TranscodeResponseID
	TranscodeSubID
	GetMasterPlaylistReqID
	MasterPlaylistDataID
	NodeStatusReqID
	NodeStatusDataID
	PingID
	PongID
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

type TranscodeSubMsg struct {
	MultiAddrs []ma.Multiaddr
	NodeID     peer.ID
	StrmID     string
	Sig        []byte
}

// struct that can be handled by gob; multiaddr can't
type TranscodeSubMsg_b struct {
	MultiAddrs [][]byte
	NodeID     []byte
	StrmID     string
	Sig        []byte
}

func (ts TranscodeSubMsg) GobEncode() ([]byte, error) {
	b := make([][]byte, len(ts.MultiAddrs))
	for i, v := range ts.MultiAddrs {
		b[i] = v.Bytes()
	}
	n := []byte(ts.NodeID)
	tsb := TranscodeSubMsg_b{MultiAddrs: b, StrmID: ts.StrmID, NodeID: n, Sig: ts.Sig}
	gob.Register(TranscodeSubMsg_b{})

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(tsb)
	return buf.Bytes(), err
}

func (ts *TranscodeSubMsg) GobDecode(data []byte) error {
	// deserialize into a struct we can handle natively
	var tsb TranscodeSubMsg_b
	dec := gob.NewDecoder(bytes.NewBuffer(data))
	err := dec.Decode(&tsb)
	if err != nil {
		return err
	}

	// convert bytes into multiaddrs
	maddrs := make([]ma.Multiaddr, len(tsb.MultiAddrs))
	for i, v := range tsb.MultiAddrs {
		m, err := ma.NewMultiaddrBytes(v)
		if err != nil {
			return err
		}
		maddrs[i] = m
	}
	var nid peer.ID
	if len(tsb.NodeID) > 0 {
		nid, err = peer.IDFromBytes(tsb.NodeID)
	} // not sure what 'nid' resolves to in the uninitialized case
	if err != nil {
		return err
	}

	// now populate our struct
	ts.MultiAddrs = maddrs
	ts.NodeID = nid
	ts.StrmID = tsb.StrmID
	ts.Sig = tsb.Sig
	return nil
}

func (ts TranscodeSubMsg) BytesForSigning() []byte {
	b := make([][]byte, len(ts.MultiAddrs)+2)
	for i, v := range ts.MultiAddrs {
		b[i] = v.Bytes()
	}
	b[len(ts.MultiAddrs)+0] = []byte(ts.StrmID)
	b[len(ts.MultiAddrs)+1] = []byte(ts.NodeID)
	return bytes.Join(b, []byte(""))
}

type GetMasterPlaylistReqMsg struct {
	ManifestID string
}

type MasterPlaylistDataMsg struct {
	ManifestID string
	MPL        string
	NotFound   bool
}

type NodeStatusReqMsg struct {
	NodeID string
}

type NodeStatusDataMsg struct {
	NodeID   string
	Data     []byte
	NotFound bool
}

type PingDataMsg string
type PongDataMsg string

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
	case TranscodeSubMsg:
		gob.Register(TranscodeSubMsg{})
		err := enc.Encode(m.Data.(TranscodeSubMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal TranscodeSubMsg: %v", err)
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
	case NodeStatusReqMsg:
		gob.Register(NodeStatusReqMsg{})
		err := enc.Encode(m.Data.(NodeStatusReqMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal NodeStatusReqMsg: %v", err)
		}
	case NodeStatusDataMsg:
		gob.Register(NodeStatusDataMsg{})
		err := enc.Encode(m.Data.(NodeStatusDataMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal NodeStatusDataMsg: %v", err)
		}
	case PingDataMsg:
		err := enc.Encode(m.Data.(PingDataMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal PingDataMsg: %v", err)
		}
	case PongDataMsg:
		err := enc.Encode(m.Data.(PongDataMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal PongDataMsg: %v", err)
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
	case TranscodeSubID:
		var ts TranscodeSubMsg
		err := dec.Decode(&ts)
		if err != nil {
			return errors.New("Failed to decode TranscodeSubMsg")
		}
		m.Data = ts
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
	case NodeStatusReqID:
		var ns NodeStatusReqMsg
		err := dec.Decode(&ns)
		if err != nil {
			return errors.New("failed to decode NodeStatusReqMsg")
		}
		m.Data = ns
	case NodeStatusDataID:
		var nsd NodeStatusDataMsg
		err := dec.Decode(&nsd)
		if err != nil {
			return errors.New("failed to decode NodeStatusDataMsg")
		}
		m.Data = nsd
	case PingID:
		var sd PingDataMsg
		err := dec.Decode(&sd)
		if err != nil {
			return errors.New("failed to decode PingDataMsg")
		}
		m.Data = sd
	case PongID:
		var sd PongDataMsg
		err := dec.Decode(&sd)
		if err != nil {
			return errors.New("failed to decode PongDataMsg")
		}
		m.Data = sd
	default:
		return errors.New("failed to decode message data")
	}

	return nil
}
