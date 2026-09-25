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

- \#3922 Demote expected cancellation logs during stream teardown (@vavo)
- \#3885 Protect external capability map reads with the capability lock (@vavo)

#### Broadcaster

#### CLI
