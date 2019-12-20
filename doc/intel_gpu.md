# Intel GPU Support

Livepeer supports decoding and encoding on Intel GPUs on Linux. GPU
transcoding can be enabled by starting Livepeer in `-transcoder` mode with the
`-intel <device-list>` flag. The `<device-list>` is a comma-separated list of Intel GPU device ID that you wish to use for transcoding. If you are
unsure of your GPU device, use the `ls /dev/dri/render*` command. For example, to select
devices /dev/dri/renderD128, and /dev/dri/renderD129:

```
./livepeer -transcoder -intel /dev/dri/renderD128,/dev/dri/renderD129
```

### Limitations

Currently the following limitations are observed:

* **Device validity** Ensure valid devices are selected when starting up the node. Currently there is no start-up check to ensure device validity.

* **YUV 4:2:0 input format** The pixel format of the source video must be in YUV 4:2:0 format (planar or
interleaved). Anything else will return an error.

* **VAAPI Availability** If running the Livepeer binary, the VA shared libraries are expected to be installed in the system.

* **Linux Only** We've only tested this on Linux. We haven't tried other platforms; if it works elsewhere, especially on Windows or OSX, let us know!

### Running Tests

A number of GPU unit tests are included. These may help verify your GPU setup.
To run these tests, the Livepeer source code must be obtained; see the
[install documentation](install.md) for details on setting up a build
environment. Then the Livepeer unit test suite can be run with the `IN_DEVICE`
environment variable. For example, to run the unit tests on GPU /dev/dri/renderD128:

```
IN_DEVICE=/dev/dri/renderD128 bash test.sh
```

A more intensive set of GPU tests is available in the LPMS repository, which is vendored within `go-livepeer`. Refer to the [LPMS README](https://github.com/livepeer/lpms/blob/ja/bottleneck/README.md) for details on how to run these tests.
