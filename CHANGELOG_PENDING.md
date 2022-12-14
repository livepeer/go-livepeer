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

- \#2635 Fix entrypoint path in built docker images (@hjpotter92)
- \#2646 Include HttpIngest and LocalVerify in param table on startup (@yondonfu)

#### Broadcaster

- \#2666 Re-use a session as long as it passes the latency score threshold check (@yondonfu)

#### Orchestrator
- \#2639 Increase IdleTimeout for HTTP connections (@leszko)
- \#2685 Add a log message when sessions are closed by the Broadcaster. Increase transcode loop timeout (@mj1)

#### Transcoder
