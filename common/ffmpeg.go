package common

import (
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/net"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
)

import "C"

func TxDataToVideoProfile(txData string) ([]ffmpeg.VideoProfile, error) {
	profiles := make([]ffmpeg.VideoProfile, 0)

	if len(txData) == 0 {
		return profiles, nil
	}
	if len(txData) < VideoProfileIDSize {
		return nil, ErrProfile
	}

	for i := 0; i+VideoProfileIDSize <= len(txData); i += VideoProfileIDSize {
		txp := txData[i : i+VideoProfileIDSize]

		p, ok := ffmpeg.VideoProfileLookup[VideoProfileNameLookup[txp]]
		if !ok {
			glog.Errorf("Cannot find video profile for job: %v", txp)
			return nil, ErrProfile // monitor to see if this is too aggressive
		}
		profiles = append(profiles, p)
	}

	return profiles, nil
}

func BytesToVideoProfile(txData []byte) ([]ffmpeg.VideoProfile, error) {
	profiles := make([]ffmpeg.VideoProfile, 0)

	if len(txData) == 0 {
		return profiles, nil
	}
	if len(txData) < VideoProfileIDBytes {
		return nil, ErrProfile
	}

	for i := 0; i+VideoProfileIDBytes <= len(txData); i += VideoProfileIDBytes {
		var txp [VideoProfileIDBytes]byte
		copy(txp[:], txData[i:i+VideoProfileIDBytes])

		p, ok := ffmpeg.VideoProfileLookup[VideoProfileByteLookup[txp]]
		if !ok {
			glog.Errorf("Cannot find video profile for job: %v", txp)
			return nil, ErrProfile // monitor to see if this is too aggressive
		}
		profiles = append(profiles, p)
	}

	return profiles, nil
}

func FFmpegProfiletoNetProfile(ffmpegProfiles []ffmpeg.VideoProfile) ([]*net.VideoProfile, error) {
	profiles := make([]*net.VideoProfile, 0, len(ffmpegProfiles))
	for _, profile := range ffmpegProfiles {
		width, height, err := ffmpeg.VideoProfileResolution(profile)
		if err != nil {
			return nil, err
		}
		br := strings.Replace(profile.Bitrate, "k", "000", 1)
		bitrate, err := strconv.Atoi(br)
		if err != nil {
			return nil, err
		}
		name := profile.Name
		if name == "" {
			name = "ffmpeg_" + ffmpeg.DefaultProfileName(width, height, bitrate)
		}
		format := net.VideoProfile_MPEGTS
		switch profile.Format {
		case ffmpeg.FormatNone:
		case ffmpeg.FormatMPEGTS:
		case ffmpeg.FormatMP4:
			format = net.VideoProfile_MP4
		default:
			return nil, ErrFormatProto
		}
		encoderProf := net.VideoProfile_ENCODER_DEFAULT
		switch profile.Profile {
		case ffmpeg.ProfileNone:
		case ffmpeg.ProfileH264Baseline:
			encoderProf = net.VideoProfile_H264_BASELINE
		case ffmpeg.ProfileH264Main:
			encoderProf = net.VideoProfile_H264_MAIN
		case ffmpeg.ProfileH264High:
			encoderProf = net.VideoProfile_H264_HIGH
		case ffmpeg.ProfileH264ConstrainedHigh:
			encoderProf = net.VideoProfile_H264_CONSTRAINED_HIGH
		default:
			return nil, ErrProfProto
		}
		encoder := net.VideoProfile_H264
		switch profile.Encoder {
		case ffmpeg.H264:
			encoder = net.VideoProfile_H264
		case ffmpeg.H265:
			encoder = net.VideoProfile_H265
		case ffmpeg.VP8:
			encoder = net.VideoProfile_VP8
		case ffmpeg.VP9:
			encoder = net.VideoProfile_VP9
		default:
			return nil, ErrProfEncoder
		}
		gop := int32(0)
		if profile.GOP < 0 {
			gop = int32(profile.GOP)
		} else {
			gop = int32(profile.GOP.Milliseconds())
		}
		var chromaFormat net.VideoProfile_ChromaSubsampling
		switch profile.ChromaFormat {
		case ffmpeg.ChromaSubsampling420:
			chromaFormat = net.VideoProfile_CHROMA_420
		case ffmpeg.ChromaSubsampling422:
			chromaFormat = net.VideoProfile_CHROMA_422
		case ffmpeg.ChromaSubsampling444:
			chromaFormat = net.VideoProfile_CHROMA_444
		default:
			return nil, ErrChromaFormat
		}
		fullProfile := net.VideoProfile{
			Name:         name,
			Width:        int32(width),
			Height:       int32(height),
			Bitrate:      int32(bitrate),
			Fps:          uint32(profile.Framerate),
			FpsDen:       uint32(profile.FramerateDen),
			Format:       format,
			Profile:      encoderProf,
			Gop:          gop,
			Encoder:      encoder,
			ColorDepth:   int32(profile.ColorDepth),
			ChromaFormat: chromaFormat,
		}
		profiles = append(profiles, &fullProfile)
	}
	return profiles, nil
}

func ProfilesToTranscodeOpts(profiles []ffmpeg.VideoProfile) []byte {
	transOpts := []byte{}
	for _, prof := range profiles {
		transOpts = append(transOpts, crypto.Keccak256([]byte(prof.Name))[0:4]...)
	}
	return transOpts
}

func ProfilesToHex(profiles []ffmpeg.VideoProfile) string {
	return hex.EncodeToString(ProfilesToTranscodeOpts(profiles))
}

func ProfilesNames(profiles []ffmpeg.VideoProfile) string {
	names := make(sort.StringSlice, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	names.Sort()
	return strings.Join(names, ",")
}

func ProfileExtensionFormat(ext string) ffmpeg.Format {
	p, ok := ffmpeg.ExtensionFormats[ext]
	if !ok {
		return ffmpeg.FormatNone
	}
	return p
}

func ProfileFormatExtension(f ffmpeg.Format) (string, error) {
	ext, ok := ffmpeg.FormatExtensions[f]
	if !ok {
		return "", ErrFormatExt
	}
	return ext, nil
}

func ProfileFormatMimeType(f ffmpeg.Format) (string, error) {
	ext, err := ProfileFormatExtension(f)
	if err != nil {
		return "", err
	}
	return TypeByExtension(ext)
}

func ParseAccelDevices(devices string, acceleration ffmpeg.Acceleration) ([]string, error) {
	if acceleration == ffmpeg.Nvidia && devices == "all" {
		return detectNvidiaDevices()
	}
	return strings.Split(devices, ","), nil
}
