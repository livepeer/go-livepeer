[![Build Status](https://circleci.com/gh/livepeer/go-livepeer.svg?style=shield&circle-token=e33534f6f4e2a6af19bb1596d7b72767a246cbab)](https://circleci.com/gh/livepeer/go-livepeer/tree/master)


# go-livepeer
[Livepeer](https://livepeer.org) is a live video streaming network protocol that is fully decentralized, highly scalable, crypto token incentivized, and results in a solution which is cheaper to an app developer or broadcaster than using traditional centralized live video solutions.  go-livepeer is a golang implementation of the protocol.

Building and running this node allows you to:

* Create a local Livepeer Network, or join the existing Livepeer test network.
* Broadcast a live stream into the network.
* Request that your stream be transcoded into multiple formats.
* Consume a live stream from the network.

For full documentation and a project overview, go to
[Livepeer Documentation](https://github.com/livepeer/wiki/wiki)

## Installing Livepeer
### Option 1: Download executables
The easiest way to install Livepeer is by downloading it from the [release page on Github](https://github.com/livepeer/go-livepeer/releases).  Pick the appropriate platform and the latest version.

### Option 2: Build from source
If you have never set up your Go programming environment, do so according to Go's [Getting Started Guide](https://golang.org/doc/install).

You can build Livepeer from scratch.  If you already have the code, you can simply run `go build ./cmd/livepeer/livepeer.go` from the project root directory.

You can also fetch and build the `livepeer` node using `go get github.com/livepeer/go-livepeer/cmd/livepeer`. It should be built and put in your $GOPATH/bin.

## Additional Dependencies and Setup

### ffmpeg
The current version of Livepeer requires [ffmpeg](https://www.ffmpeg.org/).

On OSX, run
`brew install ffmpeg --with-ffplay`

or on Debian based Linux
`apt-get install ffmpeg`

### geth
Livepeer requires a local Ethereum node. To set it up, follow the [Ethereum Installation Guide](https://github.com/ethereum/go-ethereum/wiki/Building-Ethereum)  (We have tested with 1.6.7)

### Livepeer private Ethereum testnet
Livepeer is currently only running on a private Ethereum testnet.
1. Create a geth data directory. For example, `mkdir ~/.lpGeth`. We recommend creating a new directory even if you already have one, so the Livepeer testing data will be stored separately.
2. Download the genesis json [lptestnet.json](http://eth-testnet.livepeer.org/lptestnet.json) (depending on your geth version, it may complain about 'genesis.number'. If encountered, delete the "number" field in the json.)
3. Run `geth init` to initialize your local geth node with testnet genesis block.  For example: `geth --datadir ~/.lpGeth init lptestnet.json`
4. Start `geth` with the network id `858585` and the Livepeer testnet bootnode. For example: `geth --datadir ~/.lpGeth --networkid 858585 --bootnodes "enode://080ebca2373d15762c29ca8d85ddc848f10a7ffc745f7110cacba4694728325d645292cb512d7168323bd0af1650fca825ff54c8dba20aec8878498fae3ff3c6@18.221.67.74:30303"`

Now the geth node should be running, and it should soon start downloading blocks.

## Running Livepeer

### Quick start
- Make sure you have done all the setup steps. (Have `ffmpeg` installed, have `geth` running on the Livepeer private testnet)

- Run `livepeer -testnet` to start Livepeer. Make sure you have the `livepeer` executable.  It can be downloaded from the [releases page](https://github.com/livepeer/go-livepeer/releases) (you should download the packages for your OS - darwin for Macs and linux for linux.  You should also rename them to `livepeer` and `livepeer_cli`, and make sure they have executable permissions).

- Run `livepeer_cli`. You should see a wizard launch in the command line. Make sure you installed the `livepeer_cli` executable.  It can be downloaded from the [releases page](https://github.com/livepeer/go-livepeer/releases).

- Get some test eth from the eth faucet from [http://eth-testnet.livepeer.org/](http://eth-testnet.livepeer.org/). Make sure to use the "Account Eth Addr".  You should see your Eth balance go up.

- Now get some test Livepeer tokens. Pick 'Get test Livepeer Token'. If the request is successful, you should see your `Token balance` go up.

- To broadcast, run `livepeer_cli` and pick 'Broadcast Video'.  You should see your webcam becoming active and a streamID printed on the screen.

- To see the video, run `./livepeer_cli` again, this time pick 'Stream Video'.

You should see a video stream broadcasted from your webcam.  It may feel a little delayed - that's normal. Video live streaming typically has latency from 15 seconds to a few minutes. We are working on solutions to lower this latency, using techniques like WebRTC, peer-to-peer streaming, and crypto-incentives.

### Broadcasting

Sometimes you want to use third-party broadcasting software, especially if you are running the software on Windows or Linux. Livepeer can take any RTMP stream as input, so you can use other popular streaming software to create the video stream. We recommend [OBS](https://obsproject.com/download) or [ffmpeg](https://www.ffmpeg.org/).

By default, the RTMP port is 1935.  For example, if you are using OSX with ffmpeg, run

`ffmpeg -f avfoundation -framerate 30 -pixel_format uyvy422 -i "0:0" -vcodec libx264 -tune zerolatency -b 1000k -x264-params keyint=60:min-keyint=60 -acodec aac -ac 1 -b:a 96k -f flv rtmp://localhost:1935/movie`

Similarly, you can use OBS, and change the setting->stream->URL to `rtmp://localhost:1935/movie`

If the broadcast is successful, you should be able to get a streamID by querying the local node:

`curl http://localhost:8935/streamID`

### Streaming

Sometimes the stream tool doesn't work.  You can use tools like `ffplay` to view the stream.

For example, after you get the streamID, you can view the stream by running:

`ffplay http://localhost:8935/stream/{streamID}.m3u8`

### Becoming a Transcoder

We'll walk through the steps of becoming a transcoder on the test network.  To learn more about the transcoder, refer to the [Livepeer whitepaper](https://github.com/livepeer/wiki/blob/master/WHITEPAPER.md)

- `livepeer --testnet --transcoder` to start the node as a transcoder.

- `livepeer_cli` - make sure you have test ether and test Livepeer token.  Refer to the Quick Start section for getting test ether and test tokens.

- You should see the Transcoder Status as "Not Registered".

- Pick "Become a transcoder" in the wizard.  Make sure to choose "bond to yourself".  If Successful, you should see the Transcoder Status change to "Registered"

- Wait for the next round to start, and your transcoder will become active.


## Contribution
Thank you for your interest in contributing to the core software of Livepeer.

There are many ways to contribute to the Livepeer community. To see the project overview, head to our [Wiki overview page](https://github.com/livepeer/wiki/wiki/Project-Overview). The best way to get started is to reach out to us directly via our [gitter channel](https://gitter.im/livepeer/Lobby).
