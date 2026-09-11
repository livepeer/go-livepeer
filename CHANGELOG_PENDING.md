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

#### Broadcaster

#### CLI

- [#4083](https://github.com/livepeer/go-livepeer/pull/4083) cli: Read bond, unbond, activation and transfer amounts in LPT instead of LPTU (@Strykar)
- [#4083](https://github.com/livepeer/go-livepeer/pull/4083) cli: Stop the unbond prompt from looping forever when nothing is bonded (@Strykar)
