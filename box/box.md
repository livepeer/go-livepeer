# Realtime Video AI in a Box

## Requirements
- go-livepeer compilation configuration (executing `make` should succeed)
- [mediamtx](https://github.com/bluenviron/mediamtx) is installed (executing `mediamtx` should succeed)
- [ffmpeg](https://ffmpeg.org/) is installed (executing `ffmpeg` and `ffplay` should succeed)
- Docker installed

## Usage

1. Start everything with the following command
```bash
make box
```
2. Start streaming
```bash
make box-stream
```

To also send the output to an external RTMP endpoint, set the `RTMP_OUTPUT` env var when running the above command.
```bash
RTMP_OUTPUT=rtmp://rtmp.livepeer.com/live/$STREAM_KEY make box-stream
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
