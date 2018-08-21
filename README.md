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
### Option 1: Download executables
The easiest way to install Livepeer is by downloading the `livepeer` and `livepeer_cli` executables from the [release page on Github](https://github.com/livepeer/go-livepeer/releases). 

1. Download the packages for your OS - darwin for Macs and linux for linux.
2. Untar them and optionally move the executables to your PATH.

### Option 2: Build from source
You can also build the executables from scratch.

1. If you have never set up your Go programming environment, do so according to Go's [Getting Started Guide](https://golang.org/doc/install).

2. You can fetch the code running `go get github.com/livepeer/go-livepeer/cmd/livepeer` in terminal.

3. You need to install `ffmpeg` as a dependency.  Run `./install_ffmpeg.sh`.  This will install the dependencies in `~/compiled`.  You need to have `pkg-config` installed.

4. You can now run `PKG_CONFIG_PATH=~/compiled/lib/pkgconfig go build ./cmd/livepeer/livepeer.go` from the project root directory. To get latest version, `git pull` from the project root directory.

5. To run tests in locall run `./test.sh`, to run in docker container run `./test_docker.sh`

## Running Livepeer

### Quick start
- Make sure you have successfully gone through the steps in 'Installing Livepeer' and 'Additional Dependencies'.

- Run `./livepeer -rinkeby`.

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

If the broadcast is successful, you should be able to get a streamID by querying the local node's CLI API:

`curl http://localhost:8935/manifestID`

### Streaming

You can use tools like `ffplay` or `VLC` to view the stream.

For example, after you get the streamID, you can view the stream by running:

`ffplay http://localhost:8935/stream/{manifestID}.m3u8`

Note that the default HTTP port or playback (8935) is different from the CLI API port (7935) that is used for node management and diagnostics!

### Becoming a Transcoder

We'll walk through the steps of becoming a transcoder on the test network.  To learn more about the transcoder, refer to the [Livepeer whitepaper](https://github.com/livepeer/wiki/blob/master/WHITEPAPER.md) and the [Transcoding guide](http://livepeer.readthedocs.io/en/latest/transcoding.html).

- `livepeer --rinkeby --transcoder` to start the node as a transcoder.

- `livepeer_cli` - make sure you have test ether and test Livepeer token.  Refer to the Quick Start section for getting test ether and test tokens.

- You should see the Transcoder Status as "Not Registered".

- Pick "Become a transcoder" in the wizard.  Make sure to choose "bond to yourself".  Follow the rest of the prompts, including confirming the transcoder's public IP and port on the blockchain. If Successful, you should see the Transcoder Status change to "Registered"

- Wait for the next round to start, and your transcoder will become active.

- If running on Rinkeby or mainnet, ensure your transcoder is *publicly accessible* in order to receive jobs from broadcasters. The only port that is required to be public is the one that was set during the transcoder registration step (default 8935).

## Contribution
Thank you for your interest in contributing to the core software of Livepeer.

There are many ways to contribute to the Livepeer community. To see the project overview, head to our [Wiki overview page](https://github.com/livepeer/wiki/wiki/Project-Overview). The best way to get started is to reach out to us directly via our [discord channel](https://discord.gg/q6XrfwN).
