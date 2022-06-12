# Goal

- Eliminate extra latency from network layers. 
- Use the same TCP connection to process all segments of the transcoding job.
- Check signature on frame by frame to allow minimal latency.
- Improve capacity calculation and load reporting.
- Eliminate out of order segment arrival.

# Streaming transcoder

Instead of using file as input and writing output into files we will use pipes to achieve streaming input MPEG-TS into ffmpeg and streaming output out of ffmpeg at the same time.

[![](https://mermaid.ink/img/pako:eNpljssKwkAMRX9lyMKVgi9QirjygaBU1N2Mi9hJtdCZlphBRPx3p6ILMatwc3I5D8gqS5BAXla37IIsar0zXsVZ6ZWvg6i9MKE7qk5H3bgQmpx4mjM6islUHfSB0V-bFm4ui8VmO1--6fhmf-G0p0f9bt2EaZBYflQtlfb1cPwXDvTglzQe2uCIHRY2-j4aSQNyIUcGkrhayjGUYsD4Z0RDbVFobgupGJIcyyu1AYNU-7vPIBEO9IVmBZ6j44d6vgCEVFtt)](https://mermaid.live/edit#pako:eNpljssKwkAMRX9lyMKVgi9QirjygaBU1N2Mi9hJtdCZlphBRPx3p6ILMatwc3I5D8gqS5BAXla37IIsar0zXsVZ6ZWvg6i9MKE7qk5H3bgQmpx4mjM6islUHfSB0V-bFm4ui8VmO1--6fhmf-G0p0f9bt2EaZBYflQtlfb1cPwXDvTglzQe2uCIHRY2-j4aSQNyIUcGkrhayjGUYsD4Z0RDbVFobgupGJIcyyu1AYNU-7vPIBEO9IVmBZ6j44d6vgCEVFtt)

In code we use separate goroutine for pusing frames onto input pipe and separate goroutines for reading output pipes. In the above example we have 3 output profiles 720p, 480p & 320p, using 3 output pipes and 1 input pipe.

Input goroutine reads frames from the network connection and pushes frames in ffmpeg.

Output goroutines read output pipe and send encoded frames back to the network connection.


# Streaming workflow protocol

Replacing old file-based processing workflow with frame-based streaming workflow.

Changes are focused on media data flow. 

## File-based workflow:

[![](https://mermaid.ink/img/pako:eNqVlMFO4zAQhl9l5GtpVSgVkrXqIVCJC6Si5rTZg3EmxSKxg-0gEIJnZ9yEsiVBQE7xP39mvhk7fmbK5sg483jfoFF4puXGySozQE8tXdBK19IESJN1X7zQPvTV07PLvpjAQM4BTXQa1RsvFrEAhytxsWrVSxsQ7AM6aCNJUxS0KAgZ_bBlBEfjGaxRWZN7KGWgNp-GrStn80YheNxUaLrOYohIEg7nQqygtj78uXGLpQna7axR-VvJR1qbHBwV-fe5BCW4QoX6Ye-jAVdplSwhOGl87K0jhtfpgPf01lqPkLahhDhT3jL8HCztgQ04RtPJkf-ACrrCr2aZ7rBEt5WEJThtwa-wxLdY5BARKB7hmHAE08ls7qF2VqH32myiukcp2gltUW6kugOHvimD_wBNvozuDf7becS9oB-Bw3VdWrlrvJdr6_nFdLf-5SN53x2xyS57POqew2x87Kng9sT_Hz2cU_DwZHJ8MieOzLADVqGrpM7pBniOlTIWbrHCjHF6zbGQ1H7GMvNC1qbOqeAy18E6xgtZejxgsgl2_WQU48E1-G7qbpHO9fIGOPpykg)](https://mermaid.live/edit#pako:eNqVlMFO4zAQhl9l5GtpVSgVkrXqIVCJC6Si5rTZg3EmxSKxg-0gEIJnZ9yEsiVBQE7xP39mvhk7fmbK5sg483jfoFF4puXGySozQE8tXdBK19IESJN1X7zQPvTV07PLvpjAQM4BTXQa1RsvFrEAhytxsWrVSxsQ7AM6aCNJUxS0KAgZ_bBlBEfjGaxRWZN7KGWgNp-GrStn80YheNxUaLrOYohIEg7nQqygtj78uXGLpQna7axR-VvJR1qbHBwV-fe5BCW4QoX6Ye-jAVdplSwhOGl87K0jhtfpgPf01lqPkLahhDhT3jL8HCztgQ04RtPJkf-ACrrCr2aZ7rBEt5WEJThtwa-wxLdY5BARKB7hmHAE08ls7qF2VqH32myiukcp2gltUW6kugOHvimD_wBNvozuDf7becS9oB-Bw3VdWrlrvJdr6_nFdLf-5SN53x2xyS57POqew2x87Kng9sT_Hz2cU_DwZHJ8MieOzLADVqGrpM7pBniOlTIWbrHCjHF6zbGQ1H7GMvNC1qbOqeAy18E6xgtZejxgsgl2_WQU48E1-G7qbpHO9fIGOPpykg)

Extra latency is introduced by:
- Buffering input frames to produce whole segment
- Waiting to receive the entire frame before progressing further. This is noticeable in `B <> O` hop and negligible in local `Mist <> B` & `O <> T` transfers.
- Transcoding takes on average 8 times faster than segment duration. Assuming a best case scenario where the entire card is unused.
- CDN upload.

## Streaming workflow:

[![](https://mermaid.ink/img/pako:eNqFVLFy2zAM_RUc17pDhy66nAc5zpa4F6ubF1qEZF4lQiWpJL5c_r2gSEeupCSayPeAR-CB4qsoSaHIhMO_PZoSb7WsrWwPBvjrpPW61J00Hnb5fg7ea-fn6Ob2YQ7msKC5gBUJ4_O-r9fhgAwei_tfEQ17hvMMNmQMlun0B_II9IQWmPndcQcKwRM849FR-Qf9NNt5i7KFiltFOJ7jYkHrEUvUT_ghvzkROYRdpHIW301KS1hF9llaFYXcVGn3rlSk7jmr-LBJZgorjQuzS4o3R7uWDvwJzyCt5ZoXkr4Bj5iUNjU00vP6HIMaog7uyALK8gQO6xZ5EkfqjZI2xczjrjy5fKPBd3MyObHApG4XmCLmbEPZ-L9718n55yHhZL6U4Wo0JNWlw8E0bcL1k02DzZiERo2b0cJBY_virbz4lwWJ7cTUgCnsWMQBmeg5Qkmm0nXg4AZ-_JwVOnq3TwNggRDudG3kVXHJxy-ikqdfRE1621DbNchQ_D9CT_1g2WpI9vyfJgCoAoMv_t3LqDkYJ1aiRdtKrfhheQ3EQfDF5NmKjJcKK9k3_iAO5o1D-06xbVulPVmRedvjSsje0_5syss-xqS3KYJv_wDGOnvy)](https://mermaid.live/edit#pako:eNqFVLFy2zAM_RUc17pDhy66nAc5zpa4F6ubF1qEZF4lQiWpJL5c_r2gSEeupCSayPeAR-CB4qsoSaHIhMO_PZoSb7WsrWwPBvjrpPW61J00Hnb5fg7ea-fn6Ob2YQ7msKC5gBUJ4_O-r9fhgAwei_tfEQ17hvMMNmQMlun0B_II9IQWmPndcQcKwRM849FR-Qf9NNt5i7KFiltFOJ7jYkHrEUvUT_ghvzkROYRdpHIW301KS1hF9llaFYXcVGn3rlSk7jmr-LBJZgorjQuzS4o3R7uWDvwJzyCt5ZoXkr4Bj5iUNjU00vP6HIMaog7uyALK8gQO6xZ5EkfqjZI2xczjrjy5fKPBd3MyObHApG4XmCLmbEPZ-L9718n55yHhZL6U4Wo0JNWlw8E0bcL1k02DzZiERo2b0cJBY_virbz4lwWJ7cTUgCnsWMQBmeg5Qkmm0nXg4AZ-_JwVOnq3TwNggRDudG3kVXHJxy-ikqdfRE1621DbNchQ_D9CT_1g2WpI9vyfJgCoAoMv_t3LqDkYJ1aiRdtKrfhheQ3EQfDF5NmKjJcKK9k3_iAO5o1D-06xbVulPVmRedvjSsje0_5syss-xqS3KYJv_wDGOnvy)

Removing extra latency by:
- `Mist` connects immediately to `B` when the new stream starts.
- `Mist` does not buffer frames to produce segment; Frames are forwarded to `B`.
- `B` receives the first frame and immediately chooses `O` to forward the frames.
- `O` receives the first frame and immediately chooses `T` to forward the frames.
- `T` allocates transcoding resources and uses pipes to stream frames into ffmpeg and reads encoded frames from pipes to send back to `O`. Necessary latency introduced is several frames to fill decoder & encoder buffer before we have output frames. This will vary depending on desired encoding parameters.
- Because `T` processes frames in streaming fashion we don't care if the card is unused. We now care only to not overload card with too many jobs at once.
- `O` forwards encoded frames back to `B`.
- `B` uploads encoded frames to `CDN` frame by frame as results arrive.

There is still must-have latency in the system that is nearly impossible to remove:
- Network latency between nodes. `O` in same region ~50ms; `O` in a different region ~200ms.
- Each node buffers one frame before forwarding downstream 4 x ~32ms == 128ms.
- Encoding latency can be from 3 to 16 frames; 48ms - 256ms.
- Best case: `226`ms; Bad case: `584`ms;

## Transport layer

We use `websocket` protocol. Built on top of TCP. Uses HTTP request, including headers, then upgrades to a full-duplex message exchange channel. Allows message framing (each message carries size) and has message type. We can use `text` and `binary` message types.

Other message types are: `ping`, `pong` for keeping connection alive and `close` for graceful shutdown (as opposed to closing TCP connection).

`ping` serves to find out whether peer is responsive, responding back with `pong` and We use `close` when we want peer to receive all packets in flight and from kernel buffer. 

We break TCP connection when security is violated or an error happens (like a bad payment ticket) because we want to cease data exchange immediately instead of reading everything the peer wants to send/receive.

## Messages

We have control and binary messages. Mapped to textual and binary messages available in `webscoket` protocol. 

When text message type is received we assume it is JSON encoded. Then we use `id` field to determine which `struct` to use as a target for decoding.

binary data is transferred without any encoding or additional info/metadata. 

We rely on the sequential/ordered nature of TCP/websocket. 

## Binary messages

`MpegtsChunk` struct/message contains all data about the input frame, including `Bytes []byte` field. When sending `MpegtsChunk` struct is sent as JSON message, excluding binary data, and immediately followed by binary message. On receiving side text message would arrive having `id: "MpegtsChunk"` causing JSON decode into `MpegtsChunk` struct and expecting binary message to store into `MpegtsChunk.Bytes`.

`MpegtsChunk` is suitable for input frames. Mist ingest RTMP stream frame by frame and creates MPEG-TS stream in frame portions. Because Mist operates on frames it's ideal to also forward input stream in frame by frame messages.

`SelectOutput` struct/message specifies output stream that will follow. Any binary messages received later would be part of this output.

`SelectOutput` is suitable for output streams piped from ffmpeg to `T` node. Because reading a pipe can yield arbitrary-sized chunks we have chosen to allow any number of binary messages to follow `SelectOutput` control message and still refer to same output. With this we wanted to cover entire output frame received on pipe sent back with single `SelectOutput` and single binary message and also cover the case where one output frame is received in 10 consecutive reads from ffmpeg pipe.

Example of output message sequence:

```
SO alias for SelectOutput Text message
B  alias for Binary message

 . . . . . . . . . . . SO=1 B SO=2 B B B SO=3 B B B . . . . . . . . . . . .
|      Future message |        Mesage sequence     | Last message sequence |
| Input frame encoded ^
```

Usually hardware encoder produces smallest rendition frame first. Ordering of output frames is not required because all 3 output frames carry same timestamp as input frame.

Usually pipe read includes entire output frame in single call. In this example we shown several pipe reads for output 2 & 3.

## Transcoding session

All nodes involved in transcoding job keep persistent connection.

Input and output is guaranteed to arrive in proper sequence.

Transcoding resources are allocated when connection is established to chosen transcoder. Input data flows continually and `O` can detect lack of data flow.

Encoded data flow continually and `B` can detect lack of data flow and even sub-realtime data flow.

## Failover

In case connection is broken, Node crash is assumed. Failover of downstream Node happens.

`O` can switch from crashed `T` to next one. Keeping input stream up to last Keyframe is required for new `T` to continue where previous `T` left. Hardware encoder is fast enough to catch up to desired performance because transcoding is done live. No side effects should be observable on `B` side except slight delay on repeated input frames until transcoder catches up.

`B` can detect sub-realtime data flow from `O` and decide to switch to better/closer `b`. Output latency can compound due to network bottleneck when `O` is too far over internet channel or `O` is overloaded and unable to uphold desired performance. In either case `B` should switch to alternative.

## Verifying `B` signature before sending encoded frames

`O` must not send any results back to `B` before verifying input received from `B` that it is actually signed by `B`.

`B` shall send signature of media data in `MpegtsChunk` message. `O` checks signature of each received frame and fails early if signature is bad.

> Should we keep segment-wide signature if we use frame signature?

Still a open question.

## Input chunk size

`MpegtsChunk` should ideally contain single video frame and corresponding audio frames. 

Doesn't make sense to place only portion of a video frame in `MpegtsChunk` message.

If we decide to include more than 1 video frame in `MpegtsChunk` message then additional latency occurs.

## Output frame size

> Can large data chunks prevent/delay other messages from being sent through the same socket?

In H.264 video I-frame(keyframe) can be 50 times larger than B-frame (frame sequence: `IBBBPBBBPBBB`) and P-frame is 3 times larger than B-frame. In 30 frame GOP(group-of-pictures until next keyframe) I-frame is 51% of entire GOP bytes. This is exactly the case where first large frame delays other frames for playback. But playback requires frames to be delivered in proper sequence, meaning other frames can't be decoded without having first frame.

Transcoder outputs several renditions of each input frame and we have similar situation. Doesn't matter if we send frames in sequence or interleaved so they arrive at same time. Frame renditions all have same timestamp and are equally important.

In the future we may introduce image stream of low frequency having big payload. This will create spike in our network output and if we reach available bandwidth delays are introduced. To solve this we can split the payload into multiple messages to get close to constant bitrate output.

## Congestion control algorithms 

In live stream transcoding, Livepeer-Nodes need constant bitrate for inputs and outputs. The bitrate is dictated by live input stream. Ideally we should use TCP with congestion control algorithm that ramps up to desired bitrate aggressively.

> Why not use UDP?

UDP is the right choice when we want to drop some packets or when we want to implement our own congestion control algorithm.

We need to resend each lost packet because of data integrity & payment workflow.

We can use existing congestion control algorithm.

## Verification

Undefined. Feels like we could do optimistic verification: verify segments on `B` when resulting segments are already served to CDN.

Ideally we would change verification to be streaming verification comparing frame by frame each rendition and fail early on bad compare.

## Billing

No planned changes to existing logic so far. Use virtual segment boundary to hook existing billing logic into new workflow.

## MP4 output

MP4 output is not possible in streaming workflow. ffmpeg need entire output file to place MOOV atom at the start of the file.

# History & alternatives we considered

Alternatives:
- Parallel segment uploads
- HTTP streaming workflow
- Separate `Mist <> T` streaming media flow
- Direct `RTMP` connection for input to `T` and multiple `RTMP` connections for each output back to `Mist`

__Parallel segment upload__:

**File-based workflow** latency is always greater than segment duration. Mist Livepeer process was built to post HTTP segments sequentially. Next segment is prepared by Mist earlier then first segment gets transcoded. Adding even more latency.

The fix was to post next ready segment while first was being processed. Using 2 threads instead one.

Implemented and released in production.

__HTTP streaming workflow__:

Considered the quickest implementation path to streaming workflow at the start. Got input from Mist team to switch to websocket protocol. Websocket was implemented on Mist side while HTTP supported only request-reply workflow.

__Separate `Mist <> T` streaming media flow__:

Direct channel between Mist and `T` was deemed most efficient solution. Good opportunity to refactor some portions of go code base.

While exploring this direction we were unable to include all subsystems in our pipeline.

We may revisit this workflow sometime in the future.
