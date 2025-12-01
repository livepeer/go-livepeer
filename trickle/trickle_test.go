package trickle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTrickle_Close(t *testing.T) {
	require := require.New(t)
	mux := http.NewServeMux()
	server := ConfigureServer(TrickleServerConfig{
		Mux: mux,
	})
	stop := server.Start()
	ts := httptest.NewServer(mux)
	//defer goleak.VerifyNone(t)
	defer ts.Close()
	defer stop()

	channelURL := ts.URL + "/testest"
	pub, err := NewTricklePublisher(channelURL)
	require.Nil(err)
	defer pub.Close()
	require.Error(StreamNotFoundErr, pub.Write(bytes.NewReader([]byte("first post"))))

	sub, err := NewTrickleSubscriber(subConfig(t, channelURL))
	require.Nil(err)
	sub.SetSeq(0)

	// this is lame but there is a little race condition under the hood
	// between lp.CreateChannel and the "second post" write since the
	// pre-connect in the second post does not always latch on in time
	time.Sleep(1 * time.Millisecond)

	// no autocreate requires creating the channel locally on the server
	lp := NewLocalPublisher(server, "testest", "text/plain")
	lp.CreateChannel()

	// pub was created before the channel so this should still fail
	require.Error(StreamNotFoundErr, pub.Write(bytes.NewReader([]byte("second post"))))

	// now recreate pub, should be ok
	pub, err = NewTricklePublisher(channelURL)
	require.Nil(err)
	defer pub.Close()

	// write two segments
	segs := []string{"first", "second"}
	for _, s := range segs {
		require.Nil(pub.Write(bytes.NewReader([]byte(s))), "failed writing "+s)
	}

	// now read two segments
	sub.SetSeq(0)
	for seq, s := range segs {
		resp, err := sub.Read()
		require.Nil(err, "sub.Read")
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.Nil(err, fmt.Sprintf("reading body seq=%d", seq))
		got := string(data)
		require.Equal(s, got, fmt.Sprintf("segment read seq=%d", seq))
		require.Equal(fmt.Sprintf("%d", seq), resp.Header.Get("Lp-Trickle-Seq"), "Lp-Trickle-Seq")
		require.Equal("2", resp.Header.Get("Lp-Trickle-Latest"), "Lp-Trickle-Latest")
	}

	// close the stream
	require.Nil(pub.Close())

	// requesting past the last segment should return EOS
	_, err = sub.Read()
	require.Error(err, EOS)

	// and writing past last segment should also return EOS
	require.Error(EOS, pub.Write(bytes.NewReader([]byte("invalid"))))

	// Spinning up a second subscriber should return 404
	sub2, err := NewTrickleSubscriber(subConfig(t, channelURL))
	require.Nil(err)
	_, err = sub2.Read()
	require.Error(StreamNotFoundErr, err)

	// Spinning up a second publisher should return 404
	pub2, err := NewTricklePublisher(channelURL)
	require.Nil(err)
	defer pub2.Close()
	require.Error(StreamNotFoundErr, pub2.Write(bytes.NewReader([]byte("bad post"))))
}

func TestTrickle_SetSeq(t *testing.T) {
	require, channelURL := makeServer(t)

	pub, err := NewTricklePublisher(channelURL)
	require.Nil(err)
	defer pub.Close()
	sub, err := NewTrickleSubscriber(subConfig(t, channelURL))
	require.Nil(err)

	// give sub preconnect time to latch on

	segs := []string{"first", "second", "third", "fourth"}
	for _, s := range segs {
		require.Nil(pub.Write(bytes.NewReader([]byte(s))), "failed writing "+s)
	}

	for i := range segs {
		sub.SetSeq(i)
		for j := i; j < len(segs); j++ {
			s := segs[j]
			resp, err := sub.Read()
			require.Nil(err)
			buf, err := io.ReadAll(resp.Body)
			require.Nil(err)
			require.Equal(s, string(buf))
		}
	}

	// now do it again, backwards
	for i := range segs {
		j := len(segs) - i - 1
		sub.SetSeq(j)
		s := segs[j]
		resp, err := sub.Read()
		require.Nil(err)
		buf, err := io.ReadAll(resp.Body)
		require.Nil(err)
		require.Equal(s, string(buf))
	}
}

func TestTrickle_Reset(t *testing.T) {
	// codifying some awful behavior for now
	// concurrent writes will stomp over one another
	// and subscriber has no way to distinguish
	require := require.New(t)
	mux := http.NewServeMux()
	server := ConfigureServer(TrickleServerConfig{
		Mux:        mux,
		Autocreate: true,
	})
	stop := server.Start()
	ts := httptest.NewServer(mux)
	//defer goleak.VerifyNone(t)
	defer ts.Close()
	defer stop()

	channelURL := ts.URL + "/testest"

	pub, err := NewTricklePublisher(channelURL)
	require.Nil(err)
	defer pub.Close()

	sub, err := NewTrickleSubscriber(subConfig(t, channelURL))
	require.Nil(err)
	wg := &sync.WaitGroup{}

	// give preconnects time to latch on and autocreate the channel
	time.Sleep(5 * time.Millisecond)

	respCh := make(chan *http.Response)
	buf := make([]byte, 100)
	go func() {
		sub.SetSeq(0)
		resp, err := sub.Read()
		require.Nil(err)
		n, err := io.ReadFull(resp.Body, buf[0:5])
		require.Nil(err)
		require.Equal(5, int(n))
		require.Equal("Hello", string(buf[0:5]))
		respCh <- resp
	}()

	t1, err := pub.Next()
	r1, w1 := io.Pipe()
	wg.Add(1)
	go func() {
		defer wg.Done()
		n, err := t1.Write(r1)
		require.Nil(err)
		require.Equal(5, int(n))
	}()

	w1.Write([]byte("Hello"))

	resp := <-respCh
	defer resp.Body.Close()

	readCh := make(chan bool)
	go func() {
		defer close(readCh)
		n, err := io.ReadFull(resp.Body, buf[5:11])
		require.Nil(err)
		require.Equal(6, int(n))
		require.Equal("yWorld", string(buf[5:11]))
	}()

	// give the above goroutine time to spin up
	time.Sleep(5 * time.Millisecond)

	// write again!
	r2, w2 := io.Pipe()
	wg.Add(1)
	go func() {
		defer wg.Done()
		n, err := t1.Write(r2)
		require.Nil(err)
		require.Equal(11, int(n))
	}()
	w2.Write([]byte("GoodbyWorld"))

	<-readCh

	w1.Close()
	w2.Close()

	// this is horrible because existing read heads
	// on the server is not reset during segment re-writes
	// but thats the behavior right now so codify it here
	require.Equal("HelloyWorld", string(buf[0:11]))

	// now check a fresh read
	sub.SetSeq(0)
	resp2, err := sub.Read()
	require.Nil(err)
	data, err := io.ReadAll(resp2.Body)
	defer resp2.Body.Close()
	require.Equal("GoodbyWorld", string(data))

	wg.Wait()
}

func TestTrickle_IdleSweep(t *testing.T) {
	require := require.New(t)
	mux := http.NewServeMux()
	server := ConfigureServer(TrickleServerConfig{
		Mux:           mux,
		IdleTimeout:   1 * time.Millisecond,
		SweepInterval: 10 * time.Millisecond,
	})
	stop := server.Start()
	ts := httptest.NewServer(mux)
	//defer goleak.VerifyNone(t)
	defer ts.Close()
	defer stop()

	channelURL := ts.URL + "/testest"
	lp := NewLocalPublisher(server, channelURL, "text/plain")
	lp.CreateChannel()

	sub, err := NewTrickleSubscriber(subConfig(t, channelURL))
	require.Nil(err)
	_, err = sub.Read()
	require.ErrorIs(err, StreamNotFoundErr)
}

func TestTrickle_CancelSub(t *testing.T) {
	require, url := makeServer(t)
	ctx, cancel := context.WithCancelCause(t.Context())
	sub, err := NewTrickleSubscriber(TrickleSubscriberConfig{
		URL: url,
		Ctx: ctx,
	})
	require.Nil(err)
	// without the cancel, sub.Read() will hang until the channel idles out
	customErr := errors.New("zuf")
	go cancel(customErr)
	_, err = sub.Read()
	require.ErrorIs(err, customErr)
}

func TestTrickle_SetSubStart(t *testing.T) {
	require, url := makeServer(t)
	wg := &sync.WaitGroup{}

	// Test:
	// 1. Subscribe from the beginning
	// 2. Subscribe from the current seq
	// 3. Subscribe from the next seq
	// 4. Subscribe from a specific seq

	// 1. Subscribe from the beginning
	subBeginning, err := NewTrickleSubscriber(TrickleSubscriberConfig{
		URL: url,
		Ctx: t.Context(),
	})
	require.Nil(err)
	wg.Add(1)
	go func() {
		defer wg.Done()
		expected := []string{"zeroth", "first", "second", "third", "fourth"}
		for _, e := range expected {
			resp, err := subBeginning.Read()
			require.Nil(err)
			buf, err := io.ReadAll(resp.Body)
			require.Nil(err)
			require.Equal(e, string(buf))
			resp.Body.Close()
		}
	}()

	time.Sleep(10 * time.Millisecond) // give subscriber time to latch on

	pub, err := NewTricklePublisher(url)
	require.Nil(err)
	defer pub.Close()

	require.Nil(pub.Write(bytes.NewReader([]byte("zeroth"))))
	require.Nil(pub.Write(bytes.NewReader([]byte("first"))))

	// 2. Subscribe from the current seq
	seq := Current
	subCurrent, err := NewTrickleSubscriber(TrickleSubscriberConfig{
		Ctx:   t.Context(),
		URL:   url,
		Start: &seq,
	})
	require.Nil(err)
	wg.Add(1)
	go func() {
		defer wg.Done()
		expected := []string{"first", "second", "third", "fourth"}
		for _, e := range expected {
			resp, err := subCurrent.Read()
			require.Nil(err)
			buf, err := io.ReadAll(resp.Body)
			require.Nil(err)
			require.Equal(e, string(buf))
			resp.Body.Close()
		}
	}()

	// 3. Subscribe from the next seq
	seq = Next
	subNext, err := NewTrickleSubscriber(TrickleSubscriberConfig{
		Ctx:   t.Context(),
		URL:   url,
		Start: &seq,
	})
	require.Nil(err)
	wg.Add(1)
	go func() {
		defer wg.Done()
		expected := []string{"second", "third", "fourth"}
		for _, e := range expected {
			resp, err := subNext.Read()
			require.Nil(err)
			buf, err := io.ReadAll(resp.Body)
			require.Nil(err)
			require.Equal(e, string(buf))
			resp.Body.Close()
		}
	}()

	time.Sleep(10 * time.Millisecond) // give subscribers time to latch on

	require.Nil(pub.Write(bytes.NewReader([]byte("second"))))
	require.Nil(pub.Write(bytes.NewReader([]byte("third"))))
	require.Nil(pub.Write(bytes.NewReader([]byte("fourth"))))

	// 4. Subscribe from a specific seq
	seq = 1
	subSeq, err := NewTrickleSubscriber(TrickleSubscriberConfig{
		Ctx:   t.Context(),
		URL:   url,
		Start: &seq,
	})
	require.Nil(err)
	wg.Add(1)
	go func() {
		defer wg.Done()
		expected := []string{"first", "second", "third", "fourth"}
		for _, e := range expected {
			resp, err := subSeq.Read()
			require.Nil(err)
			buf, err := io.ReadAll(resp.Body)
			require.Nil(err)
			require.Equal(e, string(buf))
			resp.Body.Close()
		}
	}()

	wg.Wait()
	pub.Close()
}

func makeServer(t *testing.T) (*require.Assertions, string) {
	require, url, _ := makeServerWithServer(t)
	return require, url
}

func makeServerWithServer(t *testing.T) (*require.Assertions, string, *Server) {
	// use this function if these defaults work, otherwise copy-paste
	require := require.New(t)
	mux := http.NewServeMux()
	server := ConfigureServer(TrickleServerConfig{
		Mux: mux,
	})
	stop := server.Start()
	ts := httptest.NewServer(mux)
	t.Cleanup(func() {
		stop()
		ts.Close()
		//goleak.VerifyNone(t)
	})

	// create the channel locally on the server
	chanName := "testest"
	lp := NewLocalPublisher(server, chanName, "text/plain")
	lp.CreateChannel()

	return require, ts.URL + "/" + chanName, server
}

func subConfig(t *testing.T, url string) TrickleSubscriberConfig {
	return TrickleSubscriberConfig{URL: url, Ctx: t.Context()}
}
