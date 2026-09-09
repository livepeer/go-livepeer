# Realtime Video AI in a Box

## Requirements
- Docker is installed (executing `docker` should succeed)
- [ffmpeg](https://ffmpeg.org/) is installed (executing `ffmpeg` and `ffplay` should succeed). The gateway also runs the `ffmpeg` on its PATH to publish output to MediaMTX. A broken build there shows up as an empty playback.
- The runner container can reach the host on port 8935 for trickle. With a default-deny firewall, allow it on the Docker bridge, for example `sudo ufw allow in on docker0 to any port 8935 proto tcp`. Blocked traffic shows up as `no orchestrators available` from the gateway.

## Without an ai-runner checkout

`make box` rebuilds the runner image from a sibling `ai-runner` checkout, which is archived. Pull the images and skip the rebuild instead:

```bash
docker pull livepeer/ai-runner:live-app-noop
docker pull livepeerci/mediamtx
export REBUILD=false
```

Without a local `mediamtx` binary, run it from the image:

```bash
docker run --rm --name mediamtx --network host -v $(pwd)/box/mediamtx.yml:/mediamtx.yml livepeerci/mediamtx
```

The noop runner takes about a minute to start. Wait until `curl http://127.0.0.1:8900/health` returns `{"status":"IDLE"}` before `make box-stream`.

## Usage (Linux AMD64)

```
export DOCKER=true
```

1. Start everything with the following command
```bash
make box
```
2. Start streaming
```bash
make box-stream
```

3. Playback the stream
```bash
make box-playback
```

## Usage (M1 / Linux ARM64)

It requires the following points:
- go-livepeer compilation configuration (executing `make` should succeed)
- [mediamtx](https://github.com/bluenviron/mediamtx) is installed (executing `mediamtx` should succeed)

1. Start everything with the following command
```bash
make box
```
2. Start streaming
```bash
make box-stream
```

3. Playback the stream
```bash
make box-playback
```

## Usage with ComfyUI Pipeline (requires GPU)

```
export DOCKER=true
export PIPELINE=comfyui
```

1. Download models

```bash
cd ../ai-runner/runner
./dl_checkpoints.sh --tensorrt
export AI_MODELS_DIR=$(pwd)/models
```

2. Start everything with the following command
```bash
make box
```
3. Start streaming
```bash
make box-stream
```

4. Playback the stream
```bash
make box-playback
```

## Additional Configuration

### RTMP Output

If you also want to send the inference output to an external RTMP endpoint, set the `RTMP_OUTPUT` env var:
```bash
export RTMP_OUTPUT=rtmp://rtmp.livepeer.com/live/$STREAM_KEY
```

This one is only required for the `box-stream` command. It is useful when you cannot use the `box-playback` command to play the stream, for example when you are using a remote non-UI machine.

### Video Input

By default, `box-stream` uses a generated test pattern. You can use a video file or webcam instead:

```bash
# Use a video file
export INPUT_VIDEO=/path/to/video.mp4

# Or use a webcam (Linux)
export INPUT_WEBCAM=/dev/video0
```

### Docker
If you want to run the box in a docker container, set the `DOCKER` env var:
```bash
export DOCKER=true
```

In general the Docker setup is simpler, but it's not possible to build the `go-liveeer` Docker image on M1 / Linux ARM64 machines.

### Rebuild

By default, all dependencies are rebuilt every time you run the `make box` command. However, if you don't want this to happen, you can set the following env variable.

```bash
export REBUILD=false
```

### Each component separately

You can also run each service separately.
```bash
make box-gateway
make box-orchestrator
make box-mediamtx
make box-stream
make box-playback
```

### Rebuilding runner
To rebuild and restart the runner, run the following command:
```bash
make box-runner
```

## Frontend

To start the frontend, run the following commands:
```bash
make box-supabase
make box-frontend
```

You can access the frontend with the following URL: http://localhost:3000/create?whipServer=http://127.0.0.1:5936/live/video-to-video/&pipeline=noop&videoJS=true

Note that the box-frontend assumes you have the https://github.com/livepeer/pipelines/ repo cloned at the parent directory.
Note also that you can start frontend together with `make box` by setting `export FRONTEND=true`.