# Changelog

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

This release supports connecting to Arbitrum Mainnet using the `-network arbitrum-one-mainnet` flag after the L1 Ethereum block 14207040 which is the block at which [LIP-73 i.e. the Confluence upgrade](https://github.com/livepeer/LIPs/blob/master/LIPs/LIP-73.md#specification) will be activated. Prior to this block, running the node wtih `-network arbitrum-one-mainnet` will result in a startup error so it is recommended to wait until after block 14207040 to run the node with the `-network arbitrum-one-mainnet` flag. **We strongly encourage all node operators to upgrade to this release so they can connect to Arbitrum Mainnet after the LIP-73 block**.

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

This release includes another gas price monitoring fix to address additional cases where Ethereum JSON-RPC providers occassionally return really low gas prices for the `eth_gasPrice` RPC call, automatic replacements for pending transactions that timeout, fixes for broadcaster stream recording, support for downloading stream recordings as mp4 files as well as variety of other bug fixes and enhancements.

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