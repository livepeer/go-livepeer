package server

import (
	"fmt"
	"net/http"

	"github.com/livepeer/lpms/ffmpeg"
)

// Livepeer netwok & crypto integration interface for tests
// Work in progess ...

type TestBroadasterRouter struct {
	verificationFrequency        int // run once in verificationFrequency number of segments
	verificationSegmentCountdown int
	availableOrchestrators       []*OrchestratorContact
}

func (b *TestBroadasterRouter) ChooseOrchestrators(r *http.Request, info MediaFormatInfo) (ChosenOrchestrators, error) {
	chosen := ChosenOrchestrators{}
	if b.verificationSegmentCountdown <= 0 {
		b.verificationSegmentCountdown = b.verificationFrequency - 1
		// Return all in verification case
		chosen.Orchestrators = append(chosen.Orchestrators, b.availableOrchestrators...)
	} else {
		b.verificationSegmentCountdown -= 1
		// TODO: keep our favourite
		chosen.Orchestrators = append(chosen.Orchestrators, b.availableOrchestrators...)
	}
	return chosen, nil
}

func (b *TestBroadasterRouter) GetTranscodeSpec(r *http.Request, info MediaFormatInfo) (TranscodeJobSpec, error) {
	var spec TranscodeJobSpec
	// Fixed manifest id in this test
	spec.ManifestID = "5ac66bc9-c000-439f-9ade-dc48b73dc54e"
	// 3 Renditions in this test
	spec.Renditions = make([]ffmpeg.VideoProfile, 0, 3)
	spec.Renditions = append(spec.Renditions, ffmpeg.VideoProfile{Name: "P720p24fps16x9", Bitrate: "2000k", Framerate: 24, AspectRatio: "16:9", Resolution: "1280x720"})
	spec.Renditions = append(spec.Renditions, ffmpeg.VideoProfile{Name: "P360p24fps16x9", Bitrate: "1200k", Framerate: 24, AspectRatio: "16:9", Resolution: "640x360"})
	spec.Renditions = append(spec.Renditions, ffmpeg.VideoProfile{Name: "P240p24fps16x9", Bitrate: "600k", Framerate: 24, AspectRatio: "16:9", Resolution: "426x240"})
	return spec, nil
}

func (b *TestBroadasterRouter) OutputProblem(context *BDownstreamDecision) error {
	// Returning error would cause breaking context.outputConnection.
	// I was also exploring possible `context.BreakConnection()` API
	return fmt.Errorf("TestBroadasterRouter always kicks O on OutputProblem")
}

type TestOrchRouter struct{}
