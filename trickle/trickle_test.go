package trickle

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	defer ts.Close()
	defer stop()

	channelURL := ts.URL + "/testest"
	pub, err := NewTricklePublisher(channelURL)
	require.Nil(err)
	require.Error(StreamNotFoundErr, pub.Write(bytes.NewReader([]byte("first post"))))

	sub := NewTrickleSubscriber(channelURL)
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
	sub2 := NewTrickleSubscriber(channelURL)
	_, err = sub2.Read()
	require.Error(StreamNotFoundErr, err)

	// Spinning up a second publisher should return 404
	pub2, err := NewTricklePublisher(channelURL)
	require.Error(StreamNotFoundErr, pub2.Write(bytes.NewReader([]byte("bad post"))))
}

func TestTrickle_SetSeq(t *testing.T) {
	require := require.New(t)
	mux := http.NewServeMux()
	server := ConfigureServer(TrickleServerConfig{
		Mux:        mux,
		Autocreate: true,
	})

	stop := server.Start()
	ts := httptest.NewServer(mux)
	defer ts.Close()
	defer stop()

	channelURL := ts.URL + "/testest"

	pub, err := NewTricklePublisher(channelURL)
	require.Nil(err)
	sub := NewTrickleSubscriber(channelURL)

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
