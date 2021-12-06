# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨

### Features ⚒

#### General

- \#2114 Add option to repeat the benchmarking process (@jailuthra)

#### Broadcaster

- \#2086 Add support for fast verification (@jailuthra @darkdragon)
- \#2085 Set max refresh sessions threshold to 8 (@yondonfu)
- \#2083 Return 422 to the push client after max retry attempts for a segment (@jailuthra)
- \#2022 Randomize selection of orchestrators in untrusted pool at a random frequency (@yondonfu)
- \#2100 Check verified session first while choosing the result from multiple untrusted sessions (@leszko)
- \#2103 Suspend sessions that did not pass p-hash verification (@leszko)
- \#2110 Transparently support HTTP/2 for segment requests while allowing HTTP/1 via GODEBUG runtime flags (@yondonfu)
- \#2124 Do not retry transcoding if HTTP client closed/canceled the connection (@leszko)
- \#2122 Add the upload segment timeout to improve failing fast (@leszko)

#### Orchestrator

#### Transcoder
- \#2094 Gracefully notify orchestrator in case of a panic in transcoder (@leszko)

### Bug Fixes 🐞

#### General

#### Broadcaster

- \#2067 Add safety checks in selectOrchestrator for auth token and ticket params in OrchestratorInfo (@yondonfu)
- \#2085 Set max refresh sessions threshold to 8 to reduce discovery frequency (@yondonfu)

#### Orchestrator

#### Transcoder
