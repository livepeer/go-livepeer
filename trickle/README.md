# Trickle Protocol

Trickle is a segmented publish-subscribe protocol that streams data in realtime, mainly over HTTP.

Breaking this down:

1. Data streams are called *channels* , Channels are content agnostic - the data can be video, audio, JSON, logs, a custom format, etc.

2. Segmented: Data for each channel is sent as discrete *segments*. For example, a video may use a segment for each GOP, or an API may use a segment for each JSON message, or a logging application may split segments on time intervals. The boundaries of each segment are application defined, but segments are preferably standalone such that a request for a single segment should return usable content on its own.

3. Publish-subscribe: Publishers push segments, and subscribers pull them.

4. Streaming: Content can be sent by publishers and received by subscribers in real-time as it is generated - data is *trickled* out.

5. HTTP: Trickle is tied to HTTP with GET / POST semantics, status codes, path based routing, metadata in headers, etc. However, other implementations are possible, such as a local, in-memory trickle client.

### Protocol Specification

TODO: more details

Publishers POST to /channel-name/seq

Subscribers GET to /channel-name/seq

The `channel-name` is any valid HTTP path part.

The `seq` is an integer sequence number indicating the segment, and must increase sequentially without gaps.

As data is published to `channel-name/seq`,the server will relay the data to all subscribers for `channel-name/seq`.

Clients are responsible for maintaining their own position in the sequence.

Servers may opt to keep the last N segments for subscribers to catch up on.

Servers will 404 if a `channel-name` or a `seq` does not exist.

Clients may pre-connect the next segment in order to set up the resource and minimize connection set-up time.

Publishers should only actively send data to one `seq` at a time, although they may still pre-connect to `seq + 1`

Publishers do not have to push content immeditely after preconnecting, however the server should have some reasonable timeout to avoid excessive idle connections.

If a subscriber retrieves a segment mid-publish, the server should return all the content it has up until that point, and trickle down the rest as it receives it.

If a timeout has been hit without sending (or receiving) content, the publisher (or subscriber) can re-connect to the same `seq`. (TODO; also indicate timeout via signaling)

Servers may offer some grace with leading sequence numbers to avoid data races, eg allowing a GET for `seq+1` if a publisher hasn't yet preconnected that number.

Publishers are responsible for segmenting content (if necessary) and subscribers are responsible for re-assembling content (if necessary)

Subscribers can initiate a subscribe with a `seq` of -1 to retrieve the most recent publish. With preconnects, the subscriber may be waiting for the *next* publish. For video this allows clients to eg, start streaming at the live edge of the next GOP.

Subscribers can retrieve the current `seq` with the `Lp-Trickle-Seq` metadata (HTTP header). This is useful in case `-1` was used to initiate the subscription; the subscribing client can then pre-connect to `Lp-Trickle-Seq + 1`

Subscribers can initiate a subscribe with a `seq` of -N to get the Nth-from-last segment. (TODO)

The server should send subscribers `Lp-Trickle-Size` metadata to indicate the size of the content up until now. This allows clients to know where the live edge is, eg video implementations can decode-and-discard frames up until the edge to achieve immediate playback without waiting for the next segment. (TODO)

The server currently has a special changefeed channel named `_changes` which will send subscribers updates on streams that are added and removed. The changefeed is disabled by default.
