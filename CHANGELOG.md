# Changelog

## v0.5.18

*May 18 2021*

This release includes an important gas price monitoring fix that addresses cases where Ethereum JSON-RPC providers occassionally return really low gas prices for the `eth_gasPrice` RPC call, reductions in the gas cost for staking actions (under certain circumstances) using `livepeer_cli`  and improvements to split orchestrator and transcoder setups that help remote transcoders retain streams. We strongly recommend all orchestrator and transcoder operators to upgrade to this version as soon as possible to access this latest set of bug fixes and improvements.

Thanks to everyone that submitted bug reports and assisted in testing!

### Breaking Changes 🚨🚨

- Payment/ticket metrics are no longer recorded with high cardinality keys (i.e. recipient, manifestID) which means those labels will no longer be available when using a monitoring system such as Prometheus

### Features ⚒

#### General

- [#1848](https://github.com/livepeer/go-livepeer/pull/1848) Use fee cut instead of fee share for user facing language in the CLI (@kyriediculous)
- [#1854](https://github.com/livepeer/go-livepeer/pull/1854) Allow to pass region in the custom s3 storage URL (@darkdarkdragon)
- [#1893](https://github.com/livepeer/go-livepeer/pull/1893) Remove high cardinality keys from payment metrics (@yondonfu)

#### Broadcaster

- [#1875](https://github.com/livepeer/go-livepeer/pull/1875) Update 'trying to transcode' log statement with manifestID (@kyriediculous)
- [#1837](https://github.com/livepeer/go-livepeer/pull/1837) Only log discovery errors when request is not cancelled (@yondonfu)

#### Orchestrator

- [#1845](https://github.com/livepeer/go-livepeer/pull/1845) Staking actions with hints (@kyriediculous)
- [#1873](https://github.com/livepeer/go-livepeer/pull/1873) Increase TicketParams expiration to 10 blocks (@kyriediculous)
- [#1849](https://github.com/livepeer/go-livepeer/pull/1849) Re-use remote transcoders for a stream sessions (@reubenr0d)

#### Transcoder

- [#1840](https://github.com/livepeer/go-livepeer/pull/1840) Automatically use all GPUs when -nvidia=all flag is set (@jailuthra)

### Bug Fixes 🐞

#### Orchestrator

- [#1860](https://github.com/livepeer/go-livepeer/pull/1860) Discard low gas prices to prevent insufficient ticket faceValue errors (@kyriediculous)
- [#1859](https://github.com/livepeer/go-livepeer/pull/1859) Handle error for invalid inferred orchestrator public IP on node startup (@reubenr0d)
- [#1864](https://github.com/livepeer/go-livepeer/pull/1864) Fix OT error handling (@reubenr0d)

#### Transcoder

- [#1862](https://github.com/livepeer/go-livepeer/pull/1862) Report the correct FPS in outputs when FPS passthrough is enabled for GPU transcoding (@jailuthra)

## v0.5.17

*April 13 2021*

This release includes a few fixes for bugs that could cause nodes to crash due to race conditions and unexpected values returned by third party services (i.e. ETH JSON-RPC providers). We strongly recommend all node operators to upgrade to this version as soon as possible to access these bug fixes.

Thanks to everyone that submitted bug reports and assisted in testing!

### Breaking Changes 🚨🚨

- The deprecated `-gasPrice`, `-s3bucket`, `-s3creds`, `-gsbucket` and `-gskey` flags are now removed

### Features ⚒

#### General

- [#1838](https://github.com/livepeer/go-livepeer/pull/1838) Remove deprecated flags: `-gasPrice`, `-s3bucket`, `-s3creds`, `-gsbucket`, `-gskey` (@kyriediculous)

#### Broadcaster

- [#1823](https://github.com/livepeer/go-livepeer/pull/1823) Mark more transcoder errors as NonRetryable (@jailuthra)

### Bug Fixes 🐞

#### General

- [#1810](https://github.com/livepeer/go-livepeer/pull/1810) Display "n/a" in CLI when max gas price isn't specified (@kyriediculous)
- [#1827](https://github.com/livepeer/go-livepeer/pull/1827) Limit the maximum size of a segment read over HTTP (@jailuthra)
- [#1809](https://github.com/livepeer/go-livepeer/pull/1809) Don't log statement that blocks have been backfilled when no blocks have elapsed (@kyriediculous)
- [#1809](https://github.com/livepeer/go-livepeer/pull/1809) Avoid nil pointer error in SyncToLatestBlock when no blocks are present in the database (@kyriediculous)
- [#1833](https://github.com/livepeer/go-livepeer/pull/1833) Prevent nil pointer errors when fetching transcoder pool size (@kyriediculous)

#### Orchestrator

- [#1830](https://github.com/livepeer/go-livepeer/pull/1830) Handle "zero" or "nil" gas price from gas price monitor (@kyriediculous)

## v0.5.16

*March 29 2021*

This release includes an important fix for a bug that could cause broadcasters to crash due to missing data in responses from misconfigured orchestrators. We strongly recommend that all broadcaster operators upgrade to this version as soon as possible to access this bug fix. 

If you are not a broadcaster operator, then upgrading to this release is not urgent.

Thanks to everyone that submitted bug reports and assisting in testing!

### Bug Fixes 🐞

#### General

- [#1813](https://github.com/livepeer/go-livepeer/pull/1813) Check that priceInfo.pixelsPerUnit is not 0 (@kyriediculous)

#### Broadcaster

- [#1782](https://github.com/livepeer/go-livepeer/pull/1782) Fix SegsInFlight data-loss on refreshing O sessions (@darkdragon)
- [#1814](https://github.com/livepeer/go-livepeer/pull/1814) Add price checks when caching orchestrator responses during discovery (@yondonfu)
- [#1818](https://github.com/livepeer/go-livepeer/pull/1818) Additional checks to avoid nil pointer errors caused by unexpected orchestrator configurations (@kyriediculous)

[Full list of changes](https://github.com/livepeer/go-livepeer/compare/v0.5.15...v0.5.16)

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