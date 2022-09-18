# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨

### Features ⚒

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞

#### CLI

#### General
- [#2583](https://github.com/livepeer/go-livepeer/pull/2583) eth: Set tx GasFeeCap to min(gasPriceEstimate, current GasFeeCap) (@yondonfu)
- [#2586](https://github.com/livepeer/go-livepeer/pull/2586) Broadcaster: Don't pass a nil context into grpc call or it panics (@thomshutt, @cyberj0g)

#### Broadcaster
- [#2573](https://github.com/livepeer/go-livepeer/pull/2573) server: Fix timeout for stream recording background jobs (@victorges)
- [#2586](https://github.com/livepeer/go-livepeer/pull/2586) Refactor RTMP connection object management to prevent race conditions (@cyberj0g)

#### Orchestrator

#### Transcoder
