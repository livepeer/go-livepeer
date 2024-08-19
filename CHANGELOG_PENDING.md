# Unreleased Changes

## vX.X

This release includes a new `-hevcDecoding` flag for transcoders to configure HEVC decoding. If the flag is omitted, the default behavior on GPUs is unchanged, which is to auto-detect HEVC decoding support at transcoder start-up. Transcoders can disable HEVC decoding on GPUs if there is an issue with HEVC jobs via `-hevcDecoding=false`. CPU transcoders now have HEVC decoding disabled by default since processing HEVC jobs is CPU-heavy, but this can be enabled by setting the `-hevcDecoding` flag.

The transcoder now support mid-stream input rotations, rather than crashing or outputting cropped video as it did before.

### Breaking Changes 🚨🚨

- [#3119](https://github.com/livepeer/go-livepeer/pull/3119) CPU transcoders no longer decode HEVC or VP9 by default

### Features ⚒

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

- [#3119](https://github.com/livepeer/go-livepeer/pull/3119) Add `-hevcDecoding` flag to toggle HEVC decoding

### Bug Fixes 🐞

#### CLI

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

- [#418](https://github.com/livepeer/lpms/pull/418) lpms: Fix CUVID crash on resolution change
- [#417](https://github.com/livepeer/lpms/pull/417) lpms: Clamp resolutions in filter expression
- [#416](https://github.com/livepeer/lpms/pull/416) lpms: Rescale DTS better during FPS passthrough
