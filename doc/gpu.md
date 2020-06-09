# GPU Support

Livepeer supports decoding and encoding on NVIDIA GPUs on Linux. GPU
transcoding can be enabled by starting Livepeer in `-transcoder` mode with the
`-nvidia <device-list>` flag. The `<device-list>` is a comma-separated
numerical list of GPU devices that you wish to use for transcoding. If you are
unsure of your GPU device, use the `nvidia-smi` utility. For example, to select
devices 0, 2 and 4:

```
./livepeer -transcoder -nvidia 0,2,4
```

### Limitations

Currently the following limitations are observed:

* **Device validity** Ensure valid devices are selected when starting up the node. Currently there is no start-up check to ensure device validity.

* **YUV 4:2:0 input format** The pixel format of the source video must be in YUV 4:2:0 format (planar or
interleaved). Anything else will return an error.

* **CUDA Availability** If running the Livepeer binary, the CUDA shared libraries are expected to be installed in `/usr/local/cuda`. If the CUDA location differs on your machine, run the node with `LD_LIBRARY_PATH=</path/to/cuda>` environment variable.

So far, Livepeer has been tested  to work with the following driver versions:

CUDA | Nvidia
--|--
10.0.130 |
10.1 | 418.39 , 430.50
10.2.89 | 440.33.01
11.0 | 450.36.06

* **Driver Limits** "Retail GPU cards may impose a software limit on the number of concurrent transcode sessions allowed on the system in official drivers.

* **Linux Only** We've only tested this on Linux. We haven't tried other platforms; if it works elsewhere, especially on Windows or OSX, let us know!

### Running Tests

A number of GPU unit tests are included. These may help verify your GPU setup.
To run these tests, the Livepeer source code must be obtained; see the
[install documentation](install.md) for details on setting up a build
environment. Then the Livepeer unit test suite can be run with the `NV_DEVICE`
environment variable. For example, to run the unit tests on GPU 1:

```
NV_DEVICE=1 bash test.sh
```

A more intensive set of GPU tests is available in the LPMS repository, which is vendored within `go-livepeer`. Refer to the [LPMS README](https://github.com/livepeer/lpms/blob/master/README.md) for details on how to run these tests.
