# BYOC AI Stream API Documentation

The AI Stream API provides real‑time video streaming with AI processing capabilities.
All endpoints are rooted at **`/process/stream`** and are served by the Livepeer Gateway.

**Base URL:** `https://{gateway-host}/process/stream`

## Stream Setup

___Start___ stream requests must include a `Livepeer` HTTP header containing a Base64‑encoded 
JSON object (the *Livepeer Header*). The header includes the job request, parameters, 
capability name and timeout. The client sends the header **unsigned**; the Gateway signs 
it before forwarding to Orchestrators.

___Update___ requests also require a `Livepeer` header but reduced number of fields. 
See below.

Routes that only require a valid stream_id in the URL path:
- Ingest routes (WHIP and RTMP)
- Status
- Data 

Example `Livepeer ` header JSON (before Base64 encoding):
```json
#Start Stream
{
  "request": "{}",
  "parameters": "{\"enable_video_ingress\":true,\"enable_video_egress\":true, \"enable_data_output\":true}",
  "capability": "video-analysis",
  "timeout_seconds": 120
}

#Update Stream
{
  "request": "{\"stream_id\": \"254987ffsg\"}",
  "parameters": "{}",
  "timeout_seconds": 15
}
```

The fields are defined as follows:

* **timeout_seconds** – maximum processing time in seconds (integer, required).
* **capability** – name of the AI capability to use (string, required).
* **request** – JSON‑encoded `JobRequestDetails`.
* **parameters** – JSON‑encoded `JobParameters`.


### JobRequestDetails
```json
{
  "stream_id": "string"
}
```
* **stream_id** – identifier of the stream to create or operate on (required).

### JobParameters
```json
{
  "orchestrators": {
    "exclude": ["orch1_url", "orch2_url"],
    "include": ["orch1_url", "orch2_url"]
  },
  "enable_video_ingress": true,
  "enable_video_egress": true,
  "enable_data_output": true
}
```
* **orchestrators** – optional filter to select specific orchestrators.
* **enable_video_ingress** – allow video input (boolean).
* **enable_video_egress** – allow video output (boolean).
* **enable_data_output** – enable server‑sent events data channel (boolean).

## Data Types (JSON)

### StartRequest
```json
{
  "stream_name": "optional stream name that will be part of the stream_id returned",
  "rtmp_output": "optional custom RTMP output URL",
  "stream_id": "optional custom stream identifier",
  "params": "JSON‑encoded string of pipeline parameters passed to worker"
}
```
*All StartRequest fields are optional; if `stream_id` is omitted a unique id will be generated.*

### StreamUrls
```json
{
  "stream_id": "string",
  "whip_url": "string (WebRTC WHIP ingest endpoint)",
  "whep_url": "string (WebRTC WHEP egress endpoint)",
  "rtmp_url": "string (RTMP ingest endpoint)",
  "rtmp_output_url": "string (comma‑separated RTMP egress URLs)",
  "update_url": "string (POST to update parameters)",
  "status_url": "string (GET current status)",
  "data_url": "string (SSE data channel, optional)"
  "stop_url": "string"
}
```

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| **POST** | `/process/stream/start` | Create a new stream. Body = `StartRequest` JSON. Returns `StreamUrls` JSON. |
| **POST** | `/process/stream/{streamId}/stop` | Stop and clean up a running stream. |
| **POST** | `/process/stream/{streamId}/whip` | Endpoint to send ingest video via WebRTC WHIP (requires `LIVE_AI_WHIP_ADDR` set on Gateway). |
| **POST** | `/process/stream/{streamId}/rtmp` | Endpoint to send ingest video via RTMP. |
| **POST** | `/process/stream/{streamId}/update` | Update stream parameters (`params` in request body passed to worker). |
| **GET** | `/process/stream/{streamId}/status` | Retrieve current stream status. |
| **GET** | `/process/stream/{streamId}/data` | Server‑Sent Events endpoint for data channel output. |

### POST `/process/stream/start`
- *Headers*: `Livepeer` (see Authentication).  
- *Body*: JSON `StartRequest`.  All fields optional, but if `stream_id` is omitted the server generates one.
- *Response*: JSON `StreamUrls` with all ingress/egress/update/status/data URLs and stream_id to use.

### POST `/process/stream/{streamId}/update`
- *Headers*: `Livepeer`.  (requires `timeout_seconds` and `request` to be JSON encoded string of {"stream_id": streamId})
- *Path*: `streamId` must reference an existing stream.  
- *Body*: JSON body passed to pipeline worker.  
- *Response*: HTTP 200 on successful parameter update.

### POST `/process/stream/{streamId}/stop`
- *Path*: `streamId` must reference an existing active stream.  
- *Body*: JSON body passed to pipeline worker.
- *Response*: HTTP 204 No Content on success.

### POST `/process/stream/{streamId}/whip`
- *Path*: `streamId` must reference an existing stream.  
- *Environment*: `LIVE_AI_WHIP_ADDR` must be set.  
- *Response*: HTTP 200.

### POST `/process/stream/{streamId}/rtmp` 
- NOTE: this is called by MediaMTX to signal an RTMP stream is being received.  Client should use RTMP URL received in `/process/stream/start` response in streaming software.
- *Path*: `streamId` must reference an existing stream.  
- *Response*: HTTP 200.

### GET `/process/stream/{streamId}/status`
- *Path*: `streamId` must reference an existing stream.  
- *Response*: JSON:
```json
{
  "whep_url": "url", 
  "orchestrator": "https://current.orchestrator.url",
  "ingest_metrics": {...metrics...}
}
```

### GET `/process/stream/{streamId}/data`
- *Path*: `streamId` must reference an existing stream.  
- *Response*: Server‑Sent Events stream delivering data‑channel messages (e.g., inference results) when `enable_data_output` is true.
	Response is `text/event-stream` and streams JSONL from the pipeline worker container.
	Response is `503` if no data output exists for stream
