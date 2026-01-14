package worker

import (
	"context"

	"github.com/Masterminds/semver/v3"
	"github.com/livepeer/go-livepeer/clog"
)

// LowestVersion returns the lowest version of a given pipeline and model ID from a list of versions.
func LowestVersion(versions []Version, pipeline string, modelId string) string {
	var res string
	var lowest *semver.Version

	for _, v := range versions {
		if v.Pipeline != pipeline || v.ModelId != modelId {
			continue
		}
		ver, err := semver.NewVersion(v.Version)
		if err != nil {
			clog.Warningf(context.Background(), "Invalid runner version '%s'", v)
			continue
		}
		if lowest == nil || ver.LessThan(lowest) {
			if lowest != nil {
				clog.Warningf(context.Background(), "Orchestrator has multiple versions set for the same pipeline and model ID. Using the lowest version: %s", ver)
			}
			lowest = ver
			res = v.Version
		}
	}
	return res
}
