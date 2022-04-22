# Configuring Transcoding Options

The Livepeer node offers several methods to configure the transcoding output.

* Webhook authentication.

* `-transcodingOptions` CLI flag with a list of presets

* `-transcodingOptions` CLI flag with a JSON configuration file

* `/setBroadcastConfig` endpoint for the CLI API

* `livepeer_cli` tool

### Codecs

Transcoding to and from H.264 is supported.

### Aspect Ratio

The Livepeer transcoder maintains the aspect ratio of the source video in order to maintain output video quality. This may sometimes result in the transcoded resolution being somewhat different from the original specification.

 If the aspect ratio of the source video does not match the target aspect ratio, the output is fit to the larger dimension, and the smaller dimension is rescaled proportionately to maintain the source aspect ratio.

If a different behavior is needed, please [let us know](https://github.com/livepeer/go-livepeer/issues/new?template=feature_request.md) by filing a feature request.

### Webhook Authentication

See the [webhook documentation](rtmpwebhookauth.md) for full details. To configure the transcoding output, either the `profiles` or `presets` fields in the webhook response can be set, or both.

The `profiles` field in the webhook follows the JSON options that are described in this document. The valid names for the `preset` field are described in the following section.

### `-transcodingOptions` CLI flag with a list of presets

The Livepeer node comes with a set of pre-defined transcoding presets. At start-up, the node can be configured with a comma-separated list of presets. The available presets are defined within the [LPMS video profiles](https://github.com/livepeer/lpms/blob/master/ffmpeg/videoprofile.go#L60-L92).


For example, to run the Livepeer node with the 720P 25fps and 240p 30fps 4x3 presets:

```
livepeer -transcodingOptions P720p25fps16x9,P240p30fps4x3
```

### `-transcodingOptions` CLI flag with a JSON configuration file

The Livepeer node can receive transcoding configuration via JSON config file. Run the node with the `-transcodingOptions` flag and a path to the JSON file.

```
livepeer -transcodingOptions /path/to/config.json
```

The JSON configuration takes a list of renditions,
```
[
    { rendition },
    { rendition },

]
```

Each rendition object takes the following fields:

* `name`  : String identifier by which the rendition can be referred to. If
  a name is omitted, the default naming scheme follows `webhook_<width>x<height>_<bitrate>`
* `width` : Integer width
* `height` : Integer height
* `bitrate` : Integer bitrate, in bits per second
* `fps` : Integer framerate, in frames per second. If zero or omitted, no FPS
  adjustment is done and the output will have the same number of frames spaced
at similar timing intervals as the source.
* `fpsDen` : Integer framerate denominator. Useful for interoperability with
  certain applications, eg NTSC's 29.97 fps (30000/1001). This value defaults to 1 if zero or omitted.
* `profile` : String codec encoding profile to use. Supported values are
  "H264Baseline", "H264Main", "H264High", "H264ConstrainedHigh". The field can
be omitted or set to "None" to use the encoder default.
* `gop` : String [GOP](https://en.wikipedia.org/wiki/Group_of_pictures) length,
  in seconds. This may help in post-transcoding segmentation to smooth out
playback if the original segments are long or irregularly-sized. Omitting this
field will use the encoder default. To force all intra frames, use "intra".

An example of a full JSON configuration:

```
[
    {
        "name": "ntsc-1080p",
        "width": 1920,
        "height": 1080,
        "bitrate": 5000000,
        "fps": 30000,
        "fpsDen": 1001,
        "profile": "H264High",
        "gop": "2"
    },{
        "name": "webrtc-720p",
        "width": 1280,
        "height": 720,
        "bitrate": 1500000,
        "profile": "H264ConstrainedHigh"
    },{
        "name":"highlight-reel",
        "width":160,
        "height":120,
        "bitrate":100000,
        "fps": 1,
        "gop":"intra"
    }
]
```


### `/setBroadcastConfig` endpoint for the CLI API

The CLI port has a `/setBroadcastConfig` API . This may be useful if the transcoding options set via CLI flags need to be adjusted at runtime. Note that pricing needs to be set; for offchain transcoding, a placehodler value will suffice.

```
curl 'http://localhost:7935/setBroadcastConfig?transcodingOptions=P720p25fps16x9,P240p30fps4x3&maxPricePerUnit=1&pixelsPerUnit=1'
```

### `livepeer_cli` tool

For a wizard-based interface to the CLI API, the `livepeer_cli` tool may be used. Look for the 'Set broadcast config' option and follow the prompts.

```
livepeer_cli
# ... status information about the node
What would you like to do? (default = stats)
# various options
16. Set broadcast config

> 16
Enter a maximum transcoding price per pixel, in wei per pixels (pricePerUnit / pixelsPerUnit).
eg. 1 wei / 10 pixels = 0,1 wei per pixel

Enter amount of pixels that make up a single unit (default: 1 pixel) - >

Enter the maximum price to pay for 1 pixels in Wei (required) - > 1
Identifier	Transcoding Options
0		P144p30fps16x9
1		P240p25fps16x9
2		P240p30fps16x9
3		P240p30fps4x3
4		P360p25fps16x9
5		P360p30fps16x9
6		P360p30fps4x3
7		P576p25fps16x9
8		P576p30fps16x9
9		P720p25fps16x9
10		P720p30fps16x9
11		P720p30fps4x3
12		P720p60fps16x9
Enter the identifiers of the video profiles you would like to use (use a comma as the delimiter in a list of identifiers) - > 10,3

```

