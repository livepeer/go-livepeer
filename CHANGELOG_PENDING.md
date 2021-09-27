# Unreleased Changes

## vX.X

### Breaking Changes 🚨🚨

- \#2027 Nvidia GPUs belonging to the Kepler series (GeForce 600, 700 series) and older, are no longer supported by go-livepeer. Cuda 11 is needed for newer GPUs, which [only supports the Maxwell series](https://arnon.dk/matching-sm-architectures-arch-and-gencode-for-various-nvidia-cards/) or newer.

### Features ⚒

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

- \#2027 Fix a memleak in the (experimental) AI content detection filter (@cyberj0g)
