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


### Transcoding


