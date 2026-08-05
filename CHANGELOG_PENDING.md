# Unreleased Changes

## v0.X.X

### Breaking Changes 🚨🚨

### Features ⚒

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞

#### General

#### Orchestrator

- [#4012](https://github.com/livepeer/go-livepeer/pull/4012) Defer ticket redemption while a sender's deposit and reserve cannot cover the full ticket face value, instead of submitting a redemption that reverts. Tickets stay queued and are redeemed on the first block after the sender tops up, for as long as they remain valid (@rickstaa)

#### Broadcaster

#### CLI
