# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨

- The `-depositMultiplier` flag has been removed. This flag was previously used by a broadcaster to configure a maximum face value for ticket params sent by orchestrator by dividing the broadcaster's current deposit by the flag's value. The `-maxFaceValue` flag can now be used to explicitly set the maximum face value for ticket params instead.

### Features ⚒

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞

#### CLI

#### General

#### Broadcaster
- [#2573](https://github.com/livepeer/go-livepeer/pull/2573) server: Fix timeout for stream recording background jobs (@victorges)
- [#2577](https://github.com/livepeer/go-livepeer/pull/2577) Replace -depositMultiplier flag with -maxFaceValue flag (@yondonfu)

#### Orchestrator

#### Transcoder
