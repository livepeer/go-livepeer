# Installing Livepeer

- [Installing Livepeer](#installing-livepeer)
	- [Install a binary release](#install-a-binary-release)
	- [Install with Docker](#install-with-docker)
	- [Install a binary pre-release](#install-a-binary-pre-release)
	- [Build from source](#build-from-source)
		- [System dependencies](#system-dependencies)
		- [Go](#go)
		- [Build and install](#build-and-install)
	- [Build with Docker](#build-with-docker)

## Install a binary release

Find the latest release for your platform on the [releases page](https://github.com/livepeer/go-livepeer/releases). Linux, Darwin (macOS) and Windows are supported.

```bash
# <RELEASE_VERSION> is the release version i.e. 0.5.14
# <YOUR_PLATFORM> is your platform i.e. linux, darwin, windows
wget https://github.com/livepeer/go-livepeer/releases/download/<RELEASE_VERSION>/livepeer-<YOUR PLATFORM>-amd64.tar.gz
tar -zxvf livepeer-<YOUR_PLATFORM>-amd64.tar.gz
mv livepeer-<YOUR_PLATFORM>-amd64/* /usr/local/bin/
```

## Install with Docker

Docker images are pushed to [DockerHub](https://hub.docker.com/r/livepeer/go-livepeer).

```bash
# <RELEASE_VERSION> is the release version i.e. 0.5.14
docker pull livepeer/go-livepeer:<RELEASE_VERSION>
```

To pull the latest pre-release version:

```bash
docker pull livepeer/go-livepeer:master
```

## Install a binary pre-release

Binaries are produced from every GitHub commit and download links are available in [the #builds channel of the Livepeer Discord server](https://discord.gg/drApskX).

These binaries are produced from go-livepeer's CI process, shown in this diagram:

![image](https://user-images.githubusercontent.com/257909/58923612-3709a800-86f5-11e9-838b-6202f296bce8.png)

## Build from source

### System dependencies 

Building `livepeer` requires some system dependencies.

Linux (Ubuntu: 16.04 or 18.04):

```bash
apt-get update && apt-get -y install build-essential pkg-config autoconf gnutls-dev git curl
# To enable transcoding on Nvidia GPUs
apt-get -y install clang-8 clang-tools-8
```

Darwin (macOS):

```bash
brew update && brew install pkg-config autoconf gnutls
```

### Go

Building `livepeer` requires Go. Follow the [official Go installation instructions](https://golang.org/doc/install).

### Build and install

1. Clone the repository:

	```bash
	git clone https://github.com/livepeer/go-livepeer.git
	cd go-livepeer
	```

2. Install `ffmpeg` dependencies:

	```bash
	./install_ffmpeg.sh 
	```

3. Set build environment variables.

	Set the `PKG_CONFIG_PATH` variable so that `pkg-config` can find the `ffmpeg` dependency files installed in step 2:

	```bash
	# install_ffmpeg.sh stores ffmpeg dependency files in this directory by default
	export PKG_CONFIG_PATH=~/compiled/lib/pkgconfig
	```

	Set the `HIGHEST_CHAIN_TAG` variable to enable mainnet support:

	```bash
	export HIGHEST_CHAIN_TAG=mainnet
	# To build with support for only development networks and the Rinkeby test network
	# export HIGHEST_CHAIN_TAG=rinkeby
	# To build with support for only development networks
	# export HIGHEST_CHAIN_TAG=dev
	```

4. Build and install `livepeer`:

	```bash
	make
	cp livepeer* /usr/local/bin
	```

## Build with Docker

1. Clone the repository:

	```bash
	git clone https://github.com/livepeer/go-livepeer.git
	cd go-livepeer
	```

2. Export tags:

	```bash
	echo $(git describe --tags) > .git.describe
	```

3. Build image:

	```bash
	docker build -t livepeerbinary:debian -f docker/Dockerfile.debian .
	```