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

* [#3939](https://github.com/livepeer/go-livepeer/pull/3939) server: Guard nil liveParams so gateway no longer panics on non-live AI requests when no orchestrators are available (@SAY-5)

#### Broadcaster

#### CLI
