package byoc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateBYOCJobTimeoutSeconds(t *testing.T) {
	require := require.New(t)

	require.NoError(ValidateBYOCJobTimeoutSeconds(1))
	require.NoError(ValidateBYOCJobTimeoutSeconds(MaxBYOCJobTimeoutSeconds))

	err := ValidateBYOCJobTimeoutSeconds(0)
	require.Error(err)
	require.Contains(err.Error(), "positive")

	err = ValidateBYOCJobTimeoutSeconds(-1)
	require.Error(err)
	require.Contains(err.Error(), "positive")

	err = ValidateBYOCJobTimeoutSeconds(MaxBYOCJobTimeoutSeconds + 1)
	require.Error(err)
	require.Contains(err.Error(), "maximum")
}
