# RTMP Ingest

Livepeer starts a RTMP server on the default port of 1935 as the ingest point
into the Livepeer network. Upon ingest, the segmenter pulls the RTMP stream
prior to transcoding.

### Ingest Configuration

By default, the RTMP server listens to localhost on the default RTMP port of 1935.

This can be set at node start-up time with the `-rtmpAddr` flag, which takes an
`interface:port` pair, such as `-rtmpAddr 0.0.0.0:1936` which would make the
Livepeer node listen to all interfaces on port 1936.

The node has a default maximum of 10 concurrent RTMP sessions. To change this, run the node with the `-maxSessions` flag indicating the limit, for example `-maxSessions 100` to raise the limit to 100 concurrent sessions.

### Stream Naming and Addressing

The stream name is taken to be the first part of the RTMP URL path. The stream name may optionally be prefixed with `/stream/` to match the HLS output address.

```
# Canonical Form
rtmp://localhost/movie1
rtmp://localhost/movie2

# Alternate Form
rtmp://localhost/stream/movie1
rtmp://localhost/stream/movie2

# Output URL
http://localhost:8935/stream/movie1.m3u8
http://localhost:8935/stream/movie2.m3u8
```

If no name is provided, then a stream name is randomly generated. For example:

```
# Ingest URL
rtmp://localhost
rtmp://localhost/stream # alternate form; /stream gets stripped

# RTMP Playback URL
rtmp://localhost/<randomStreamName>/<randomStreamKey>

# HLS Playback URL
http://localhost:8935/stream/<randomStreamName>.m3u8
```

There are two options for using randomly generated stream names.

If the node is started with the `-currentManifest` flag, then the latest stream can be
accessed via the HLS `current.m3u8` endpoint, regardless of its name, eg

`curl http://localhost:8935/stream/current.m3u8`

Alternatively, a list of active streams can be found by querying the CLI API:

`curl http://localhost:7935/status`


### Stream Authentication

Streams can be authenticated through a webhook. See the documentation on the
[RTMP Authentication Webhook](rtmpwebhookauth.md) for more details.

### RTMP Playback Protection

The RTMP stream can be played back, or pulled from Livepeer by another part of
the ingest infrastructure. To prevent unauthorized RTMP playback of streams
whose name is known, a stream key is randomly appended to the playback URL at
ingest itme. However, the broadcaster can control the stream key by
appending the key to the RTMP URL.


```
# Ingest and Playback URL: protected by stream key
rtmp://localhost/movie/Secret/Stream/Key

# HLS Output URL: publicly known
http://localhost:8935/stream/movie.m3u8
```

Here, the stream name is `movie` and the stream key is `Secret/Stream/Key`.
The RTMP stream can then be played back with this complete RTMP URL. The key is
optional; if one is not supplied, then a random key will be generated. The key
may also be specified via webhook.

### HTTP Push

Livepeer starts an HTTP server on the default port of 8935, as another ingest point
into the Livepeer network. Upon ingest, HTTP stream is pushed to the segmenter
prior to transcoding. The stream can be pushed via a PUT or POST HTTP request to the
`/live/` endpoint. HTTP request timeout is 8 seconds.

HTTP ingest is enabled by default. However, if the HTTP server is publicly accessible (i.e. listening on a non-local host) and an authentication webhook URL is not specified then HTTP ingest will be disabled. In this case, to enable HTTP ingest, set an authentication webhook URL using `-authWebhookUrl` and/or use the `-httpIngest` flag when starting the node. To always disable HTTP ingest start the node with `-httpIngest=false`.

The body of the request should be the binary data of the video segment.

Two HTTP headers should be provided:
  * `Content-Resolution` - in the format `widthxheight`, for example: `1920x1080`.
  * `Content-Duration` - duration of the segment, in milliseconds. Should be an integer.
    If Content-Duration is missing, 2000ms is assumed by default.

The upload URL should have this structure:

```
http://broadcasters:8935/live/movie/12.ts
```

Where `movie` is name of the stream and `12` is the sequence number of the segment.

The HLS manifest will be available at:

```
http://broadcasters:8935/stream/movie.m3u8
```

MPEG TS and MP4 are supported as formats. To receive results as MP4, upload the
segment to a path ending with ".mp4" rather than ".ts", such as:

```
http://broadcasters:8935/live/movie/14.mp4
```

Possble statuses returned by HTTP request:
- 500 Internal Server Error - in case there was error during segment's transcode
- 503 Service Unavailable - if the broadcaster wasn't able to find an orchestrator to transcode the segment
- 200 OK - if transcoded successfully. Returned only after transcode completed 

Optionally, actual transcoded segments or URLs pointing to them can be returned in the response.
If `multipart/mixed` specified in the `Accept` header, then `Content-Type` of the response will be set to `multipart/mixed`, and the body will consist of 'parts', according to [RFC1341](https://www.w3.org/Protocols/rfc1341/7_2_Multipart.html). If external object storage is used by broadcaster, then each part will have `application/vnd+livepeer.uri` content type, and will contain URL for the transcoded segment. If no external object storage is used, then `Content-Type` of each part will be `video/MP2T`, and the body of the part will contain the actual transcoded segment.

Each part will also contain these headers:
- `Content-Length` - length of the part in bytes
- `Rendition-Name` - profile name of the rendition, either as specified in broadcaster's configuration or as returned from webhook. For example - `P144p25fps16x9`
- `Content-Disposition` will contain `attachment; filename="FILENAME"`. For example - `attachment; filename="P144p25fps16x9_12.ts"` or `attachment; filename="P144p25fps16x9_17.txt"`




Sample URLs and requests:

```
# Push URL, MPEG TS
http://localhost:8935/live/movie/12.ts

# HLS Playback URL
http://localhost:8935/stream/movie.m3u8

# Curl request, MPEG TS
curl -X PUT -H "Accept: multipart/mixed" -H "Content-Duration: 2000" -H "Content-Resolution: 1920x1080"  --data-binary "@bbb0.ts" http://localhost:8935/live/movie/0.ts

# Curl request, MP4
curl -X PUT -H "Accept: multipart/mixed" -H "Content-Duration: 2000" -H "Content-Resolution: 1920x1080"  --data-binary "@bbb1.ts" http://localhost:8935/live/movie/1.mp4

# HTTP push via FFmpeg
# (ffmpeg produces 2s segments by default; Content-Duration header will be missing but go-livepeer will presume 2s)
ffmpeg -re -i movie.mp4 -c:a copy -c:v copy -f hls http://localhost:8935/live/movie/
```

Example responses:

```
HTTP/1.1 200 OK
Content-Type: multipart/mixed; boundary=2f8514cee02991b34c00
Date: Fri, 31 Jan 2020 00:02:22 GMT
Transfer-Encoding: chunked

--2f8514cee02991b34c00
Content-Disposition: attachment; filename="P240p30fps16x9_10.txt"
Content-Length: 34
Content-Type: application/vnd+livepeer.uri
Rendition-Name: P240p30fps16x9

https://external.storage/stream/movie/P240p30fps16x9/10.ts
--2f8514cee02991b34c00--
```

```
HTTP/1.1 200 OK
Content-Type: multipart/mixed; boundary=94eaf473f7957940e066
Date: Fri, 31 Jan 2020 00:04:40 GMT
Transfer-Encoding: chunked

--94eaf473f7957940e066
Content-Disposition: attachment; filename="P240p30fps16x9_10.ts"
Content-Length: 105656
Content-Type: video/MP2T
Rendition-Name: P240p30fps16x9

Binary data here

--94eaf473f7957940e066--

```

```
HTTP/1.1 200 OK
Content-Type: multipart/mixed; boundary=94eaf473f7957940e066
Date: Fri, 31 Jan 2020 00:04:40 GMT
Transfer-Encoding: chunked

--94eaf473f7957940e066
Content-Disposition: attachment; filename="P240p30fps16x9_10.mp4"
Content-Length: 105656
Content-Type: video/mp4
Rendition-Name: P240p30fps16x9

Binary data here

--94eaf473f7957940e066--

```

### HTTP Push Examples: 
* [Python example](https://gist.github.com/j0sh/265c33197ce464ff7cd0a26f81be8f78#file-livepeer-multipart-py)
