# Realtime Video AI in a Box

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
export MODEL_DIR=$(pwd)/models
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
