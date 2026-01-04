# Unreleased Changes

## v0.X.X

### Breaking Changes 🚨🚨

### Features ⚒

* Gateway-native WHEP server for playback of realtime AI video

#### General

* [#3841](https://github.com/livepeer/go-livepeer/pull/3841) byoc/streaming: add streaming workload processing capability to BYOC (@ad-astra-video)

#### Broadcaster

#### Orchestrator
- \#3693 Forward `HF_TOKEN` env var (for huggingface) to runner container on spawn (@hjpotter92)
- \#3693 Better hostname for runner container to distinguish when multiple containers are running on same host (@hjpotter92)

* [#3814](https://github.com/livepeer/go-livepeer/pull/3814) ai/worker: Add scope pipeline support to worker and build scripts (@victorges)
* [#3823](https://github.com/livepeer/go-livepeer/pull/3823) ai/worker: Add sd15-v2v image support (@victorges)
* [#3843](https://github.com/livepeer/go-livepeer/pull/3843) ai/worker: Add sdxl-v2v image support (@victorges)

#### Transcoder

### Bug Fixes 🐞

#### General

* [#3777](https://github.com/livepeer/go-livepeer/pull/3777) docker: Forcefully SIGKILL runners after timeout (@pwilczynskiclearcode)
* [#3779](https://github.com/livepeer/go-livepeer/pull/3779) worker: Fix orphaned containers on node shutdown (@victorges)
* [#3781](https://github.com/livepeer/go-livepeer/pull/3781) worker/docker: Destroy containers from watch routines (@victorges)

#### CLI
