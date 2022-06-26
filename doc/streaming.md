# Goal

- Eliminate extra latency from network layers. 
- Use the same TCP connection to process all segments of the transcoding job.
- Check signature on frame by frame to allow minimal latency.
- Improve capacity calculation and load reporting.
- Eliminate out of order segment arrival.

# Streaming transcoder

Instead of using file as input and writing output into files we will use pipes to achieve streaming input MPEG-TS into ffmpeg and streaming output out of ffmpeg at the same time.

[![](https://mermaid.ink/img/pako:eNpljssKwkAMRX9lyMKVgi9QirjygaBU1N2Mi9hJtdCZlphBRPx3p6ILMatwc3I5D8gqS5BAXla37IIsar0zXsVZ6ZWvg6i9MKE7qk5H3bgQmpx4mjM6islUHfSB0V-bFm4ui8VmO1--6fhmf-G0p0f9bt2EaZBYflQtlfb1cPwXDvTglzQe2uCIHRY2-j4aSQNyIUcGkrhayjGUYsD4Z0RDbVFobgupGJIcyyu1AYNU-7vPIBEO9IVmBZ6j44d6vgCEVFtt)](https://mermaid.live/edit#pako:eNpljssKwkAMRX9lyMKVgi9QirjygaBU1N2Mi9hJtdCZlphBRPx3p6ILMatwc3I5D8gqS5BAXla37IIsar0zXsVZ6ZWvg6i9MKE7qk5H3bgQmpx4mjM6islUHfSB0V-bFm4ui8VmO1--6fhmf-G0p0f9bt2EaZBYflQtlfb1cPwXDvTglzQe2uCIHRY2-j4aSQNyIUcGkrhayjGUYsD4Z0RDbVFobgupGJIcyyu1AYNU-7vPIBEO9IVmBZ6j44d6vgCEVFtt)

In code we use separate goroutine for pushing frames onto input pipe and separate goroutines for reading output pipes. In the above example we have 3 output profiles 720p, 480p & 320p, using 3 output pipes and 1 input pipe.

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


## Transcoding job

All nodes involved in transcoding job keep persistent connection.

Input and output is guaranteed to arrive in proper sequence.

Transcoding resources are allocated when connection is established to chosen transcoder. Input data flows continually and `O` can detect lack of data flow.

Encoded data flow continually and `B` can detect lack of data flow and even sub-realtime data flow.

Faster-than-realtime metric no longer applies. For live streams it's no longer important as `T` processes frames as they arrive, meaning one stream can't saturate encoder hardware. When `T` is overloaded all streams notice compounding latency in the output.

VOD job can saturate encoder hardware and introduce side effects on other streams on same GPU.

## Failover

In case connection is broken, Node crash is assumed. Failover of downstream Node happens.

`O` can switch from crashed `T` to next one. Keeping input stream up to last Keyframe is required for new `T` to continue where previous `T` left. Hardware encoder is fast enough to catch up to desired performance because transcoding is done live. No side effects should be observable on `B` side except slight delay on repeated input frames until transcoder catches up.

`B` can detect sub-realtime data flow from `O` and decide to switch to better/closer `b`. Output latency can compound due to network bottleneck when `O` is too far over internet channel or `O` is overloaded and unable to uphold desired performance. In either case `B` should switch to alternative.

## Verifying `B` signature before sending encoded frames

`O` must not send any results back to `B` before verifying input received from `B` that it is actually signed by `B`.

`B` shall send signature of media data in `InputChunk` message. `O` checks signature of each received frame and fails early if signature is bad.

`O` sends signature of media data in `OutputChunk` message. `B` checks signature of each received frame and disconnects `O` on bad signature.

> Should we keep segment-wide signature if we use frame signature?

Still a open question.

## Testing

Integration test is in place. Spawning single Mist, single `B`, two `O`s with multiple `T`s.

Tests cover networking and transcoding. 

Mocked: Info from chain, Node discovery, Mist ingest.

For signing to work test shares information that should be retrieved from chain, and new keys are created each time.

In the future we should include Mist and have on-chain variant.

## Compatibility

New `B` needs ability to forward jobs to old `O` versions. `B` would order `O` candidates placing `O`s with new version first.

## Deployment

With proper failover on `O` & `T` we can just start new instances and shutdown old instances. Marking nodes for shutdown to prevent routing streams to leaving nodes. Mist & `B` need to have stream migration in place.

## Input chunk size

`InputChunk` should ideally contain single video frame and corresponding audio frames. 

Doesn't make sense to place only portion of a video frame in `InputChunk` message.

If we decide to include more than 1 video frame in `InputChunk` message then additional latency occurs.

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

## MP4 output

MP4 output is not possible in streaming workflow. ffmpeg need entire output file to place MOOV atom at the start of the file.


# Discussion points

> Can we create payment ticket at segment start? Can we estimate pixels and settle later on vitual-segment-boundary?

> What is proper action on `O`'s slow pace detected by `B`

`B` sends duplicate stream to several `O`s in verification phase. Each `O` receives input and sends results at its own pace. Sometimes `O` receive pace is slow, causing input frames to buffer excessively on `O` side. `O` and `T` are intentionally coded to apply back preasure resulting in slow receive pace, however we should also consider the case when receive pace is good and send pace is slow causing latency to compound.

> How slow-pace handling differs in VOD use case ?

Without pace limit Mist can push VOD frames at network limit. This is desirable on `O` side as jobs are completed faster and payment received faster.

We should consider to put suitable pace managing logic in VOD scenario.

> Method for choosing best `O`, whose renditions we forward back to Mist

Simplest solution would be fastest first frame wins.

> When to switch to different O

> Can frame signing replace segment-wide signature?

> Decide on error-funnel mechanism

> GPU load/capacity strategy

Proposal: When GPU load approaches max we can close idle worker connections in turn stopping new job assignment to this GPU. O chooses from all available connections randomly. When GPU is maxed out no connections for more jobs.

> Mist & B stream migration when deploying new version

> What to do on duplicate manifestID/streamName? Is it possible in Mist?

# What is left

Flushing transcoder on virtual-segment-boundary, then sending back all frames to complete payment ticket.

Failove logic.

Cleanup - work in progress.

Include Netint cards in our tests.

Error funnel.

Add sequence number to segments, like frames do.

Apply `hwDeviceIndex` to StreamingTranscoder.

Compatibility with old node versions.

"Monitor" mechanism.

Filter `O`s without required capabilities from input stream.

`NeedsBypass` corner case moved to Mist.

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
