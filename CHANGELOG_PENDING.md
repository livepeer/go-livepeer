# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨

### Features ⚒

#### General
- \#2758 Accept only active Os to receive traffic and redeem tickets (@leszko)
- \#2775 Reduce number of ETH RPC calls during block polling (@leszko)

#### Broadcaster

#### Orchestrator
- \#2781 Add automatic session limit and set max sessions with livepeer_cli

#### Transcoder

### Bug Fixes 🐞
- \#2759 Parse keystore address without 0x prefix, fix parse error logging
- \#2764 Call session end asynchronously to avoid unnecessary blocking (@mjh1)
- \#2777 Only write session end log message if session exists (@mjh1)

#### CLI

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder
