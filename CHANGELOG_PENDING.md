# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨

### Features ⚒

#### General
- \#2208 Improve Feed PubSub: execute subscribers' blocking operations in separate goroutines (@leszko)
- \#2222 Use L1 block number for Ticket Parameters and Round Initialization (@leszko)
- \#2240 Backfill always from the back last seen in DB (instead of the last round block) (@leszko)
- \#2171 Make transactions compatible with Arbitrum and fix setting max gas price (@leszko)
- \#2252 Use different hardcoded redeemGas when connected to an Arbitrum network (@leszko)
- \#2251 Add fail fast for Arbitrum One Mainnet when LIP-73 has not been activated yet (@leszko)
- \#2253 Redeem tickets only when recipient is active (@leszko)

#### Broadcaster

#### Orchestrator

#### Transcoder
* H.265/HEVC encoding and decoding
* VP8/VP9 decoding

### Bug Fixes 🐞
- \#2245 Fast verification fix (@oscar-davids)

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder
