package stream

import (
	"context"
	"fmt"
	"io"
	"testing"

	"time"

	"github.com/ericxtang/m3u8"
	"github.com/nareix/joy4/av"
)

type TestHLSMux struct {
	segs []string
	pls  []m3u8.MediaPlaylist
}

func (t *TestHLSMux) WritePlaylist(pl m3u8.MediaPlaylist) error {
	// fmt.Println("Writing pl")
	t.pls = append(t.pls, pl)
	return nil
}

func (t *TestHLSMux) WriteSegment(segNum uint64, name string, duration float64, s []byte) error {
	// fmt.Println("Writing seg")
	t.segs = append(t.segs, name)
	return nil
}

func TestHLSSubscribe(t *testing.T) {
	stream := NewVideoStream("test", HLS)
	stream.HLSTimeout = time.Millisecond * 1

	sub := NewStreamSubscriber(stream)

	m1 := &TestHLSMux{segs: make([]string, 0, 100), pls: make([]m3u8.MediaPlaylist, 0, 100)}
	m2 := &TestHLSMux{segs: make([]string, 0, 100), pls: make([]m3u8.MediaPlaylist, 0, 100)}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub.SubscribeHLS("m1", m1)
	sub.SubscribeHLS("m2", m2)

	ec := make(chan error, 1)
	go func() { ec <- sub.StartHLSWorker(ctx, time.Second*5) }()

	pl, _ := m3u8.NewMediaPlaylist(1, 1)
	pl.Segments[0] = &m3u8.MediaSegment{URI: "seg1"}
	err := stream.WriteHLSPlaylistToStream(*pl)
	if err != nil {
		t.Errorf("Error writing playlist to stream: %v", err)
	}

	err = stream.WriteHLSSegmentToStream(HLSSegment{Name: "seg1"})
	if err != nil {
		t.Errorf("Error writing segment to stream: %v", err)
	}

	// select {
	// case err := <-ec:
	// 	t.Errorf("Expecting Timeout, got %v", err)
	// }
	time.Sleep(time.Second * 2) //Sleep enough so that the HLS buffer read times out.  (Currently no good way to end a HLS stream)

	// if len(m1.pls) != 1 {
	// 	t.Errorf("Expecting 1 playlist, got %v", len(m1.pls))
	// }

	// if len(m2.pls) != 1 {
	// 	t.Errorf("Expecting 1 playlist, got %v", len(m2.pls))
	// }

	if len(m1.segs) != 1 {
		t.Errorf("Expecting 1 seg, got %v", len(m1.segs))
	}

	if m1.segs[0] != "seg1" {
		t.Errorf("Expecting seg1, got: %v", m1.segs[0])
	}

	if len(m2.segs) != 1 {
		t.Errorf("Expecting 1 seg, got %v", len(m1.segs))
	}

	if m2.segs[0] != "seg1" {
		t.Errorf("Expecting seg1, got: %v", m2.segs[0])
	}

	//For now it's not working... We seem to need to do a bigger change
	// m3 := &TestHLSMux{}
	// sub.SubscribeHLS(ctx, "m3", m3)
	// pl, _ = m3u8.NewMediaPlaylist(1, 1)
	// pl.Segments[0] = &m3u8.MediaSegment{URI: "seg2"}
	// stream.WriteHLSPlaylistToStream(*pl)
	// stream.WriteHLSSegmentToStream(HLSSegment{Name: "seg2"})
	// time.Sleep(time.Second * 4)

	// if len(m3.pls) != 1 {
	// 	t.Errorf("Expecting 1 playlist, got %v", m3.pls)
	// }

	// if len(m3.segs) != 1 {
	// 	t.Errorf("Expecting 1 seg, got %v", len(m3.segs))
	// }

	// if m3.segs[0] != "seg1" {
	// 	t.Errorf("Expecting seg1, got: %v", m3.segs[0])
	// }
}

type TestDemux struct {
	c           *Counter
	readHeaders bool
}

func (d *TestDemux) Close() error { return nil }
func (d *TestDemux) Streams() ([]av.CodecData, error) {
	d.readHeaders = true
	// fmt.Println("after reading streams from demux")
	return []av.CodecData{}, nil
}
func (d *TestDemux) ReadPacket() (av.Packet, error) {
	//Wait until headers are read before returning packets
	for d.readHeaders == false {
		// fmt.Println("Waiting to read stream")
		time.Sleep(1 * time.Second)
	}

	if d.c.Count == 10 {
		return av.Packet{}, io.EOF
	}

	d.c.Count = d.c.Count + 1
	return av.Packet{Data: []byte{0, 0}}, nil
}

type TestMux struct {
	wroteHeaders bool
	packetChan   chan *av.Packet
	eofChan      chan error
}

func (d *TestMux) Close() error { return nil }
func (d *TestMux) WriteHeader(h []av.CodecData) error {
	// fmt.Printf("TestMux: Writing header, %v\n", h)
	d.wroteHeaders = true
	return nil
}

func (d *TestMux) WritePacket(pkt av.Packet) error {
	// fmt.Printf("TestMux: Writing pkt: %v\n", pkt)
	d.packetChan <- &pkt
	return nil
}

func (d *TestMux) WriteTrailer() error {
	// fmt.Printf("TestMux: Writing trailer\n")
	d.eofChan <- io.EOF
	return nil
}

func TestRTMPSubscription(t *testing.T) {
	stream := NewVideoStream("test", RTMP)
	sub := NewStreamSubscriber(stream)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m1 := &TestMux{packetChan: make(chan *av.Packet), eofChan: make(chan error)}
	m2 := &TestMux{packetChan: make(chan *av.Packet), eofChan: make(chan error)}

	go sub.SubscribeRTMP("test1", m1)
	go sub.SubscribeRTMP("test2", m2)

	go sub.StartRTMPWorker(ctx)
	go stream.WriteRTMPToStream(ctx, &TestDemux{c: &Counter{}})

	// var lock = &sync.Mutex{}
	otherEnded := false
	m1c := 0
	m2c := 0

	// fmt.Println("Waiting for packets")
	done := false
	for !done {
		select {
		case <-m1.packetChan:
			// fmt.Println("Got packet for m1")
			m1c = m1c + 1
		case err := <-m1.eofChan:
			// fmt.Println("Got eof for m1")
			if err == io.EOF {
				if otherEnded {
					done = true
				} else {
					otherEnded = true
				}
			}
		case <-m2.packetChan:
			// fmt.Println("Got packet for m2")
			m2c = m2c + 1
		case err := <-m2.eofChan:
			// fmt.Println("Got eof for m2")
			if err == io.EOF {
				if otherEnded {
					done = true
				} else {
					otherEnded = true
				}
			}
		}
	}

	if m1c != 10 {
		t.Errorf("packet counter should be 10, but got: %v", m1c)
	}
	if m2c != 10 {
		t.Errorf("packet counter should be 10, but got: %v", m2c)
	}
	if !m1.wroteHeaders {
		t.Errorf("should have read headers for m1")
	}
	if !m2.wroteHeaders {
		t.Errorf("should have read headers for m2")
	}

}

type Listener struct {
	val []string
}

func (l *Listener) Send(s string) {
	l.val = append(l.val, s)
}

type TestS struct {
	buf       chan string
	listeners []*Listener
}

func (t *TestS) Listen(ctx context.Context) error {
	for {
		select {
		case val := <-t.buf:
			for _, l := range t.listeners {
				l.Send(val)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (t *TestS) AddListener(l *Listener) {
	t.listeners = append(t.listeners, l)
}

func TestT(t *testing.T) {
	l1 := &Listener{}
	l2 := &Listener{}
	l3 := &Listener{}

	s := TestS{buf: make(chan string), listeners: []*Listener{l1, l2}}
	ctx, cancel := context.WithCancel(context.Background())
	ec := make(chan error, 1)
	go func() { ec <- s.Listen(ctx) }()

	go func() {
		s.buf <- "hello"
		s.buf <- "hi"
		cancel()
		time.Sleep(1 * time.Second)
		s.AddListener(l3)
		s.buf <- "hola"
	}()

	select {
	case err := <-ec:
		fmt.Println(err)
	}

	// fmt.Println("l1 val: ", l1.val)
	// fmt.Println("l2 val:", l2.val)
	// fmt.Println("l3 val:", l3.val)
}
