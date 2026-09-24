# Unreleased Changes

## v0.X.X

### Breaking Changes 🚨🚨

- Removed BYOC (bring your own container): the `/process/request/`,
  `/process/stream/*`, `/process/token`, `/capability/register`,
  `/capability/unregister` and `/ai/stream/*` endpoints, external capability
  registration and job pricing, and the `byoc` capability. Orchestrators can no
  longer serve externally registered capabilities.

### Features ⚒

#### General

#### Broadcaster

#### Orchestrator

- [#4011](https://github.com/livepeer/go-livepeer/pull/4011) Add LIP-118 reward caller support so a separate wallet can call `reward()` on behalf of an orchestrator (@rickstaa). With an explicit `-reward`, an unauthorized account now exits at startup instead of silently never calling reward.

#### Transcoder

### Bug Fixes 🐞

#### General

#### Broadcaster

#### CLI
