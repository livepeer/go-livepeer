# Unreleased Changes

## v0.X.X

### Breaking Changes 🚨🚨

* [#3959](https://github.com/livepeer/go-livepeer/pull/3959) remote-signer: Refuse to start when `/generate-live-payment` would be unauthenticated (`-remoteSignerWebhookUrl` unset) on a publicly-accessible `-httpAddr`; pass `-remoteSignerAllowNoAuth` to override (@rickstaa)

### Features ⚒

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞

#### General

#### Broadcaster

* [a6c4b1e](https://github.com/livepeer/go-livepeer/commit/a6c4b1ef70d8f4d3da0e7d8164ac8d1faf80ad0e) pm: Reject ticket params with a zero expiration block so they are subjected to the economic caps (@rickstaa)

#### CLI
