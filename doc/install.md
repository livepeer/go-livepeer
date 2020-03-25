# Installing Livepeer

## Option 1: Download pre-built executables from Livepeer

The easiest way to install Livepeer is by downloading the pre-built `livepeer` and `livepeer_cli` executables.

### Downloading / installing the software

1. Go to [Livepeer's release page on Github](https://github.com/livepeer/go-livepeer/releases).
2. Under "Assets", download the `.tar.gz` files for your operating system - `darwin` for MacOS, `linux` for Linux.
2. Untar them with `tar -xzf livepeer-linux-amd64.tar.gz` or `tar -xzf livepeer-darwin-amd64.tar.gz`

### Running the software

You can then run the `livepeer` binary with the following simple command.
```
./livepeer-linux-amd64/livepeer -broadcaster
```
This creates an `RTMP` endpoint on `127.0.0.1:1935` and an `HLS` media server on `127.0.0.1:8935`.

### Basic test of the installation

You can serve `RTMP` content into the endpoint using the following command:
```
ffmpeg -re -f lavfi -i testsrc=size=1000x1000:rate=30,format=yuv420p -f lavfi -i sine -threads 1 -c:v libx264 -b:v 4000000 -preset ultrafast -x264-params keyint=30 -strict -2 -c:a aac -f flv rtmp://127.0.0.1:1935/test_signal
```
You can query the `HLS` content coming from the media server using:
```
curl http://127.0.0.1:8935/stream/test_signal.m3u8
```
If you receive the following message, your basic `livepeer` node is running:
```
#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:PROGRAM-ID=0,BANDWIDTH=4000000,RESOLUTION=1000x1000
test_signal/source.m3u8
```

For regular builds published by Livepeer's automated build process, see below.

## Option 2: Build from source

You can also build your own executables from source code.

### Pre-requisites and Setup

&ensp; 1\. Install Go, using Go's [Getting Started Guide](https://golang.org/doc/install).

&ensp; 2\. Make sure you have the necessary libraries installed:

 * Linux: `apt-get update && apt-get -y install build-essential pkg-config autoconf gnutls-dev git`

 * OSX: `brew update && brew install pkg-config autoconf gnutls`

&ensp; 3\. Fetch the code running `go get github.com/livepeer/go-livepeer/cmd/livepeer` in terminal. This will download the software to `/src/github.com/livepeer/go-livepeer` in your `go` install folder.

&ensp; 4\. You need to install `FFmpeg` as a dependency.  Run `./install_ffmpeg.sh` from the `go-livepeer` folder.

### Make the software

&ensp; 5\. You can now run `HIGHEST_CHAIN_TAG=dev PKG_CONFIG_PATH=~/compiled/lib/pkgconfig make` from the project root directory.

&ensp; &ensp; a\. `HIGHEST_CHAIN_TAG` (available values: `dev`, `rinkeby` & `mainnet`) represents the highest compatible chain to build for with an ordering of `dev < rinkeby < mainnet`, e.g. using `HIGHEST_CHAIN_TAG=rinkeby` will allow the node to run on the public Rinkeby test network but throw an error when trying to connect to the main Ethereum network.

&ensp; &ensp; b\. `PKG_CONFIG_PATH` is the path where `pkg-config` files for `FFmpeg` dependencies have been installed _(see step 3 & 4)_. This defaults to `~/compiled/lib/pkgconfig` if you used the `FFmpeg` install script in step 4.

### Test your build

&ensp; 6\. To run tests locally `./test.sh`, to run in docker container run `./test_docker.sh`

## Option 3: Using Docker

If you prefer to use [the official livepeer docker image](https://hub.docker.com/r/livepeer/go-livepeer) by simply pulling it

```bash
docker pull livepeer/go-livepeer
```

**Or** if you'd like to modify the code or try something out, you can build the image using the `Dockerfile.debian` in the repo

```bash
# clone this repo
git clone https://github.com/livepeer/go-livepeer.git

# do the modification you'd like to do the code
# ...

# have the repo tags exported to file
echo $(git describe --tags) > .git.describe

# in repo root folder
docker build -t livepeerbinary:debian -f docker/Dockerfile.debian .

# test it
docker run -it livepeerbinary:debian livepeer -version
```

## Option 4: Private testnet deployments

To setup a full Livepeer network deployment, try out our [test-harness](https://github.com/livepeer/test-harness) which automates the process of deploying the Livepeer developer testnet. This includes the Livepeer solidity contracts along with secondary services like a full metrics suite for debugging and a fully working Livepeer nodes running locally or on Google Cloud Platform (GCP).

Read more about GCP deployments [here](https://github.com/livepeer/test-harness/blob/master/docs/demo.md).

## Option 5: Automated Build Process - Latest Codebase

There are also binaries produced from every GitHub commit made available in [the #builds channel of the Livepeer Discord server](https://discord.gg/drApskX).

Those binaries are produced from go-livepeer's CI process, shown in this diagram:

![image](https://user-images.githubusercontent.com/257909/58923612-3709a800-86f5-11e9-838b-6202f296bce8.png)
