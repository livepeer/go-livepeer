# Unreleased Changes

## v0.X.X

### Breaking Changes 🚨🚨

- BYOC orchestrator job requests now hard-reject invalid non-empty `Livepeer-Payment` headers with `HTTP 400 Bad Request` (`Could not parse payment`) instead of logging and continuing until balance depletion. Gateways/SDKs must omit stale payment headers or send a valid fresh payment.

### Features ⚒

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞

#### General

#### CLI
