package e2e

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"testing"
)

func TestMistJson(t *testing.T) {
	assert := assert.New(t)
	buildLivepeer(assert)
	// run
	lp := exec.Command("./livepeer", "-j")
	stdoutRes, err := lp.Output()
	assert.NoError(err)

	// parse output
	jsonMap := make(map[string](interface{}))
	err = json.Unmarshal(stdoutRes, &jsonMap)
	assert.NoError(err)
	// only check for name element
	assert.Contains(jsonMap, "name")
}
