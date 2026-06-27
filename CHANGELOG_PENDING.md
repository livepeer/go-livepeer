# Unreleased Changes

## v0.X.X

### Breaking Changes 🚨🚨

- BYOC orchestrator job requests now hard-reject invalid non-empty `Livepeer-Payment` headers with `HTTP 400 Bad Request` (`Could not parse payment`) instead of logging and continuing until balance depletion. Gateways/SDKs must omit stale payment headers or send a valid fresh payment.

### Features ⚒

#### General

- [#3944](https://github.com/livepeer/go-livepeer/pull/3944) Bridge slog level to the glog `-v` flag so `-v` controls newer subsystem logging (@rickstaa)

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞

#### General

* [#3962](https://github.com/livepeer/go-livepeer/pull/3962) remote-signer: Default `-cliAddr` to a loopback address in remote signer mode so the node no longer fails to start by binding the CLI server to `:80` (@rickstaa)

#### Broadcaster

* [a6c4b1e](https://github.com/livepeer/go-livepeer/commit/a6c4b1ef70d8f4d3da0e7d8164ac8d1faf80ad0e) pm: Reject ticket params with a zero expiration block so they are subjected to the economic caps (@rickstaa)

#### CLI
