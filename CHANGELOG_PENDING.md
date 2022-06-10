# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨

### Features ⚒

#### General
- \#2429 Binaries are uploaded to gcp cloud storage following the new directory structure (@hjpotter92)
- \#2437 Make CLI flag casing consistent for dataDir flag (@thomshutt)
- \#2440 Create and push `go-livepeer:latest` tagged images to dockerhub (@hjpotter92)

#### Broadcaster
- \#2327 Parallelize handling events in Orchestrator Watcher (@red-0ne)

#### Orchestrator
- \#2290 Allow orchestrator to set max ticket faceValue (@0xB79)

#### Transcoder

### Bug Fixes 🐞
- \#2339 Fix failing `-nvidia all` flag on VM (red-0ne)

#### CLI
- \#2438 Add new menu option to gracefully exit livepeer_cli (@emranemran)
- \#2431 New flag to enable content detection on Nvidia Transcoder: `-detectContent`. When set, Transcoder will initialize Tensorflow runtime on each Nvidia GPU, and will run an additional Detector profile, if requested by the transcoding job.


#### General

#### Broadcaster

#### Orchestrator

#### Transcoder
