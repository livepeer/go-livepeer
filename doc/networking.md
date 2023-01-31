# Livepeer Networking Protocol v2 Spec

## High level flow

![broadcaster-transcoder network v2 1](https://user-images.githubusercontent.com/292510/41455677-c8437268-7032-11e8-9ce8-bfdd9b6e3fc0.png)
[Sequence diagram source](https://sequencediagram.org/index.html#initialData=C4S2BsFMAIBkQG6QA6UgJ2gOUsA7gPboDWIAdgObQIBMAUHcgIbqgDGIzZw0ASpBRABnYOgCedAELoCTACZsmIjAFoAfP0EjxALmgB5dGwAWkbU2BE+kAI51Nw0WJXrpshUuAY9hk2dEWVvxCyAxu8orK6Oq+puaW6DoAOmQAKuhMZEJsBHIY1nbhHlEAPC6x-hkJOumZ2bn5AJJkAGYEDAzgBAShRZFe0Wq1WTl5idAAygIAtpDcBVIyEZ4YZSrD9WN6UxSz88Ghc3J0QA)

For a reference to each message used by Livepeer, refer to the [Protocol Buffers definitions](https://github.com/livepeer/go-livepeer/blob/master/net/lp_rpc.proto).


## Broadcaster to Registry

The broadcaster does an on-chain lookup to retrieve the orchestrator's ServiceURI in order to discover prices, capabilities and to initiate a transcoding job.

## Broadcaster to Orchestrator:

### gRPC `GetOrchestrator : OrchestratorRequest -> OrchestratorInfo`

The `GetOrchestrator` method is called by the broadcaster to discover information about an orchestrator. The request contains the following data:

```protobuf
// This request is sent by the broadcaster in `GetOrchestrator` to request
// information on the orchestrator.
message OrchestratorRequest {

  // Ethereum address of the broadcaster
  bytes address = 1;

  // Broadcaster's signature over its hex-encoded address
  bytes sig     = 2;
}
```

The `sig` field is the broadcaster's signature using its Ethereum key, over the broadcaster's public Ethereum address, expressed as a hex-encoded string. The signature consists of the resulting bytes without any further encoding.
`sig = broadcaster.Sign(address.Hex())`

Verification of `OrchestratorRequest` consists of the following steps:
1. Check the signature `sig` was produced by the address given by `address`.

The `OrchestratorInfo` response contains:

```protobuf
// The orchestrator sends this in response to `GetOrchestrator`, containing the
// miscellaneous data related to the job.
message OrchestratorInfo {

  // URI of the orchestrator to use for submitting segments
  string orchestrator = 1;

  // Parameters for probabilistic micropayment tickets
  TicketParams ticket_params = 2;

  // Orchestrator's preferred object storage, if any
  repeated OSInfo storage = 32;
}
```

## Broadcaster to Transcoder

### POST `/segment`

Invoked each by the broadcaster for each segment that needs to be transcoded. The orchestrator address is taken from the `orchestrator` field in `OrchestratorInfo`.

#### Required Headers:

* **Livepeer-Segment**
Proves that the broadcaster generated this segment. Serialized protobuf struct, base64 encoded.
```protobuf
// Data included by the broadcaster when submitting a segment for transcoding.
message SegData {

  // Manifest ID this segment belongs to
  bytes manifestId = 1;

  // Sequence number of the segment to be transcoded
  int64 seq        = 2;

  // Hash of the segment data to be transcoded
  bytes hash       = 3;

  // Transcoding profiles to use
  bytes profiles   = 4;

  // Broadcaster signature for the segment. Corresponds to:
  // broadcaster.sign(manifestId | seqNo | dataHash | profiles)
  bytes sig        = 5;

  // Broadcaster's preferred storage medium(s)
  repeated OSInfo storage = 32;
}
```
* Content-Type
`video/MP2T` or `application/vnd+livepeer.uri`

The composition of the body (and certain headers) varies based on the content-type. For the content-type of `video/MP2T` , the body is composed of the bytes of the segment. For the content-type of `application/vnd+livepeer.uri`, the body holds a URI where the data can be downloaded from.

Processing a `/segment` request consists of the following steps:

1. Verify the segment signature from the broadcaster.
1. Download the body
1. Verify the keccak256 hash of the body matches `SegData.hash`
1. Return 200 OK header. If any of the above steps fail, a non-200 response is returned.
1. Transcoder performs additional checks and transcodes the segment.
1. Return a `TranscodeResult` body based on the results of the transcode.

#### Response:

The response is split into two parts: the 200 OK  (or error) is sent after the download, and the response body consisting of a `TranscodeResult` is sent after the transcode completes. This gives broadcasters approximate visibility into how long the upload and transcode steps each take.

```protobuf
// Response that a transcoder sends after transcoding a segment.
message TranscodeResult {

    // Sequence number of the transcoded results.
    int64 seq = 1;

    // Result of transcoding can be an error, or successful with more info
    oneof result {
        string error = 2;
        TranscodeData data = 3;
    }
}

// A set of transcoded segments following the profiles specified in the job.
message TranscodeData {

    // Transcoded data, in the order specified in the job options
    repeated TranscodedSegmentData segments = 1;

    // Signature of the hash of the concatenated hashes
    bytes sig = 2;
}

// Individual transcoded segment data.
message TranscodedSegmentData {

    // URL where the transcoded data can be downloaded from
    string url = 1;

}
```

### Notes

Currently, any errors are dumped directly into the response in stringified form. This gives broadcasters more information to diagnose problems with remote transcoders. However, we may not want to return such details forever, as this may leak internal information that is best left private to a transcoder.

Broadcasters can use the difference in time between the request submission and the 200ok to approximate the upload time. The time between the 200ok and receiving the response body approximates the transcode time.

There is an end-to-end request timeout of 8 seconds, However, issues are likely to appear earlier, and any issues will likely to lead to gaps in playback and stuttering. For example, live streams that consistently take 4+ seconds (the segment length) to upload and transcode will be outrun by players.

## Ping

### gRPC `Ping : PingPong -> PingPong`

Upon startup, an orchestrator will verify its availability on the network. This is done by sending itself a `Ping` request with a random value, and verifying its own signature in the response.

```protobuf
message PingPong {

  // Implementation defined
  bytes value = 1;
}
```

### Notes

The address the orchestrator uses to check availability is as follows:
* If a `-serviceAddr` is set, use that address
* If a Service URI is set in the Ethereum service registry, use that address
* Otherwise, discover the node's public IP and use that address

## Orchestrator To Redeemer

*Applicable when running a ticket redemption service by using the `-redeemer` flag*

### gRPC `QueueTicket`: `Ticket` -> `QueueTicketRes`

`QueueTicket` is a unary RPC method that is called by an Orchestrator to send a winning ticket to the `Redeemer`. The request message has following format: 

```protobuf
message Ticket {
    TicketParams ticket_params                  = 1;
    bytes sender                                = 2;
    TicketExpirationParams expiration_params    = 3;
    TicketSenderParams sender_params            = 4;
    bytes recipient_rand                        = 5;
}
```

The `Redeemer` will send back an empty message, `QueueTicketRes` on success as well as a `200 OK` status. Upon error a `500 Internal Server Error` error code will be returned alongside the error. 

### gRPC `MaxFloat`: `MaxFloatReq` -> `MaxFloatUpdate`

`MaxFloat` is a unary RPC method that is called by an Orchestrator when it requires the max float for a sender but no local cache is available. Its request takes in a sender's ethereum address:

```protobuf
message MaxFloatReq {
    bytes sender = 1;
}
```

The server will respond with the `MaxFloat` for that `sender` in raw bytes format:

```protobuf
message MaxFloatUpdate {
    bytes max_float = 1;
}
```

### gRPC `MonitorMaxFloat`: `MaxFloatReq` -> stream `MaxFloatUpdate`

`MonitorMaxFloat` is a server-side streaming RPC method that is called by an Orchestrator to receive max float updates for a sender. The stream will remain open until either the client or the server closes the stream or connection. 

The request follows the same format as the `MaxFloat` method:

```protobuf
message MaxFloatReq {
    bytes sender = 1;
}
```

The response message also follows the same format but is now a _stream_  of `MaxFloatUpdate` messages: 

```protobuf
message MaxFloatUpdate {
    bytes max_float = 1;
}
```


## TLS Certificates

Self-signed, with the DNSName field is set to the host name as specified in the registry URL. Generated anew each time a transcoder node starts up.

### Notes
Orchestrator/transcoder certificates are self-signed, and generated anew each time the node starts up. The current TLS implementation in the broadcaster will fail out if the DNSName field does not match, otherwise the self-signed certificate is not verified.

IPs will also work in the DNS Name field (at least, the go client does not fail out). However, this may be problematic for orchestrators that are on unstable IPs or otherwise "move around". Arguably, orchestrators shouldn't move around, so perhaps this would serve to discourage that mode of operation.

## Design Considerations

### gRPC and HTTP

The division of the protocol into gRPC and plain-HTTP parts may seem odd. There
is a method to the madness: gRPC messages are encoded using Protocol Buffers,
and Protocol Buffers was not designed to handle [large blobs of data](https://developers.google.com/protocol-buffers/docs/techniques#large-data).
Hence, when we need to transmit a large blob (such as a video segment), the
transmission is done through raw HTTP. For purposes other than sending large
blobs, gRPC gives us a convenient framework for building network protocols.

### Upgrade Path

See the [official recommendations](https://developers.google.com/protocol-buffers/docs/proto#updating)
for updating Protocol Buffers messages. The same principles of "add but don't
modify or remove fields" carry over to gRPC service definitions: new services
can be added, but names should not be changed, nor should services be removed
unless the intent is to break backwards compatibility.
