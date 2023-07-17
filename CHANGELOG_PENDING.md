# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨
- \#2821 Bump nvidia/cuda base version for docker builds (@stronk-dev and @hjpotter92)

### Features ⚒

#### General
- \#2758 Accept only active Os to receive traffic and redeem tickets (@leszko)
- \#2775 Reduce number of ETH RPC calls during block polling (@leszko)
- \#2815 Add new logging methods to publish a set of public logs (@emranemran)

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞
- \#2759 Parse keystore address without 0x prefix, fix parse error logging
- \#2764 Call session end asynchronously to avoid unnecessary blocking (@mjh1)
- \#2777 Only write session end log message if session exists (@mjh1)
- \#2804 Bump livepeer-data and go version due to breaking interface change (@victorges)

#### CLI

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder
