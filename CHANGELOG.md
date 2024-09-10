# Changelog

## v0.7.9

- [#3165](https://github.com/livepeer/go-livepeer/pull/3165) Add node version and orch addr to transcoded metadata

### Features ⚒

#### Broadcaster

- [#3158](https://github.com/livepeer/go-livepeer/pull/3158) Add a metric tag for Orchestrator version

### Bug Fixes 🐞

#### Broadcaster

- [#3164](https://github.com/livepeer/go-livepeer/pull/3164) Fix media compatibility check
- [#3166](https://github.com/livepeer/go-livepeer/pull/3166) Clean up inactive sessions
- [#3086](https://github.com/livepeer/go-livepeer/pull/3086) Clear known sessions with inadequate latency scores

## v0.7.8

### Features ⚒

#### Broadcaster

- [#3127](https://github.com/livepeer/go-livepeer/pull/3127) Add flag `-ignoreMaxPriceIfNeeded` (@leszko)

## v0.7.7

This release includes a new `-hevcDecoding` flag for transcoders to configure HEVC decoding. If the flag is omitted, the default behavior on GPUs is unchanged, which is to auto-detect HEVC decoding support at transcoder start-up. Transcoders can disable HEVC decoding on GPUs if there is an issue with HEVC jobs via `-hevcDecoding=false`. CPU transcoders now have HEVC decoding disabled by default since processing HEVC jobs is CPU-heavy, but this can be enabled by setting the `-hevcDecoding` flag.

The transcoder now support mid-stream input rotations, rather than crashing or outputting cropped video as it did before.

### Breaking Changes 🚨🚨

- [#3119](https://github.com/livepeer/go-livepeer/pull/3119) CPU transcoders no longer decode HEVC or VP9 by default

#### Transcoder

- [#3119](https://github.com/livepeer/go-livepeer/pull/3119) Add `-hevcDecoding` flag to toggle HEVC decoding

### Bug Fixes 🐞

#### Transcoder

- [#418](https://github.com/livepeer/lpms/pull/418) lpms: Fix CUVID crash on resolution change
- [#417](https://github.com/livepeer/lpms/pull/417) lpms: Clamp resolutions in filter expression
- [#416](https://github.com/livepeer/lpms/pull/416) lpms: Rescale DTS better during FPS passthrough

## v0.7.6

-   [#3055](https://github.com/livepeer/go-livepeer/pull/3055) census: Rename broadcaster metrics to gateway metrics
-   [#3053](https://github.com/livepeer/go-livepeer/pull/3053) cli: add `-gateway` flag and deprecate `-broadcaster` flag.
-   [#3056](https://github.com/livepeer/go-livepeer/pull/3056) cli: add `-pricePerGateway` flag and deprecate `-pricePerBroadcaster` flag.
-   [#3060](https://github.com/livepeer/go-livepeer/pull/3060) refactor: rename internal references from Broadcaster to Gateway

### Breaking Changes 🚨🚨

### Features ⚒


## v0.7.5

### Breaking Changes 🚨🚨

### Features ⚒

#### General

- [#3050](https://github.com/livepeer/go-livepeer/pull/3050) Create option to filter Os by min livepeer version used (@leszko)
- [#3029](https://github.com/livepeer/go-livepeer/pull/3029) Initialize round by any B/O who has the initializeRound flag set to true (@leszko)
- [#3040](https://github.com/livepeer/go-livepeer/pull/3040) Fix function names (@kevincatty)

#### Broadcaster

- [#2995](https://github.com/livepeer/go-livepeer/pull/2995) server: Allow Os price to increase up to 2x mid-session (@victorges)
- [#2999](https://github.com/livepeer/go-livepeer/pull/2999) server,discovery: Allow B to use any O in case none match maxPrice (@victorges)

### Bug Fixes 🐞

#### Broadcaster

- [#2994](https://github.com/livepeer/go-livepeer/pull/2994) server: Skip redundant maxPrice check in ongoing session (@victorges)

#### Orchestrator

- [#3001](https://github.com/livepeer/go-livepeer/pull/3001) Fix transcoding price metrics (@leszko)

#### Transcoder

- [#3003](https://github.com/livepeer/go-livepeer/pull/3003) Fix issue in the transcoding layer for WebRTC input (@j0sh)

## v0.7.4

### Breaking Changes 🚨🚨

### Features ⚒

#### General

- [#2989](https://github.com/livepeer/go-livepeer/pull/2989) Revert "Update ffmpeg version" (@thomshutt)

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞

#### CLI

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

## v0.7.3

### Breaking Changes 🚨🚨

### Features ⚒

#### General

- [#2978](https://github.com/livepeer/go-livepeer/pull/2978) Update CUDA version from 11.x to 12.x (@leszko)
- [#2973](https://github.com/livepeer/go-livepeer/pull/2973) Update ffmpeg version (@thomshutt)
- [#2981](https://github.com/livepeer/go-livepeer/pull/2981) Add support for prices in custom currencies like USD (@victorges)

#### Broadcaster

#### Orchestrator

#### Transcoder

### Bug Fixes 🐞

#### CLI

#### General

#### Broadcaster

#### Orchestrator

#### Transcoder

## v0.7.2

### Breaking Changes 🚨🚨

- None

#### General
- [#2938](https://github.com/livepeer/go-livepeer/pull/2938) Add `tmp` folder to `.gitignore` (@rickstaa)

#### Broadcaster
- [#2896](https://github.com/livepeer/go-livepeer/pull/2896) Use FPS of 60, rather than 120 for cost estimation (@thomshutt)
- [#2948](https://github.com/livepeer/go-livepeer/pull/2948) Remove logging from metrics methods (@thomshutt)

#### Orchestrator
- [#2911](https://github.com/livepeer/go-livepeer/pull/2911) Set default price with livepeer_cli option 20 (@eliteprox)
- [#2928](https://github.com/livepeer/go-livepeer/pull/2928) Added `startupAvailabilityCheck` param to skip the availability check on startup (@stronk-dev)
- [#2905](https://github.com/livepeer/go-livepeer/pull/2905) Add `reward_call_errors` Prometheus metric (@rickstaa)
- [#2958](https://github.com/livepeer/go-livepeer/pull/2958) Return parsing error when failing to parse B prices (@thomshutt)

#### Transcoder

### Bug Fixes 🐞
- [#2914](https://github.com/livepeer/go-livepeer/pull/2914) fixes a bug that prevented `pricePerBroadcaster` JSON files with line-breaks from being parsed correctly (@rickstaa).

## v0.7.1

### Breaking Changes 🚨🚨

### Features ⚒

#### Broadcaster
- [#2884](https://github.com/livepeer/go-livepeer/pull/2884) Detect resolution change and reinit hw session (@leszko)
- [#2883](https://github.com/livepeer/go-livepeer/pull/2883) Skip transcoding when empty profiles were sent to B (@leszko)

#### General
- [#2882](https://github.com/livepeer/go-livepeer/pull/2882) Add parameter to force HW Session Reinit (@leszko)

## v0.7.0

### Breaking Changes 🚨🚨

### Features ⚒

#### Broadcaster
- [#2837](https://github.com/livepeer/go-livepeer/pull/2837) Set a floor of 1.5s for accepted Orchestrator response times, regardless of segment length (@thomshutt)
- [#2839](https://github.com/livepeer/go-livepeer/pull/2839) Add Orchestrator blocklist CLI parameter (@mjh1)
- [#2729](https://github.com/livepeer/go-livepeer/pull/2729) Switch to a custom error type for max transcode attempts (@mjh1)
- [#2844](https://github.com/livepeer/go-livepeer/pull/2844) Exit early during verification when Orchestrator's address is nil (@ad-astra-video)
- [#2842](https://github.com/livepeer/go-livepeer/pull/2842) Add Keys to public logs (@ad-astra-video)
- [#2872](https://github.com/livepeer/go-livepeer/pull/2872) New Selection Algorithm (@leszko)

#### Orchestrator
- [#2781](https://github.com/livepeer/go-livepeer/pull/2781) Add automatic session limit and set max sessions with livepeer_cli (@eliteprox)
- [#2848](https://github.com/livepeer/go-livepeer/pull/2848) Improve Transcode Quality (@leszko)
- [#2851](https://github.com/livepeer/go-livepeer/pull/2851) Change scaling to `scale_npp` instead of `scale_cuda` (Linux-only) (@leszko)

#### General
- [#2843](https://github.com/livepeer/go-livepeer/pull/2843) Increase log level of payment creation (@iameli)
- [#2850](https://github.com/livepeer/go-livepeer/pull/2850) Update default ports (@ad-astra-video)
- [#2859](https://github.com/livepeer/go-livepeer/pull/2859) Update currentRound endpoint to return string instead of bytes (@ad-astra-video)
- [#2878](https://github.com/livepeer/go-livepeer/pull/2878) Fix Mist JSON (@leszko)

### Bug Fixes 🐞
- [#2820](https://github.com/livepeer/go-livepeer/pull/2820) Fix segfault in setting a pricePerBroadcaster (@stronk-dev)

#### CLI
- [#2825](https://github.com/livepeer/go-livepeer/pull/2825) Enable quieter logging with -v=2 and -v=1 (@iameli)

## v0.6.0

### Breaking Changes 🚨🚨
- [#2821](https://github.com/livepeer/go-livepeer/pull/2821) Bump nvidia/cuda base version for docker builds (@stronk-dev and @hjpotter92)

### Features ⚒

#### Broadcaster
- [#2827](https://github.com/livepeer/go-livepeer/pull/2827) Introduce configurable Orchestrator blocklist (@mjh1)

#### General
- [#2758](https://github.com/livepeer/go-livepeer/pull/2758) Accept only active Os to receive traffic and redeem tickets (@leszko)
- [#2775](https://github.com/livepeer/go-livepeer/pull/2775) Reduce number of ETH RPC calls during block polling (@leszko)
- [#2815](https://github.com/livepeer/go-livepeer/pull/2815) Add new logging methods to publish a set of public logs (@emranemran)

### Bug Fixes 🐞
- [#2759](https://github.com/livepeer/go-livepeer/pull/2759) Parse keystore address without 0x prefix, fix parse error logging
- [#2764](https://github.com/livepeer/go-livepeer/pull/2764) Call session end asynchronously to avoid unnecessary blocking (@mjh1)
- [#2777](https://github.com/livepeer/go-livepeer/pull/2777) Only write session end log message if session exists (@mjh1)
- [#2804](https://github.com/livepeer/go-livepeer/pull/2804) Bump livepeer-data and go version due to breaking interface change (@victorges)

## v0.5.38

### Breaking Changes 🚨🚨

None

### Bug Fixes 🐞

#### Broadcaster
- [#2709](https://github.com/livepeer/go-livepeer/pull/2709) Add logging for high keyframe interval, reduce log level for discovery loop
- [#2684](https://github.com/livepeer/go-livepeer/pull/2684) Fix transcode success rate metric
- [#2740](https://github.com/livepeer/go-livepeer/pull/2740) Fix incorrect processing of VerificationFreq parameter
- [#2735](https://github.com/livepeer/go-livepeer/pull/2735) Fix EndTranscodingSession() call and potential race
- [#2747](https://github.com/livepeer/go-livepeer/pull/2747) Fixed a transcoding bug that occurred when remote transcoder was removed

#### General
- [#2713](https://github.com/livepeer/go-livepeer/pull/2713) Add support for keyfiles with -ethKeystorePath, update flag descriptions, flagset output to stdout

## v0.5.37

### Breaking Changes 🚨🚨
- Potentially breaking for environments with tight disk space: the docker image is 700 Mb larger now, because of bundled Tensorflow libraries

#### General
- [#2697](https://github.com/livepeer/go-livepeer/pull/2697) Fix backwards compatibility of livepeer_cli with prior livepeer version (@eliteprox)

#### Broadcaster
- [#2709](https://github.com/livepeer/go-livepeer/pull/2709) Add logging for high keyframe interval, reduce log level for discovery loop (@eliteprox)
- [#2684](https://github.com/livepeer/go-livepeer/pull/2684) Fix transcode success rate metric to better account for errors (@mjh1)
- [#357](https://github.com/livepeer/lpms/pull/357) Tune fast verification signature comparison (@cyberj0g)

#### Orchestrator
- [#2707](https://github.com/livepeer/go-livepeer/pull/2707) Fix remote transcoders quietly getting dropped from selection (@stronk-dev)
- [#2710](https://github.com/livepeer/go-livepeer/pull/2710) Fix `Invalid Ticket senderNonce` error when segments are sent from B to O out of order or in parallel (@cyberj0g)

#### Transcoder
- [#2686](https://github.com/livepeer/go-livepeer/pull/2686) Control non-stream specific scene classification with command line args (@cyberj0g)
- [#2705](https://github.com/livepeer/go-livepeer/pull/2705) Fix: standalone transcoder will not exit when orchestrator disconnects or terminates the session (@stronk-dev)

## v0.5.36

#### General
- [#2696](https://github.com/livepeer/go-livepeer/pull/2696) Add Rinkeby network deprecation warning (@leszko)

#### Transcoder
- [#2686](https://github.com/livepeer/go-livepeer/pull/2686) Control non-stream specific scene classification with command line args

#### General

- [#2635](https://github.com/livepeer/go-livepeer/pull/2635) Fix entrypoint path in built docker images (@hjpotter92)
- [#2646](https://github.com/livepeer/go-livepeer/pull/2646) Include HttpIngest and LocalVerify in param table on startup (@yondonfu)

#### Broadcaster

- [#2666](https://github.com/livepeer/go-livepeer/pull/2666) Re-use a session as long as it passes the latency score threshold check (@yondonfu)

#### Orchestrator
- [#2639](https://github.com/livepeer/go-livepeer/pull/2639) Increase IdleTimeout for HTTP connections (@leszko)
- [#2685](https://github.com/livepeer/go-livepeer/pull/2685) Add a log message when sessions are closed by the Broadcaster. Increase transcode loop timeout (@mj1)

## v0.5.35

*October 21 2022*

This release contains a number of node stability and quality of life improvements.

### Breaking Changes 🚨🚨
None

#### General
- [#2616](https://github.com/livepeer/go-livepeer/pull/2616) cmd: Echo explicitly set config values on node start
- [#2583](https://github.com/livepeer/go-livepeer/pull/2583) eth: Set tx GasFeeCap to min(gasPriceEstimate, current GasFeeCap) (@yondonfu)
- [#2586](https://github.com/livepeer/go-livepeer/pull/2586) Broadcaster: Don't pass a nil context into grpc call or it panics (@thomshutt, @cyberj0g)

#### Broadcaster
- [#2573](https://github.com/livepeer/go-livepeer/pull/2573) server: Fix timeout for stream recording background jobs (@victorges)
- [#2586](https://github.com/livepeer/go-livepeer/pull/2586) Refactor RTMP connection object management to prevent race conditions (@cyberj0g)

#### Orchestrator
- [#2591](https://github.com/livepeer/go-livepeer/pull/2591) Return from transcode loop if transcode session is ended by B (@yondonfu)
- [#2592](https://github.com/livepeer/go-livepeer/pull/2592) Enable Orchestrator to set pricing by broadcaster ETH address
- [#2628](https://github.com/livepeer/go-livepeer/pull/2628) Use IdleTimeout to prevent hanging HTTP connections when B does not use O (fix "too many files open" error) (@leszko)

## v0.5.34

*August 16 2022*

This release fixes issues with short segments timing out at the upload stage and with `unsupported block number` errors. It also contains a change to prepare for the upcoming Arbitrum Nitro migration.

### Breaking Changes 🚨🚨
None

#### General
- [#2550](https://github.com/livepeer/go-livepeer/pull/2550) Fix requesting LPT from faucet after the Nitro migration (@leszko)
- [#2563](https://github.com/livepeer/go-livepeer/pull/2563) Fix unsupported block number errors (@yondonfu)

#### Broadcaster
- [#2541](https://github.com/livepeer/go-livepeer/pull/2541) Enforce a minimum timeout for segment upload (@victorges)


## v0.5.33

*July 18 2022*

This release contains optimisations to speed up the block backfill process, a number of fixes for transcoding bugs and a switch to using a lower `avgGasPrice` to prevent dropping streams during gas price spikes.

### Breaking Changes 🚨🚨

None

### Features ⚒

#### General
- [#1333](https://github.com/livepeer/go-livepeer/pull/1333) Display git-sha in startup logging (@emranemran)
- [#2443](https://github.com/livepeer/go-livepeer/pull/2443) Add e2e tests for O configuration and resignation (@red-0ne)
- [#2489](https://github.com/livepeer/go-livepeer/pull/2489) Backfill blocks in batches (@leszko)

#### Broadcaster
- [#2462](https://github.com/livepeer/go-livepeer/pull/2462) cmd: Delete temporary env variable LP_IS_ORCH_TESTER (@leszko)

#### Orchestrator
- [#2465](https://github.com/livepeer/go-livepeer/pull/2465) server: Don't fail to get Transcode Results if Detections header missing (@thomshutt)

#### Transcoder

### Bug Fixes 🐞

- [#2466](https://github.com/livepeer/go-livepeer/pull/2466) bugfix: rendition resolution fix for portrait input videos; Min resolution applied for Nvidia hardware (@AlexKordic)
- [#338](https://github.com/livepeer/go-livepeer/pull/338) lpms: Add exception handling code for importing a binary signature (@oscar_davids)
- [#337](https://github.com/livepeer/go-livepeer/pull/337) lpms: fix the audio missing issue during transcoding (@oscar_davids)

#### CLI
- [#2456](https://github.com/livepeer/go-livepeer/pull/2456) cli: Show O rather than B options when -redeemer flag set (@thomshutt)

#### General

#### Broadcaster

#### Orchestrator
- [#2493](https://github.com/livepeer/go-livepeer/pull/2493) cmd: Fix reward flag (@leszko)
- [#2481](https://github.com/livepeer/go-livepeer/pull/2481) Lower `avgGasPrice` to prevent dropping streams during the gas price spikes (@leszko)

#### Transcoder

## v0.5.32

*June 9 2022*

This release adds new content detection feature for Nvidia Transcoders that can be run if requested by transcoding job.
It also includes some CLI bug fixes.

### Breaking Changes 🚨🚨

None

### Features ⚒

#### General
- [#2429](https://github.com/livepeer/go-livepeer/pull/2429) Binaries are uploaded to gcp cloud storage following the new directory structure (@hjpotter92)
- [#2437](https://github.com/livepeer/go-livepeer/pull/2437) Make CLI flag casing consistent for dataDir flag (@thomshutt)
- [#2440](https://github.com/livepeer/go-livepeer/pull/2440) Create and push `go-livepeer:latest` tagged images to dockerhub (@hjpotter92)

#### Broadcaster
- [#2327](https://github.com/livepeer/go-livepeer/pull/2327) Parallelize handling events in Orchestrator Watcher (@red-0ne)

#### Orchestrator
- [#2290](https://github.com/livepeer/go-livepeer/pull/2290) Allow orchestrator to set max ticket faceValue (@0xB79)

### Bug Fixes 🐞
- [#2339](https://github.com/livepeer/go-livepeer/pull/2339) Fix failing `-nvidia all` flag on VM (@red-0ne)

#### CLI
- [#2438](https://github.com/livepeer/go-livepeer/pull/2438) Add new menu option to gracefully exit livepeer_cli (@emranemran)
- [#2431](https://github.com/livepeer/go-livepeer/pull/2431) New flag to enable content detection on Nvidia Transcoder: `-detectContent`. When set, Transcoder will initialize Tensorflow runtime on each Nvidia GPU, and will run an additional Detector profile, if requested by the transcoding job.(@cyberj0g)

## v0.5.31

*May 23 2022*

This release removes special handling for single frame segments from the previous release that was causing transcoding issues with certain types of content.

It also introduces a number of fixes around making the Stream Tester more reliable and a CLI fix to choose more logical default address.

### Breaking Changes 🚨🚨

None

### Features ⚒

#### General
- [#2383](https://github.com/livepeer/go-livepeer/pull/2383) Add E2E Tests for checking Livepeer on-chain interactions (@leszko)
- [#2223](https://github.com/livepeer/go-livepeer/pull/2223) Refactor `drivers` package as a reusable and more performant lib (@victorges)

#### Broadcaster
- [#2392](https://github.com/livepeer/go-livepeer/pull/2392) Add LP_EXTEND_TIMEOUTS env variable to extend timeouts for Stream Tester (@leszko)
- [#2413](https://github.com/livepeer/go-livepeer/pull/2413) Fix Webhook discovery, refresh pool before getting pool size (@leszko)

### Orchestrator
- [#](https://github.com/livepeer/go-livepeer/pull/)#2423 Revert problematic single segment fix (@thomshutt)

### CLI
- [#2416](https://github.com/livepeer/go-livepeer/pull/2416) Use the O's currently registered Service URI as default address (@emranemran)

## v0.5.30

*May 11 2022*

This release includes support for Netint transcoding hardware, dynamic timeouts for Orchestrator discovery, protection against rounds with a zero block hash and a number of small Orchestrator bug fixes.

### Breaking Changes 🚨🚨

None

### Features ⚒

#### General
- [#2348](https://github.com/livepeer/go-livepeer/pull/2348) Support Netint transcoding hardware (@cyberj0g)
- [#2289](https://github.com/livepeer/go-livepeer/pull/2289) Add timeouts to ETH client (@leszko)
- [#2282](https://github.com/livepeer/go-livepeer/pull/2282) Add checksums and gpg signature support with binary releases. (@hjpotter92)
- [#2344](https://github.com/livepeer/go-livepeer/pull/2344) Use T.TempDir to create temporary test directory (@Juneezee)
- [#2353](https://github.com/livepeer/go-livepeer/pull/2353) Codesign and notarize macOS binaries to be allowed to run without warnings on apple devices (@hjpotter92)
- [#2351](https://github.com/livepeer/go-livepeer/pull/2351) Refactor livepeer.go to enable running Livepeer node from the code (@leszko)
- [#2372](https://github.com/livepeer/go-livepeer/pull/2372) Upload test coverage reports to codecov (@hjpotter92)

#### Broadcaster
- [#2309](https://github.com/livepeer/go-livepeer/pull/2309) Add dynamic timeout for the orchestrator discovery (@leszko)

#### Orchestrator
- [#2362](https://github.com/livepeer/go-livepeer/pull/2362) Backdate tickets by one round if the block hash for the current round is 0 (@leszko)

#### Transcoder

### Bug Fixes 🐞
- [#2355](https://github.com/livepeer/go-livepeer/pull/2355) Fix ZeroSegments error (@AlexKordic)

#### CLI
- [#2345](https://github.com/livepeer/go-livepeer/pull/2345) Improve user feedback when specifying numeric values for some wizard options (@kparkins)

#### General
- [#2299](https://github.com/livepeer/go-livepeer/pull/2299) Split devtool Orchestrator run scripts into versions with/without external transcoder and prevent Transcoder/Broadcaster run scripts from using same CLI port (@thomshutt)
- [#2346](https://github.com/livepeer/go-livepeer/pull/2346) Fix syntax errors in example JSON (@thomshutt)

#### Broadcaster
- [#2291](https://github.com/livepeer/go-livepeer/pull/2291) Calling video comparison to improve the security strength (@oscar-davids)
- [#2326](https://github.com/livepeer/go-livepeer/pull/2326) Split Auth/Webhook functionality into its own file (@thomshutt)
- [#2357](https://github.com/livepeer/go-livepeer/pull/2357) Begin accepting auth header (from Mist) and have it override callback URL values (@thomshutt)
- [#2385](https://github.com/livepeer/go-livepeer/pull/2385) Change verification randomness variable meaning (@thomshutt)

#### Orchestrator
- [#2284](https://github.com/livepeer/go-livepeer/pull/2284) Fix issue with not redeeming tickets by Redeemer (@leszko)
- [#2352](https://github.com/livepeer/go-livepeer/pull/2352) Fix standalone orchestrator not crashing under UnrecoverableError (@leszko)
- [#2359](https://github.com/livepeer/go-livepeer/pull/2359) Fix redeeming tickets with zero block hash (@leszko)
- [#2390](https://github.com/livepeer/go-livepeer/pull/2390) Fix Orchestrator registration error when we receive an empty HTTP body (@leszko)

#### Transcoder
- N/A

## v0.5.29

*February 21 2022*

This release is a fast follow release for v0.5.28 with a few bug fixes including a fix for a nil pointer error when parsing block header logs that could cause the node to crash and a fix for displaying the global LPT supply and participation rate in `livepeer_cli`.

This release also includes a darwin arm64 build and darwin/linux binaries compiled using Go 1.17.6.

### Breaking Changes 🚨🚨

- The `segment_transcoded_appeared_total` and `transcode_latency_seconds` metrics are removed because they were tracked per transcoding profile and the node already tracks the overall version of the metrics
- The `upload_time_seconds` and `discovery_errors_toatl` metrics are tracked per orchestrators instead of per stream

### Features ⚒

#### General

- [#2274](https://github.com/livepeer/go-livepeer/pull/2274) Reduce the number of metrics datapoints (@darkdragon)

#### Transcoder

- [#2259](https://github.com/livepeer/go-livepeer/pull/2259) Improve startup capability test to check for Nvidia encoder session limit and to fail fast if a default capability is not supported (@cyberj0g)

### Bug Fixes 🐞

#### General

- [#2267](https://github.com/livepeer/go-livepeer/pull/2267) Fix nil pointer in the block header logs (@leszko)
- [#2276](https://github.com/livepeer/go-livepeer/pull/2276) Use global total supply instead of L2 supply to calculate participation rate on Arbitrum networks (@leszko)

#### Orchestrator

- [#2266](https://github.com/livepeer/go-livepeer/pull/2266) Fix default reward cut and fee cut in `livepeer_cli` (@leszko)

## v0.5.28

*February 11th 2022*

This release supports connecting to Arbitrum Mainnet using the `-network arbitrum-one-mainnet` flag after the L1 Ethereum block 14207040 which is the block at which [LIP-73 i.e. the Confluence upgrade](https://github.com/livepeer/LIPs/blob/master/LIPs/LIP-73.md#specification) will be activated. Prior to this block, running the node with `-network arbitrum-one-mainnet` will result in a startup error so it is recommended to wait until after block 14207040 to run the node with the `-network arbitrum-one-mainnet` flag. **We strongly encourage all node operators to upgrade to this release so they can connect to Arbitrum Mainnet after the LIP-73 block**.

Additional updates in this release include various improvements to compatibility with Arbitrum networks as well as the initial groundwork for enabling H.265/HEVC encoding/decoding and VP8/VP9 decoding jobs on the network.

### Features ⚒

#### General

- [#2208](https://github.com/livepeer/go-livepeer/pull/2208) Improve Feed PubSub: execute subscribers' blocking operations in separate goroutines (@leszko)
- [#2222](https://github.com/livepeer/go-livepeer/pull/2222) Use L1 block number for Ticket Parameters and Round Initialization (@leszko)
- [#2240](https://github.com/livepeer/go-livepeer/pull/2240) Backfill always from the back last seen in DB (instead of the last round block) (@leszko)
- [#2171](https://github.com/livepeer/go-livepeer/pull/2171) Make transactions compatible with Arbitrum and fix setting max gas price (@leszko)
- [#2252](https://github.com/livepeer/go-livepeer/pull/2252) Use different hardcoded redeemGas when connected to an Arbitrum network (@leszko)
- [#2251](https://github.com/livepeer/go-livepeer/pull/2251) Add fail fast for Arbitrum One Mainnet when LIP-73 has not been activated yet (@leszko)
- [#2254](https://github.com/livepeer/go-livepeer/pull/2254) Add `arbitrum-one-mainnet` network (@leszko)

#### Orchestrator

- [#2253](https://github.com/livepeer/go-livepeer/pull/2253) Redeem tickets only when recipient is active (@leszko)

#### Transcoder

- [#2135](https://github.com/livepeer/go-livepeer/pull/2135) LPMS updates to support H.265/HEVC encoding and decoding as well as VP8/VP9 decoding (@cyberj0g)

### Bug Fixes 🐞

#### Broadcaster

- [#2245](https://github.com/livepeer/go-livepeer/pull/2245) Fast verification fix (@oscar-davids)

## v0.5.27

*February 3rd 2022*

This release fixes a bug with the voting option in `livepeer_cli` that caused the node to crash as well as a few other small updates. **If you need to vote in the recently created [LIP-73 poll](https://explorer.livepeer.org/voting/0xd00a506956896c8dfd5dfed0133f870cbe8e49f2) with your node's keystore based wallet using livepeer_cli you should upgrade to this release**.

### Features ⚒

#### General

- [#2196](https://github.com/livepeer/go-livepeer/pull/2196) Add support for Mist runtime environment (@hjpotter92)
- [#2212](https://github.com/livepeer/go-livepeer/pull/2212) y/n confirmation when sending a transaction via the CLI (@noisersup)
- [#2218](https://github.com/livepeer/go-livepeer/pull/2218) Display http status codes in livepeer_cli (@noisersup)

### Bug Fixes 🐞

#### General

- [#2216](https://github.com/livepeer/go-livepeer/pull/2216) Fix Accept Multiline message on Windows (@leszko)
- [#2227](https://github.com/livepeer/go-livepeer/pull/2227) Fix nil pointer in client.Vote() (@leszko)

## v0.5.26

*January 24th 2022*

This release adds support for a new L2 Arbitrum Rinkeby contract deployment used for the [Confluence](https://github.com/livepeer/LIPs/blob/master/LIPs/LIP-73.md) testnet. **If you are planning on participating in the testnet you should upgrade to this release.**

This release also includes a small update for the node to fail fast if the config file specified is invalid.

### Features ⚒

#### General

- [#2198](https://github.com/livepeer/go-livepeer/pull/2198) Add fail-fast approach in case of incorrect config file (@leszko)
- [#2180](https://github.com/livepeer/go-livepeer/pull/2180) Generate contract bindings related to L2 Arbitrum upgrade (@leszko)
- [#2204](https://github.com/livepeer/go-livepeer/pull/2204) Support both L1 and L2 contract interfaces (@leszko)
- [#2202](https://github.com/livepeer/go-livepeer/pull/2202) Add `arbitrum-one-rinkeby` network (@leszko)

#### Broadcaster

- [#2188](https://github.com/livepeer/go-livepeer/pull/2188) Include pending Os in the discovery mechanism (@leszko)

## v0.5.25

*January 12th 2022*

This release adds support for a new L1 Rinkeby contract deployment used for the [Confluence](https://github.com/livepeer/LIPs/blob/master/LIPs/LIP-73.md) testnet. **If you are planning on participating in the testnet you should upgrade to this release.** This release does not contain any updates for mainnet.

### Breaking Changes 🚨🚨

- The node will no longer connect with the old L1 Rinkeby contract deployment when the `-network rinkeby` flag is used. Instead the node will connect to the new L1 Rinkeby contract deployment when the `-network rinkeby` flag is used using a new hardcoded Controller contract address at [0x9a9827455911a858E55f07911904fACC0D66027E](https://rinkeby.etherscan.io/address/0x9a9827455911a858E55f07911904fACC0D66027E).

#### General

- [#2177](https://github.com/livepeer/go-livepeer/pull/2177) Update hardcoded Rinkeby Eth Controller Address (@leszko)

## v0.5.24

*January 5th 2022*

This is a fast follow patch release for v0.5.24 to fix a bug that caused orchestrators to return errors right after a transaction is submitted resulting in the previously active streams to be re-routed from the orchestrators. **If you are running an orchestrator you should upgrade to this release as soon as possible.**

In order to fix this bug, the feature from v0.5.24 that set the `maxFeePerGas` of transactions to `-maxGasPrice` is temporarily disabled and will be re-enabled in the next release. The node will continue not submitting transactions if the current expected gas price for a transaction exceeds `-maxGasPrice`, but due to the disabling of the aforementioned feature, it is possible for a transaction to be mined at a gas price higher than `-maxGasPrice` if the gas price increases quickly after the node performs its `-maxGasPrice` check. The next release will ensure that a transaction cannot be mined at a gas price higher than `-maxGasPrice`.

### Features ⚒

#### General

- [#2157](https://github.com/livepeer/go-livepeer/pull/2157) Add support for EIP-712 typed data signing in `livepeer_cli` (@yondonfu)

#### Broadcaster

- [#1989](https://github.com/livepeer/go-livepeer/pull/1989) Record realtime ratio metric as a histogram (@victorges)

#### Orchestrator

- [#2146](https://github.com/livepeer/go-livepeer/pull/2146) Allows Os receive payments while pending activation (@leszko)

### Bug Fixes 🐞

#### General

- [#2163](https://github.com/livepeer/go-livepeer/pull/2163) Fix Session crashes right after the O sends a tx, revert #2111 Ensure `maxFeePerGas` in all transactions never exceed `-masGasPrice` (@leszko)

## v0.5.23

*December 20th 2021*

This release includes an important fix for a memory leak that can be triggered when using the CUDA MPEG-7 video signature capability (when using a Nvidia GPU), which is used for [fast verification](https://forum.livepeer.org/t/transcoding-verification-improvements-fast-full-verification/1499), for orchestrators/transcoders. **If you are running an orchestrator or transcoder you should upgrade to this release as soon as possible.**

Additional highlights of this release:

- Support for running fast verification on broadcasters
- Support for configuring `livepeer` using a file. See [the docs](https://livepeer.org/docs/installation/configuring-livepeer) for instructions on using a configuration file
- An improvement to the failover behavior in split orchestrator + transcoder setups in the scenario where a transcoder crashes
- Ensure that the the fee per gas for a transaction never exceeds the value set for `-maxGasPrice`

Thanks to everyone that submitted bug reports and assisted in testing!

### Features ⚒

#### General

- [#2114](https://github.com/livepeer/go-livepeer/pull/2114) Add option to repeat the benchmarking process with `livepeer_bench` (@jailuthra)
- [#2111](https://github.com/livepeer/go-livepeer/pull/2111) Ensure `maxFeePerGas` in all transactions never exceed `-masGasPrice` defined by the user (@leszko)
- [#2126](https://github.com/livepeer/go-livepeer/pull/2126) Round up bumped gas price for the replacement transactions (@leszko)
- [#2051](https://github.com/livepeer/go-livepeer/pull/2051) Removes HTTP and HTTPS protocols from FFmpeg build (@darkdarkdragon)
- [#2127](https://github.com/livepeer/go-livepeer/pull/2127) Add warnings to ETH/LPT accounts in Livepeer CLI (@leszko)
- [#2121](https://github.com/livepeer/go-livepeer/pull/2121) Add contextual logging (@darkdarkdragon)
- [#2137](https://github.com/livepeer/go-livepeer/pull/2137) Support config file (@leszko)

#### Broadcaster

- [#2086](https://github.com/livepeer/go-livepeer/pull/2086) Add support for fast verification (@jailuthra @darkdragon)
- [#2085](https://github.com/livepeer/go-livepeer/pull/2085) Set max refresh sessions threshold to 8 (@yondonfu)
- [#2083](https://github.com/livepeer/go-livepeer/pull/2083) Return 422 to the push client after max retry attempts for a segment (@jailuthra)
- [#2022](https://github.com/livepeer/go-livepeer/pull/2022) Randomize selection of orchestrators in untrusted pool at a random frequency (@yondonfu)
- [#2100](https://github.com/livepeer/go-livepeer/pull/2100) Check verified session first while choosing the result from multiple untrusted sessions (@leszko)
- [#2103](https://github.com/livepeer/go-livepeer/pull/2103) Suspend sessions that did not pass p-hash verification (@leszko)
- [#2110](https://github.com/livepeer/go-livepeer/pull/2110) Transparently support HTTP/2 for segment requests while allowing HTTP/1 via GODEBUG runtime flags (@yondonfu)
- [#2124](https://github.com/livepeer/go-livepeer/pull/2124) Do not retry transcoding if HTTP client closed/canceled the connection (@leszko)
- [#2122](https://github.com/livepeer/go-livepeer/pull/2122) Add the upload segment timeout to improve failing fast (@leszko)

#### Transcoder

- [#2094](https://github.com/livepeer/go-livepeer/pull/2094) Gracefully notify orchestrator in case of a panic in transcoder (@leszko)

### Bug Fixes 🐞

#### Broadcaster

- [#2067](https://github.com/livepeer/go-livepeer/pull/2067) Add safety checks in selectOrchestrator for auth token and ticket params in OrchestratorInfo (@yondonfu)

#### Transcoder

- [#2108](https://github.com/livepeer/go-livepeer/pull/2108) Fix memory leak in CUDA MPEG-7 video signature filter (@oscar-davids)

## v0.5.22

*October 28 2021*

This release includes an important MPEG-7 video signature capability fix, which is used for [fast verification](https://forum.livepeer.org/t/transcoding-verification-improvements-fast-full-verification/1499), for orchestrators/transcoders running on Windows. **If you are running an orchestrator or transcoder on Windows you should upgrade to this release as soon as possible.**

Additional highlights of this release:

- Support for EIP-1559 (otherwise known as type 2) Ethereum transactions which results in more predictable transaction confirmation times, reduces the chance of stuck pending transactions and avoids overpaying in gas fees. If you are interested in additional details on the implications of EIP-1559 transactions refer to this [resource](https://hackmd.io/@timbeiko/1559-resources).
- An improvement in ticket parameter generation for orchestrators to prevent short lived gas price spikes on the Ethereum network from disrupting streams.
- The node will automatically detect if the GPU enters an unrecoverable state and crash. The reason for crashing upon detecting an unrecoverable GPU state is that no transcoding will
  be possible in this scenario until the node is restarted. We recommend node operators to setup a process for monitoring if their node is still up and starting the node if it has crashed. For reference, a bash script similar to [this one](https://gist.github.com/jailuthra/03c3d65d0bbff457cae8f9a14b4c04b7) can be used to automate restarts of the node in the event of a crash.

Thanks to everyone that submitted bug reports and assisted in testing!

### Features ⚒

#### General

- [#2013](https://github.com/livepeer/go-livepeer/pull/2013) Add support for EIP-1559 transactions (@yondonfu)
- [#2073](https://github.com/livepeer/go-livepeer/pull/2073) Make filtering orchestrators in the DB that haven't been updated in last day optional (@yondonfu)

### Bug Fixes 🐞

#### Broadcaster

- [#2075](https://github.com/livepeer/go-livepeer/pull/2075) Check if track is found in when serving recordings request (@darkdarkdragon)

#### Orchestrator

- [#2071](https://github.com/livepeer/go-livepeer/pull/2071) Update tx cost check when generating ticket params to be more resistant to gas price spikes (@yondonfu)

#### Transcoder

- [#2057](https://github.com/livepeer/go-livepeer/pull/2057) Prevent stuck sessions by crashing on unrecoverable CUDA errors (@jailuthra)
- [#2066](https://github.com/livepeer/go-livepeer/pull/2066) Fix filename parsing errors for signature_cuda filter on Windows (@jailuthra)

## v0.5.21

*September 29 2021*

This release includes a new orchestrator/transcoder MPEG-7 video signature capability which is required for broadcasters to run transcoding verification. The MPEG-7 video signatures are used as perceptual hashes in a "fast verification" algorithm that is described in the [Fast and Full Transcoding Verification design](https://forum.livepeer.org/t/transcoding-verification-improvements-fast-full-verification/1499). If a CPU is used for transcoding, the video signature computation will occur on the CPU. If a GPU is used for transcoding (i.e. the `-nvidia` flag is used), the video signature computation will occur on the GPU. Orchestrators and transcoders should upgrade to this release as soon as possible to ensure that they will be able to continue serving broadcasters that run fast verification (the complete broadcaster implementation will be shipped at a later date).

Orchestrators and transcoders should also make note of the breaking change in this release that drops support for Nvidia GPUs from the Kepler series.

Thanks to everyone that submitted bug reports and assisted in testing!

### Breaking Changes 🚨🚨

- [#2027](https://github.com/livepeer/go-livepeer/pull/2027) Nvidia GPUs belonging to the Kepler series (GeForce 600, 700 series) and older, are no longer supported by go-livepeer. Cuda 11 is needed for newer GPUs, which [only supports the Maxwell series](https://arnon.dk/matching-sm-architectures-arch-and-gencode-for-various-nvidia-cards/) or newer.

### Features ⚒

#### General

- [#2041](https://github.com/livepeer/go-livepeer/pull/2041) Update help description for `{min,max}GasPrice` (@Strykar)

#### Transcoder

- [#2036](https://github.com/livepeer/go-livepeer/pull/2036) Generate mpeg7 perceptual hashes for fast verification (@jailuthra)

### Bug Fixes 🐞

#### Transcoder

- [#2027](https://github.com/livepeer/go-livepeer/pull/2027) Fix a memleak in the (experimental) AI content detection filter (@cyberj0g)

## v0.5.20

*September 10 2021*

This release includes a few important bug fixes including:

- A fix for winning tickets incorrectly being marked as redeemed in the node's database even though they have not been redeemed yet
- A fix for node crashes due to the submission of a replacement transaction when a max gas price configured
- A fix for increased gas usage of reward transactions if an orchestrator did not receive stake updates in the previous round (via a reward call or delegation)

Additionally, this release includes a new `-autoAdjustPrice` flag that allows orchestrators to enable/disable automatic price adjustments based on the overhead for ticket redemption (which is determined by the current ETH gas price). Orchestrators can disable automatic price adjustments by setting `-autoAdjustPrice=false` (default is true) to ensure that they advertise a constant price to broadcasters avoiding the scenario where they lose all jobs due to a gas price spike that causes their advertised price to exceed the max price set by broadcasters. If orchestrators disable automatic price adjustments then it is recommended to also use the `-maxGasPrice` flag to set a maximum gas price for transactions to wait to redeem tickets during lower gas prices periods.

Thanks to everyone that submitted bug reports and assisted in testing!

### Features ⚒

#### Broadcaster

- [#1946](https://github.com/livepeer/go-livepeer/pull/1946) Send transcoding stream health events to a metadata queue (@victorges)

#### Orchestrator

- [#2025](https://github.com/livepeer/go-livepeer/pull/2025) Add -autoAdjustPrice flag to enable/disable automatic price adjustments (@yondonfu)

#### Transcoder

- [#1979](https://github.com/livepeer/go-livepeer/pull/1979) Upgrade to ffmpeg v4.4 and improved API for (experimental) AI tasks (@jailuthra)

### Bug Fixes 🐞

- [#1992](https://github.com/livepeer/go-livepeer/pull/1992) Eliminate data races in mediaserver.go (@darkdarkdragon)
- [#2011](https://github.com/livepeer/go-livepeer/pull/2011) Configurable delay between sessions in livepeer_bench (@jailuthra)

#### General

- [#2001](https://github.com/livepeer/go-livepeer/pull/2001) Fix max gas price nil pointer error in replace transaction (@kyriediculous)

#### Broadcaster

- [#2026](https://github.com/livepeer/go-livepeer/pull/2026) Run signature verification even without a verification policy specified (@yondonfu)

#### Orchestrator

- [#2018](https://github.com/livepeer/go-livepeer/pull/2018) Only mark tickets for failed transactions as redeemed when there is an error checking the transaction (@yondonfu)
- [#2029](https://github.com/livepeer/go-livepeer/pull/2029) Fix active total stake calculation when generating hints for rewardWithHint (@yondonfu)

## v0.5.19

*August 10 2021*

This release includes another gas price monitoring fix to address additional cases where Ethereum JSON-RPC providers occasionally return really low gas prices for the `eth_gasPrice` RPC call, automatic replacements for pending transactions that timeout, fixes for broadcaster stream recording, support for downloading stream recordings as mp4 files as well as variety of other bug fixes and enhancements.

In addition to the gas price monitoring fix and support for automatic replacements for pending transactions that timeout, a few additional configuration options are introduced to give node operators more control over gas prices and transactions:

- `-maxTransactionReplacements <INTEGER>` can be used to specify the max number of times to replace a pending transaction that times out. The default value is 1.
- `-txTimeout <DURATION>` can be used to specify the timeout duration for a pending transaction after which a replacement transaction would be submitted. The default value is 5m.
- `-minGasPrice <INTEGER>` can be used to specify the minimum gas price (in wei) to use for transactions. The default is 1 gwei on mainnet.

More information about these new flags is accessible via `livepeer -help`.

The default value for the `-maxTicketEV` flag for broadcasters has been updated to 3000 gwei based on the default value of 1000 gwei for the `-ticketEV` flag for orchestrators which is safer for broadcasters. For more information on these default values, refer to the [payment docs for video developers](https://livepeer.org/docs/video-developers/core-concepts/payments) and the [payment docs for video miners](https://livepeer.org/docs/video-miners/core-concepts/payments).

An experimental version of a deep neural network (DNN) based scene classification capability is mentioned in the changelog, but please note that while this is the first step towards enabling this capability on the network for video miners, this feature is **NOT** yet usable on the network today and is undergoing rapid development.

Thanks to everyone that submitted bug reports and assisted in testing!

### Features ⚒

#### General

- [#1911](https://github.com/livepeer/go-livepeer/pull/1911) [Experimental] Enable scene classification for Adult/Soccer (@jailuthra, @yondonfu)
- [#1915](https://github.com/livepeer/go-livepeer/pull/1915) Use gas price monitor for gas price suggestions for all Ethereum transactions (@kyriediculous)
- [#1930](https://github.com/livepeer/go-livepeer/pull/1930) Support custom minimum gas price (@yondonfu)
- [#1942](https://github.com/livepeer/go-livepeer/pull/1942) Log min and max gas price when monitoring is enabled (@kyriediculous)
- [#1923](https://github.com/livepeer/go-livepeer/pull/1923) Use a transaction manager with better transaction handling and optional replacement transactions instead of the default JSON-RPC client (@kyriediculous)
- [#1954](https://github.com/livepeer/go-livepeer/pull/1954) Add signer to Ethereum client config (@kyriediculous)

#### Broadcaster

- [#1877](https://github.com/livepeer/go-livepeer/pull/1877) Refresh TicketParams for the active session before expiry (@kyriediculous)
- [#1879](https://github.com/livepeer/go-livepeer/pull/1879) Add mp4 download of recorded stream (@darkdarkdragon)
- [#1899](https://github.com/livepeer/go-livepeer/pull/1899) Record million pixels processed metric (@yondonfu)
- [#1888](https://github.com/livepeer/go-livepeer/pull/1888) Should not save (when recording) segments with zero video frames (@darkdarkdragon)
- [#1908](https://github.com/livepeer/go-livepeer/pull/1908) Prevent Broadcaster from sending low face value PM tickets (@kyriediculous)
- [#1934](https://github.com/livepeer/go-livepeer/pull/1934) http push: return 422 for non-retryable errors (@darkdarkdragon)
- [#1943](https://github.com/livepeer/go-livepeer/pull/1943) log maximum transcoding price when monitoring is enabled (@kyriediculous)
- [#1950](https://github.com/livepeer/go-livepeer/pull/1950) Fix extremely long delay before uploaded segment gets transcoded (@darkdarkdragon)
- [#1933](https://github.com/livepeer/go-livepeer/pull/1933) server: Return 0 video frame segments unchanged (@darkdarkdragon)
- [#1932](https://github.com/livepeer/go-livepeer/pull/1932) Serialize writes of JSON playlist (@darkdarkdragon)
- [#1985](https://github.com/livepeer/go-livepeer/pull/1985) Set default -maxTicketEV to 3000 gwei (@yondonfu)

#### Orchestrator

- [#1931](https://github.com/livepeer/go-livepeer/pull/1931) Bump ticket redemption gas estimate to 350k to account for occasional higher gas usage (@yondonfu)

#### Transcoder

- [#1944](https://github.com/livepeer/go-livepeer/pull/1944) Enable B-frames in Nvidia encoder output (@jailuthra)

### Bug Fixes 🐞

#### General

- [#1968](https://github.com/livepeer/go-livepeer/pull/1968) Fix nil pointer error in embedded transaction receipts returned from the TransactionManager (@kyriediculous)
- [#1977](https://github.com/livepeer/go-livepeer/pull/1977) Fix error logging for failed replacement transaction (@yondonfu)

## v0.5.18

*May 18 2021*

This release includes an important gas price monitoring fix that addresses cases where Ethereum JSON-RPC providers occasionally return really low gas prices for the `eth_gasPrice` RPC call, reductions in the gas cost for staking actions (under certain circumstances) using `livepeer_cli`  and improvements to split orchestrator and transcoder setups that help remote transcoders retain streams. We strongly recommend all orchestrator and transcoder operators to upgrade to this version as soon as possible to access this latest set of bug fixes and improvements.

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