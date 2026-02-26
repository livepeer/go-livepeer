# Unreleased Changes

## v0.X.X

### Breaking Changes 🚨🚨

### Features ⚒

#### General

#### Broadcaster

#### Orchestrator
- \#3693 Forward `HF_TOKEN` env var (for huggingface) to runner container on spawn (@hjpotter92)
- \#3693 Better hostname for runner container to distinguish when multiple containers are running on same host (@hjpotter92)

* [#3857](https://github.com/livepeer/go-livepeer/pull/3857) byoc: fix orchestrator streaming reserve capacity (@ad-astra-video)

#### Transcoder

### Bug Fixes 🐞

#### General

* build: install XQuartz in macOS CI to fix arm64 builds (@hjpotter92)

#### CLI
