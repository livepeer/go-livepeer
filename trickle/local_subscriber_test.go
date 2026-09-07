package trickle

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"testing"
)

func TestLocalSubscriber_OverrunSeq(t *testing.T) {
	// check local subscriber behavior when the local sequence
	// falls behind the server's own sequence window
	require, url, server := makeServerWithServer(t)

	pub, err := NewTricklePublisher(url)
	require.Nil(err)
	defer pub.Close()

	sub := NewLocalSubscriber(server, "testest")

	// Publish more segments
	for i := 0; i < maxSegmentsPerStream+1; i++ {
		require.Nil(pub.Write(bytes.NewReader(fmt.Appendf(nil, "write %d", i))))
	}

	// should fetch the next one by default
	td, err := sub.Read()
	require.Nil(err)

	// more segments
	require.Nil(pub.Write(bytes.NewReader([]byte("abc"))))
	require.Nil(pub.Write(bytes.NewReader([]byte("def"))))

	// Read just the first segment for now
	data, err := io.ReadAll(td.Reader)
	require.Nil(err)
	require.Equal("abc", string(data))

	td, err = sub.Read()
	require.Nil(err)
	data, err = io.ReadAll(td.Reader)
	require.Equal("def", string(data))

	// Push data beyond the server's buffer
	for i := 0; i < maxSegmentsPerStream+1; i++ {
		require.Nil(pub.Write(bytes.NewReader(fmt.Appendf(nil, "next write %d", i))))
	}

	// sub is out of the server's segment window now
	td, err = sub.Read()
	require.Equal("seq not found", err.Error())

	sub.SetSeq(-2)
	td, err = sub.Read()
	require.Nil(err)
	data, err = io.ReadAll(td.Reader)
	require.Equal("next write 5", string(data))

	require.Nil(pub.Write(bytes.NewReader([]byte("ghi"))))

	td, err = sub.Read()
	require.Nil(err)
	data, err = io.ReadAll(td.Reader)
	require.Nil(err)
	require.Equal("ghi", string(data))

	sub.SetSeq(-1)
	td, err = sub.Read()
	require.Nil(err)

	require.Nil(pub.Write(bytes.NewReader([]byte("jkl"))))
	require.Nil(pub.Write(bytes.NewReader([]byte("mno"))))

	data, err = io.ReadAll(td.Reader)
	require.Equal("jkl", string(data))
	require.Nil(err)

	td, err = sub.Read()
	require.Nil(err)
	data, err = io.ReadAll(td.Reader)
	require.Equal("mno", string(data))
	require.Nil(err)

}

func TestLocalSubscriber_PreconnectOnEmpty(t *testing.T) {
	// Checks that the channel seq still increments even on zero-byte writes
	require, url, server := makeServerWithServer(t)

	pub, err := NewTricklePublisher(url)
	require.Nil(err)
	defer pub.Close()

	sub := NewLocalSubscriber(server, "testest")
	done := make(chan struct{})

	go func() {
		defer close(done)
		require.Nil(pub.Write(bytes.NewReader([]byte("hello"))))
		require.Nil(pub.Close())
	}()

	setSeqCount := 0

	for i := 0; ; i++ {
		sub.SetSeq(-1)
		td, err := sub.Read()
		if err != nil && err.Error() == "stream not found" {
			// would be better if this would be EOS but roll with it for now
			break
		}
		require.Nil(err)
		require.Equal(strconv.Itoa(setSeqCount), td.Metadata["Lp-Trickle-Seq"])

		n, err := io.Copy(io.Discard, td.Reader)
		require.Nil(err)
		if i == 0 {
			require.Equal(5, int(n)) // first post - "hello"
		} else {
			// second post (preconnect) is cancelled, completed as a zero-byte segment
			require.Equal(0, int(n))
		}
		setSeqCount++
	}

	<-done
	require.Equal(2, setSeqCount)
}
