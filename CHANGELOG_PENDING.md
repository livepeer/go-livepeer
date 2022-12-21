# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨

### Features ⚒

#### General
- \#2696 Add Rinkeby network deprecation warning (@leszko)

#### Broadcaster

#### Orchestrator

#### Transcoder
- \#2686 Control non-stream specific scene classification with command line args

### Bug Fixes 🐞
- \#2697 Fix backwards compatibility of livepeer_cli with prior livepeer version

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
