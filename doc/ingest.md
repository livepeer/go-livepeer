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
[RTMP Authentication Webhook](rtmpauthwebhook.md) for more details.

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
