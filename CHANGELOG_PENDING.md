# Unreleased Changes

## v0.X.X

### Breaking Changes 🚨🚨

### Features ⚒

#### General

#### Broadcaster

#### Orchestrator

- [#4011](https://github.com/livepeer/go-livepeer/pull/4011) Add LIP-118 reward caller support so a separate wallet can call `reward()` on behalf of an orchestrator (@rickstaa). With an explicit `-reward`, an unauthorized account now exits at startup instead of silently never calling reward.

#### Transcoder

### Bug Fixes 🐞

#### General

- [#4037](https://github.com/livepeer/go-livepeer/pull/4037) Fix `-ethPassword` when it points at a file: a newly created keystore was encrypted with the file path and then unlocked with the file contents, so the node failed to start (@Strykar)

#### Broadcaster

#### CLI
