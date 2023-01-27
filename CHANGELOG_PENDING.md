# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨

### Features ⚒

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞

#### CLI

#### General

#### Broadcaster
- \#2709 Add logging for high keyframe interval, reduce log level for discovery loop
- \#2684 Fix transcode success rate metric
- \#2649 Fix for broadcaster local verification fails for vertical videos

#### Orchestrator

#### Transcoder
- \#2732 Fix transcoder crash on session teardown, fix maxing out available sessions, because fast verification session teardown is not being called
