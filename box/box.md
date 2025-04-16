# Realtime Video AI in a Box

## Requirements
- go-livepeer compilation configuration (executing `make` should succeed)
- [mediamtx](https://github.com/bluenviron/mediamtx) is installed (executing `mediamtx` should succeed)
- [ffmpeg](https://ffmpeg.org/) is installed (executing `ffmpeg` and `ffplay` should succeed)
- Docker installed

## Setup

### Pipeline

The box runs the `noop` pipeline by default, which doesn't need a GPU to function. If you want to run the `comfyui` pipeline instead, make sure you set the `PIPELINE` env in **all** your shell sessions:
```bash
export PIPELINE=comfyui
```

If you do run the `comfyui` pipeline, make sure you've also downloaded the required models with:
```bash
cd ../ai-runner/runner
./dl_checkpoints.sh --tensorrt
```

This will make the models available in the `./ai-runner/runner/models` folder, which is mounted by the orchestrator on the runner containers.

### RTMP Output

If you also want to send the inference output to an external RTMP endpoint, set the `RTMP_OUTPUT` env var:
```bash
export RTMP_OUTPUT=rtmp://rtmp.livepeer.com/live/$STREAM_KEY
```

This one is only required for the `box-stream` command.

## Usage

1. Start everything with the following command
```bash
make box
```
2. Start streaming
```bash
make box-stream
```

3. Playback the stream
```
make box-playback
```

## Usage (each service separately)

You can also run each service separately.
```bash
make box-gateway
make box-orchestrator
make box-mediamtx
make box-stream
make box-playback
```

## Rebuilding runner
To rebuild and restart the runner, run the following command:
```bash
make box-runner
```
