package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/livepeer/lpms/ffmpeg"
	"github.com/stretchr/testify/require"
)

func TestNoAuthIfNoAuthURLPassed(t *testing.T) {
	_, err := authenticateStream(nil, "")
	require.NoError(t, err)
}

func TestAuthFailsIfAuthServerDoesNotExist(t *testing.T) {
	badURL, err := url.Parse("http://1.2.3.4.5.6.7.8:1234/nope")
	require.NoError(t, err)

	_, err = authenticateStream(badURL, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no such host")
}

func TestAuthFailsIfServerReturnsErrorCode(t *testing.T) {
	s, serverURL := stubAuthServer(t, http.StatusBadRequest, "")
	defer s.Close()

	_, err := authenticateStream(serverURL, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "status=400")
}

func TestAuthSucceedsIfServerReturnsEmptyBody(t *testing.T) {
	s, serverURL := stubAuthServer(t, http.StatusOK, "")
	defer s.Close()

	_, err := authenticateStream(serverURL, "")
	require.NoError(t, err)
}

func TestAuthFailsIfServerReturnsInvalidJSON(t *testing.T) {
	s, serverURL := stubAuthServer(t, http.StatusOK, `{"this": "does not have a closing brace"`)
	defer s.Close()

	_, err := authenticateStream(serverURL, "")
	require.EqualError(t, err, "unexpected end of JSON input")
}

func TestAuthFailsIfManifestIDEmpty(t *testing.T) {
	s, serverURL := stubAuthServer(t, http.StatusOK, `{"streamID": "123"}`)
	defer s.Close()

	_, err := authenticateStream(serverURL, "")
	require.EqualError(t, err, "empty manifest id not allowed")
}

func TestAuthSucceeds(t *testing.T) {
	s, serverURL := stubAuthServer(t, http.StatusOK, `{"manifestID": "123", "streamID": "456"}`)
	defer s.Close()

	resp, err := authenticateStream(serverURL, "https://some-url.com/test")
	require.NoError(t, err)
	require.Equal(t, "123", resp.ManifestID)
	require.Equal(t, "456", resp.StreamID)
}

func TestNoErrorWhenTranscodeAuthHeaderNotPassed(t *testing.T) {
	r, err := http.NewRequest(http.MethodPost, "some.com/url", nil)
	require.NoError(t, err)

	config, err := getTranscodeConfiguration(r)

	require.NoError(t, err)
	require.Nil(t, config)
}

func TestErrorWhenTranscodeAuthHeaderIsInvalidJSON(t *testing.T) {
	r, err := http.NewRequest(http.MethodPost, "some.com/url", nil)
	require.NoError(t, err)
	r.Header.Add("Livepeer-Transcode-Configuration", `{"end brace": "is missing"`)

	_, err = getTranscodeConfiguration(r)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected end of JSON input")
}

func TestTranscodeAuthHeaderParsing(t *testing.T) {
	r, err := http.NewRequest(http.MethodPost, "some.com/url", nil)
	require.NoError(t, err)
	r.Header.Add("Livepeer-Transcode-Configuration", `{"manifestID": "id-123", "sessionID": "id-456"}`)

	config, err := getTranscodeConfiguration(r)

	require.NoError(t, err)
	require.NotNil(t, config)
	require.Equal(t, "id-123", config.ManifestID)
	require.Equal(t, "id-456", config.SessionID)
}

func TestProfileEqualityWithNoProfiles(t *testing.T) {
	a := authWebhookResponse{}
	b := authWebhookResponse{}

	require.True(t, a.areProfilesEqual(b))
}

func TestProfileEqualityFailsWhenProfilesDiffer(t *testing.T) {
	a := authWebhookResponse{
		Profiles: []ffmpeg.JsonProfile{
			{
				Name:    "Name 1",
				Profile: "Profile 1",
				GOP:     "intra",
				Bitrate: 10000,
				Width:   1024,
				Height:  768,
				FPS:     1,
				Encoder: "encoder-1",
				FPSDen:  1,
			},
		},
	}
	b := authWebhookResponse{
		Profiles: []ffmpeg.JsonProfile{
			{
				Name:    "Name DIFFERENT",
				Profile: "Profile 1",
				GOP:     "intra",
				Bitrate: 10000,
				Width:   1024,
				Height:  768,
				FPS:     1,
				Encoder: "encoder-1",
				FPSDen:  1,
			},
		},
	}

	require.False(t, a.areProfilesEqual(b))
}

func TestProfileEqualityFailsWhenNumProfilesDiffer(t *testing.T) {
	a := authWebhookResponse{
		Profiles: []ffmpeg.JsonProfile{
			{
				Name:    "Name 1",
				Profile: "Profile 1",
				GOP:     "intra",
				Bitrate: 10000,
				Width:   1024,
				Height:  768,
				FPS:     1,
				Encoder: "encoder-1",
				FPSDen:  1,
			},
			{
				Name:    "Name 2",
				Profile: "Profile 2",
				GOP:     "",
				Bitrate: 20000,
				Width:   1,
				Height:  1,
				FPS:     1000,
				Encoder: "encoder-2",
				FPSDen:  900,
			},
		},
	}
	b := authWebhookResponse{
		Profiles: []ffmpeg.JsonProfile{
			{
				Name:    "Name 1",
				Profile: "Profile 1",
				GOP:     "intra",
				Bitrate: 10000,
				Width:   1024,
				Height:  768,
				FPS:     1,
				Encoder: "encoder-1",
				FPSDen:  1,
			},
		},
	}

	require.False(t, a.areProfilesEqual(b))
}

func TestProfileEqualityWithMultipleProfiles(t *testing.T) {
	a := authWebhookResponse{
		Profiles: []ffmpeg.JsonProfile{
			{
				Name:    "Name 1",
				Profile: "Profile 1",
				GOP:     "intra",
				Bitrate: 10000,
				Width:   1024,
				Height:  768,
				FPS:     1,
				Encoder: "encoder-1",
				FPSDen:  1,
			},
			{
				Name:    "Name 2",
				Profile: "Profile 2",
				GOP:     "",
				Bitrate: 20000,
				Width:   1,
				Height:  1,
				FPS:     1000,
				Encoder: "encoder-2",
				FPSDen:  900,
			},
		},
	}
	b := authWebhookResponse{
		Profiles: []ffmpeg.JsonProfile{
			{
				Name:    "Name 1",
				Profile: "Profile 1",
				GOP:     "intra",
				Bitrate: 10000,
				Width:   1024,
				Height:  768,
				FPS:     1,
				Encoder: "encoder-1",
				FPSDen:  1,
			},
			{
				Name:    "Name 2",
				Profile: "Profile 2",
				GOP:     "",
				Bitrate: 20000,
				Width:   1,
				Height:  1,
				FPS:     1000,
				Encoder: "encoder-2",
				FPSDen:  900,
			},
		},
	}

	require.True(t, a.areProfilesEqual(b))
}

func stubAuthServer(t *testing.T, respCode int, respBody string) (*httptest.Server, *url.URL) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(respCode)
				if len(respBody) > 0 {
					fmt.Fprintln(w, respBody)
				}
			},
		),
	)

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	return server, serverURL
}
