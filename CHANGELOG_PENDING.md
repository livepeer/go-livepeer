# Unreleased Changes

## v0.X.X

### Breaking Changes 🚨🚨

### Features ⚒

* Gateway-native WHEP server for playback of realtime AI video

#### General

#### Broadcaster

#### Orchestrator
- \#3693 Forward `HF_TOKEN` env var (for huggingface) to runner container on spawn (@hjpotter92)
- \#3693 Better hostname for runner container to distinguish when multiple containers are running on same host (@hjpotter92)

#### Transcoder

### Bug Fixes 🐞

#### General

* [#3779](https://github.com/livepeer/go-livepeer/pull/3779) worker: Fix orphaned containers on node shutdown (@victorges)
* [#3777](https://github.com/livepeer/go-livepeer/pull/3777) docker: Forcefully SIGKILL runners after timeout (@pwilczynskiclearcode)

#### CLI
