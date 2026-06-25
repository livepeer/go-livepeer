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

* [#3958](https://github.com/livepeer/go-livepeer/pull/3958) remote-signer: Authorize the request before generating or signing a payment, so an unauthorized request no longer triggers ticket signing or sender-nonce consumption (@rickstaa)

#### Broadcaster

* [a6c4b1e](https://github.com/livepeer/go-livepeer/commit/a6c4b1ef70d8f4d3da0e7d8164ac8d1faf80ad0e) pm: Reject ticket params with a zero expiration block so they are subjected to the economic caps (@rickstaa)

#### CLI
