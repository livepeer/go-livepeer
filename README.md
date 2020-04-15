[![Build Status](https://circleci.com/gh/livepeer/go-livepeer.svg?style=shield&circle-token=e33534f6f4e2a6af19bb1596d7b72767a246cbab)](https://circleci.com/gh/livepeer/go-livepeer/tree/master)
[![Go Report Card](https://goreportcard.com/badge/github.com/livepeer/go-livepeer)](https://goreportcard.com/report/github.com/livepeer/go-livepeer)
[![Gitter](https://img.shields.io/gitter/room/nwjs/nw.js.svg)](https://gitter.im/livepeer/Lobby)

# go-livepeer
[Livepeer](https://livepeer.org) is a live video streaming network protocol that is fully decentralized, highly scalable, crypto token incentivized, and results in a solution which is cheaper to an app developer or broadcaster than using traditional centralized live video solutions.  go-livepeer is a golang implementation of the protocol.

Building and running this node allows you to:

* Create a local Livepeer Network, or join the existing Livepeer test network.
* Broadcast a live stream into the network.
* Request that your stream be transcoded into multiple formats.
* Consume a live stream from the network.

For full documentation and a project overview, go to
[Livepeer Documentation](http://livepeer.readthedocs.io/en/latest/index.html) or [Livepeer Wiki](https://github.com/livepeer/wiki/wiki)

## Installing Livepeer

### Easiest Option: Download executables
The easiest way to install Livepeer is by downloading the `livepeer` and `livepeer_cli` executables from the [release page on Github](https://github.com/livepeer/go-livepeer/releases).

1. Download the packages for your OS - darwin for Macs and linux for linux.
2. Untar them and optionally move the executables to your PATH.

Alternative Livepeer installation options are also available:
* [Build from Source](doc/install.md#source)
* [Docker](doc/install.md/#docker)
* [Private Testnet](doc/install.md/#testnet)

#### Building on Windows

Building on Windows is possible using MSYS2 and mingw64. For convenience, there is an `.\windows-build.ps1` script
that initializes an appropriate MSYS2 environment in the `.\msys2` directory. Steps to use:

1. Install [Chocolatey](https://chocolatey.org/)
2. Run `.\windows-build.ps1`
3. Wait around 20 minutes

If you run into problems, it's recommended to delete your local `.\msys2` directory and re-run the script to rebuild
everything from scratch.

## Running Livepeer

### Quick start
- Make sure you have successfully gone through the steps in 'Installing Livepeer' and 'Additional Dependencies'.

- Run `./livepeer -broadcaster -network rinkeby -ethUrl <ETHEREUM_RPC_URL>`.
  * `<ETHEREUM_RPC_URL>` is the JSON-RPC URL of an Ethereum node

- Run `./livepeer_cli`.
  * You should see a wizard launch in the command line.
  * The wizard should print out `Account Eth Addr`, `Token balance`, and `Eth balance`

- Get some test eth for the Rinkeby faucet. Make sure to use the Eth account address from above. Remember to add 0x as a prefix to address, if not present.
  * You can check that the request is successful by going to `livepeer_cli` and selecting `Get node status`. You should see a positive `Eth balance`.

- Now get some test Livepeer tokens. Pick `Get test Livepeer Token`.
  * You can check that the request is successful by going to `livepeer_cli` and selecting `Get node status`. You should see your `Token balance` go up.

- You should have some test Eth and test Livepeer tokens now.  If that's the case, you are ready to broadcast.


### Broadcasting

For full details, read the [Broadcasting guide](http://livepeer.readthedocs.io/en/latest/broadcasting.html).

Sometimes you want to use third-party broadcasting software, especially if you are running the software on Windows or Linux. Livepeer can take any RTMP stream as input, so you can use other popular streaming software to create the video stream. We recommend [OBS](https://obsproject.com/download) or [ffmpeg](https://www.ffmpeg.org/).

By default, the RTMP port is 1935.  For example, if you are using OSX with ffmpeg, run

`ffmpeg -f avfoundation -framerate 30 -pixel_format uyvy422 -i "0:0" -vcodec libx264 -tune zerolatency -b 1000k -x264-params keyint=60:min-keyint=60 -acodec aac -ac 1 -b:a 96k -f flv rtmp://localhost:1935/movie`

Similarly, you can use OBS, and change the Settings->Stream->URL to `rtmp://localhost:1935/movie` , along with the keyframe interval to 4 seconds, via `Settings -> Output -> Output Mode (Advanced) -> Streaming tab -> Keyframe Interval 4`.

If the broadcast is successful, you should be able to access the stream at:

`http://localhost:8935/stream/movie.m3u8`

where the "movie" stream name is taken from the path in the RTMP URL.

See the documentation on [RTMP ingest](doc/ingest.md) or [HTTP ingest](doc/ingest.md#http-push) for more details.

#### Authentication of incoming streams

Incoming streams can be authenticated using a webhook. More details in the [webhook docs](doc/rtmpwebhookauth.md).


### Streaming

You can use tools like `ffplay` or `VLC` to view the stream.

For example, after you start streaming to `rtmp://localhost/movie`, you can view the stream by running:

`ffplay http://localhost:8935/stream/movie.m3u8`

Note that the default HTTP port or playback (8935) is different from the CLI API port (7935) that is used for node management and diagnostics!

### Using Amazon S3 for storing stream's data

You can use S3 to store source and transcoded data.
For that livepeer should be run like this `livepeer -s3bucket region/bucket -s3creds accessKey/accessKeySecret`. Stream's data will be saved into directory `MANIFESTID`, where MANIFESTID - id of the manifest associated with stream. In this directory will be saved all the segments data, plus manifest, named `MANIFESTID_full.m3u8`.
Livepeer node doesn't do any storage management, it only saves data and never deletes it.

### Becoming an Orchestrator

We'll walk through the steps of becoming a transcoder on the test network.  To learn more about the transcoder, refer to the [Livepeer whitepaper](https://github.com/livepeer/wiki/blob/master/WHITEPAPER.md) and the [Transcoding guide](http://livepeer.readthedocs.io/en/latest/transcoding.html).

- `livepeer -orchestrator -transcoder -network rinkeby -ethUrl <ETHEREUM_RPC_URL>` to start the node as an orchestrator with an attached local transcoder .

- `livepeer_cli` - make sure you have test ether and test Livepeer token.  Refer to the Quick Start section for getting test ether and test tokens.

- You should see the Transcoder Status as "Not Registered".

- Pick "Become a transcoder" in the wizard.  Make sure to choose "bond to yourself".  Follow the rest of the prompts, including confirming the transcoder's public IP and port on the blockchain. If Successful, you should see the Transcoder Status change to "Registered"

- Wait for the next round to start, and your transcoder will become active.

- If running on Rinkeby or mainnet, ensure your orchestrator is *publicly accessible* in order to receive jobs from broadcasters. The only port that is required to be public is the one that was set during the transcoder registration step (default 8935).

### Standalone Orchestrators

Orchestrators can be run in standalone mode without an attached transcoder. Standalone transcoders will need to connect to this orchestrator in order for the orchestrator to process jobs.

- `livepeer -network rinkeby -ethUrl <ETHEREUM_RPC_URL> -orchestrator -orchSecret asdf`

The orchSecret is a shared secret used to authenticate remote transcoders. It can be any arbitrary string.

### Standalone Transcoders

A standalone transcoder can be run which connects to a remote orchestrator. The orchestrator will send transcoding tasks to this transcoder as segments come in.

- `livepeer -transcoder -orchAddr 127.0.0.1:8935 -orchSecret asdf`

### GPU Transcoding

GPU transcoding on NVIDIA is supported; see the [GPU documentation](doc/gpu.md) for usage details.

### Verification

Experimental verification of video using the Epic Labs classifier can be enabled with the `-verifierUrl` flag. Pass in the address of the verifier API:

- `livepeer -broadcaster -verifierUrl http://localhost:5000/verify -verifierPath /path/to/verifier`

Refer to the [classifier documentation](https://github.com/livepeer/verification-classifier) for more details on getting the classifier API installed and running.

### CLI endpoint spec

The Livepeer node exposes a HTTP interface for monitoring and managing the node. Details on the available endpoints are [here](doc/httpcli.md).

## Contribution
Thank you for your interest in contributing to the core software of Livepeer.

There are many ways to contribute to the Livepeer community. To see the project overview, head to our [Wiki overview page](https://github.com/livepeer/wiki/wiki/Project-Overview). The best way to get started is to reach out to us directly via our [discord channel](https://discord.gg/q6XrfwN).
