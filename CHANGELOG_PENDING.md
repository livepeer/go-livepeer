# Unreleased Changes

## v0.X.X

### Breaking Changes 🚨🚨

- Removed the deprecated BYOC (bring your own container) support, including its
  endpoints, external capability registration and the `byoc` capability.

### Features ⚒

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞

#### General

- [#4037](https://github.com/livepeer/go-livepeer/pull/4037) Fix `-ethPassword` when it points at a file: a newly created keystore was encrypted with the file path and then unlocked with the file contents, so the node failed to start (@Strykar)

#### Broadcaster

#### CLI
