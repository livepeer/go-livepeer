# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨

- Payment/ticket metrics are no longer recorded with high cardinality keys (i.e. recipient, manifestID) which means those labels will no longer be available when using a monitoring system such as Prometheus

### Features ⚒

#### General

- \#1848 Use fee cut instead of fee share for user facing language in the CLI (@kyriediculous)
- \#1854 Allow to pass region in the custom s3 storage URL (@darkdarkdragon)
- \#1893 Remove high cardinality keys from payment metrics (@yondonfu)

#### Broadcaster

- \#1875 Update 'trying to transcode' log statement with manifestID (@kyriediculous)
- \#1837 Only log discovery errors when request is not cancelled (@yondonfu)

#### Orchestrator

- \#1845 Staking actions with hints (@kyriediculous)
- \#1873 Increase TicketParams expiration to 10 blocks (@kyriediculous)
- \#1849 Re-use remote transcoders for a stream sessions (@reubenr0d)

#### Transcoder

- \#1840 Automatically use all GPUs when -nvidia=all flag is set (@jailuthra)

### Bug Fixes 🐞

#### General

#### Broadcaster

#### Orchestrator

- \#1860 Discard low gas prices to prevent insufficient ticket faceValue errors (@kyriediculous)
- \#1859 Handle error for invalid inferred orchestrator public IP on node startup (@reubenr0d)
- \#1864 Fix OT error handling (@reubenr0d)

#### Transcoder

- \#1862 Report the correct FPS in outputs when FPS passthrough is enabled for GPU transcoding (@jailuthra)