# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨

### Features ⚒

#### General

- #2938 Add `tmp` folder to `.gitignore` (@rickstaa)

#### Broadcaster

- #2896 Use FPS of 60, rather then 120 for cost estimation (@thomshutt)
- #2948 Remove logging from metrics methods (@thomshutt)

#### Orchestrator

- #2911 Set default price with livepeer_cli option 20 (@eliteprox)
- #2928 Added `startupAvailabilityCheck` param to skip the availability check on startup (@stronk-dev)
- #2905 Add `reward_call_errors` Prometheus metric (@rickstaa)

#### Transcoder

### Bug Fixes 🐞

- \[#2914](https://github.com/livepeer/go-livepeer/issues/2912) fixes a bug that prevented `pricePerBroadcaster` JSON files with line-breaks from being parsed correctly (@rickstaa).

#### CLI

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder
