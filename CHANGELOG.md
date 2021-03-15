# Changelog

## v0.5.15

*March 15th 2021*

This release includes an important fix for a bug that could cause transcoding to become stuck for certain corrupt or unsupported source segments. As a result of this bug, some operators saw a high number of sessions in metrics reporting on their orchestrators or transcoders despite receiving a low amount of streams in practice. We strongly recommend that all orchestrator and transcoder operators upgrade to this version as soon as possible to access this bug fix.

Thanks everyone that submitted bug reports and assisted in testing!

### Upcoming Changes

- The following flags are pending deprecation and will be removed in the next release:
    - `-gasPrice`
    - `-s3bucket`
    - `-s3creds`
    - `-gsbucket`
    - `-gskey`

### Features ⚒

#### General

- [#1759](https://github.com/livepeer/go-livepeer/pull/1759) Log non-nil error when webserver stops (@AlexMapley)
- [#1773](https://github.com/livepeer/go-livepeer/pull/1773) Fix Windows build by downloading pre-configured nasm (@iameli)
- [#1779](https://github.com/livepeer/go-livepeer/pull/1779) Fix "Build from Source" link in the README (@chrishobcroft)
- [#1778](https://github.com/livepeer/go-livepeer/pull/1778) Add live mode to `livepeer_bench` and expose additional metrics (@jailuthra)
- [#1785](https://github.com/livepeer/go-livepeer/pull/1785) Update the Windows build to be fully static and to use go1.15 (@iameli)
- [#1727](https://github.com/livepeer/go-livepeer/pull/1727) Add a `-maxGasPrice` flag to set the maximum gas price to use for transactions (@kyriediculous)
- [#1790](https://github.com/livepeer/go-livepeer/pull/1790) Add changelog process (@yondonfu)
- [#1791](https://github.com/livepeer/go-livepeer/pull/1791) Switch to Github actions for Linux build and test (@yondonfu)

#### Broadcaster

- [#1754](https://github.com/livepeer/go-livepeer/pull/1754) Count bytes of video data received/sent per stream and expose via the /status endpoint (@darkdragon)
- [#1764](https://github.com/livepeer/go-livepeer/pull/1764) Mark all input errors in LPMS as non-retryable during transcoding (@jailuthra)

#### Orchestrator

- [#1731](https://github.com/livepeer/go-livepeer/pull/1731) Add support for webhook to authenticate and set prices for broadcasters at the start of a session (@kyriediculous)
- [#1761](https://github.com/livepeer/go-livepeer/pull/1761) Add a `livepeer_router` binary that can route broadcasters to different orchestrators (@yondonfu)

### Bug Fixes 🐞

#### General

- [#1729](https://github.com/livepeer/go-livepeer/pull/1729) Make sure the block watcher service can process multiple blocks in a single polling interval (@kyriediculous)
- [#1795](https://github.com/livepeer/go-livepeer/pull/1795) Fix Darwin build by changing optimization flag used for gnutls dependency (@iameli)

#### Broadcaster

- [#1766](https://github.com/livepeer/go-livepeer/pull/1766) Flush JSON playlist during recording after it is modified (@jailuthra)
- [#1770](https://github.com/livepeer/go-livepeer/pull/1770) Fix parallel reading from object storage (@darkdragon)

#### Transcoder

- [#1775](https://github.com/livepeer/go-livepeer/pull/1775) Fix transcoder load balancer race condition around session cleanup (@jailuthra)
- [#1784](https://github.com/livepeer/go-livepeer/pull/1784) Use auth token sessionID to index into sessions map in transcoder load balancer (@jailuthra)

[Full list of changes](https://github.com/livepeer/go-livepeer/compare/v0.5.14...v0.5.15)