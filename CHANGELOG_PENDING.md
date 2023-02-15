# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨

### Features ⚒

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞
- #2747 Fixed a transcoding bug that occurred when remote transcoder was removed

#### CLI

#### General
- \#2713 Add support for keyfiles with -ethKeystorePath, update flag descriptions, flagset output to stdout

#### Broadcaster
- \#2684 Fix transcode success rate metric
- \#2649 Fix for broadcaster local verification fails for vertical videos
- \#2740 Fix incorrect processing of VerificationFreq parameter, which caused fast verification to run too often

#### Orchestrator

#### Transcoder
- \#2732 Fix transcoder crash on session teardown, fix maxing out available sessions, because fast verification session teardown is not being called
