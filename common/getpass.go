package common

// GetPass attempts to read a file for a password at the supplied location.
// If it fails, then the original supplied string will be returned to the caller.
// A valid string will always be returned, regardless of whether an error occurred.
func GetPass(s string) (string, error) {
	return ReadFileOrString(s)
}
