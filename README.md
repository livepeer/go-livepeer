# golp
[Livepeer](https://livepeer.org) is a decentralized live streaming broadcast platform.  On the Livepeer network, video transcoding and delivery happens in a peer-to-peer fashion, and participants who contribute to the network will be compensated via a crypto token called the Livepeer Token.

Building and running this node allows you to:

* Create a local Livepeer Network, or join the existing Livepeer POC network.
* Broadcast a live stream into the network.
* Request that your stream be transcoded into multiple formats.
* Consume a live stream from the network.

For full documentation and a project overview, go to 
[Livepeer Documentation](https://github.com/livepeer/wiki/wiki)

## Installation
The easiest way to install Livepeer is by downloading it from the [release page on Github](https://github.com/livepeer/golp/releases).  Pick the appropriate platform and the latest version.

## Build
You can build Livepeer from scratch.  Livepeer is built with Go, and the dependencies should all be vendored.  You can simply run `go build ./cmd/livepeer/livepeer.go` from the project root directory.

If you have never set up your Go programming environment, do so according to Go's [Getting Started Guide](https://golang.org/doc/install).

Now fetch and build the `livepeer` node using go - `go get github.com/livepeer/golp/cmd/livepeer`

## Setup
The current version of Livepeer requires [ffmpeg](https://www.ffmpeg.org/).

On OSX, run
`brew install ffmpeg --with-ffplay`

or on Debian based Linux
`apt-get install ffmpeg`

## Usage
The simplest way to start Livepeer is by running `./livepeer`.  This will use the default data directory, default ports, and connect to the default test network boot node.

To see more configuration details, use `./livepeer -h`

### Start Streaming
Livepeer takes RTMP streams as input. You can use any streaming software to create the RTMP stream. We recommend [OBS](https://obsproject.com/download) or [ffmpeg](https://www.ffmpeg.org/).

By default, the RTMP port is 1935.  For example, if you are using OSX with ffmpeg, run 
`ffmpeg -f avfoundation -framerate 30 -pixel_format uyvy422 -i "0:0" -vcodec libx264 -tune zerolatency -b 1000k -x264-params keyint=60:min-keyint=60 -acodec aac -ac 1 -b:a 96k -f flv rtmp://localhost:1935/movie`

To get the streamID, you can do `curl http://localhost:8935/streamID`

Now that the stream is available on the network, it's viewable from any node. For example, you can view it by starting a new node and viewing it from there:

`./livepeer -p=15001 -rtmp=1936 -http=8936 -datadir=./data1`

&&

`./livepeer stream -hls=true -port=8936 -id={StreamID}`

### Ethereum Transactions
By default, Livepeer nodes run in off-chain mode.  To start Livepeer in on-chain mode, you need to provide connection information to an Ethereum node.

TODO

### Transcoding
TODO

