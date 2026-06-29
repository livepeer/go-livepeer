# Unreleased Changes

## v0.X.X

### Breaking Changes 🚨🚨

* [#3959](https://github.com/livepeer/go-livepeer/pull/3959) remote-signer: Refuse to start when `/generate-live-payment` would be unauthenticated (`-remoteSignerWebhookUrl` unset) on a publicly-accessible `-httpAddr`; pass `-remoteSignerAllowNoAuth` to override (@rickstaa)

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
