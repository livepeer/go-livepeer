# Installing and Managing Go 

## Go

Follow the instructions at https://go.dev/doc/install to download and install Go.

If you are developing on Apple Silicon (M1), you will need to:

- Use Go >= 1.16 as arm64 support was [introduced in 1.16](https://go.dev/doc/go1.16)
- Download the `*-darwin-arm64` binary instead of the `*-darwin-amd64` binary on the [downloads page](https://go.dev/dl/).

### Managing Go Versions

There are a few ways to manage different Go versions:

1. [go install](https://go.dev/doc/manage-install)
2. [gvm](https://github.com/moovweb/gvm)

Using `gvm` has the benefit of automatically aliasing `go` to whichever version of Go you are currently using as opposed to having to use a command like `go1.10.7`.

**gvm: Installing arm64 binaries on >= macOS 11**

Until https://github.com/moovweb/gvm/pull/380 is merged, gvm does not support installing arm64 binaries on >= macOS 11 (i.e. Big Sur). A workaround for this issue is to install gvm using the fork for the PR:

```
# Download installer script
curl -s -S -L https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer
# Run installer script using feature branch from fork
SRC_REPO=https://github.com/jeremy-ebler-vineti/gvm.git bash gvm_installer feature/support-big-sur
```

Then, you can run the following command which should download the arm64 binary if you are on an arm64 machine:

```
gvm install <version> -B
```
