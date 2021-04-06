# Unreleased Changes

## vX.X

### Bug Fixes 🐞

#### General

- \#1810 Display "n/a" in CLI when max gas price isn't specified (@kyriediculous)
- \#1827 Limit the maximum size of a segment read over HTTP (@jailuthra)
- \#1809 Don't log statement that blocks have been backfilled when no blocks have elapsed
- \#1809 Avoid nil pointer error in SyncToLatestBlock when no blocks are present in the database (@kyriediculous)

#### Orchestrator

- \#1830 handle "zero" or "nil" gas price from gas price monitor (@kyriediculous)

### Features ⚒

#### Broadcaster

- \#1823 Mark more transcoder errors as NonRetryable (@jailuthra)
