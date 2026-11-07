# Unreleased Changes

## v0.X.X

### Breaking Changes 🚨🚨

- Removed the deprecated BYOC (bring your own container) support, including its
  endpoints, external capability registration and the `byoc` capability.
- Removed the deprecated ai-runner batch pipelines, including their endpoints and
  the standalone AI worker node type. `-aiWorker` now requires `-orchestrator`,
  and `-aiRunnerImage` is ignored.

### Features ⚒

#### General

- [#4122](https://github.com/livepeer/go-livepeer/pull/4122) Warn at startup that go-livepeer is deprecated in favor of the [node stack](https://github.com/livepeer/node) (@rickstaa)

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞

#### General

- [#4078](https://github.com/livepeer/go-livepeer/pull/4078) Default `-cliAddr` to a loopback address for every node type, so redeemer and reward caller nodes no longer fail to start by binding the CLI server to `:80` (@rickstaa)

#### Broadcaster

#### CLI
