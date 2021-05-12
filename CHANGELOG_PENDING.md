# Unreleased Changes

## vX.X

### Features ⚒

#### General

- \#1848 Use fee cut instead of fee share for user facing language in the CLI (@kyriediculous)
- \#1854 Allow to pass region in the custom s3 storage URL (@darkdarkdragon)

#### Broadcaster

- \#1875 Update 'trying to transcode' log statement with manifestID (@kyriediculous)

#### Orchestrator

- \#1845 Staking actions with hints (@kyriediculous)
- \#1873 Increase TicketParams expiration to 10 blocks (@kyriediculous)

#### Transcoder

- \#1840 Automatically use all GPUs when -nvidia=all flag is set (@jailuthra)

### Bug Fixes 🐞

#### General

#### Broadcaster

#### Orchestrator

- \#1860 Discard low gas prices to prevent insufficient ticket faceValue errors (@kyriediculous)
- \#1859 Handle error for invalid inferred orchestrator public IP on node startup (@reubenr0d)
- \#1880 Don't mark a winning ticket as redeemed if a transaction is submitted but pending (@kyriediculous)

#### Transcoder

- \#1862 Report the correct FPS in outputs when FPS passthrough is enabled for GPU transcoding (@jailuthra)