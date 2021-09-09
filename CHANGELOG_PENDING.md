# Unreleased Changes

## vX.X

### Features ⚒

#### General

#### Broadcaster
- \#1946 Send transcoding stream health events to a metadata queue (@victorges)

#### Orchestrator

- \#2025 Add -autoAdjustPrice flag to enable/disable automatic price adjustments (@yondonfu)

#### Transcoder

- \#1979 Upgrade to ffmpeg v4.4 and improved API for (experimental) AI tasks (@jailuthra)

### Bug Fixes 🐞

- \#1992 Eliminate data races in mediaserver.go (@darkdarkdragon)
- \#2011 Configurable delay between sessions in livepeer_bench (@jailuthra)

#### General

- \#2001 Fix max gas price nil pointer error in replace transaction (@kyriediculous)

#### Broadcaster

- \#2026 Run signature verification even without a verification policy specified (@yondonfu)

#### Orchestrator

- \#2018 Only mark tickets for failed transactions as redeemed when there is an error checking the transaction (@yondonfu)

#### Transcoder
