# Unreleased Changes

## v0.X.X

### Breaking Changes 🚨🚨

- Removed the deprecated ai-runner batch pipelines (text-to-image, image-to-image, image-to-video, upscale, audio-to-text, segment-anything-2, llm, image-to-text, text-to-speech). Gateway and orchestrator routes for them return `410 Gone`, `-aiModels` and price configuration reject them, and the standalone `-aiWorker` node type together with the `AIWorker` gRPC service and `/aiResults` endpoint are gone. `-aiWorker` now requires `-orchestrator`; `-aiRunnerImage` and the `default`/`batch` keys of `-aiRunnerImageOverrides` are removed. `live-video-to-video` and transcoding are unaffected.

### Features ⚒

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞

#### General

#### Broadcaster

#### CLI
