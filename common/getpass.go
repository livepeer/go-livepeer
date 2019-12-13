package common

import (
	"bufio"
	"fmt"
	"os"
)

// GetPass attempts to read a file for a password at the supplied location.
// If it fails, then the original supplied string will be returned to the caller.
// A valid string will always be returned, regardless of whether an error occurred.
func GetPass(s string, idx int) (string, error) {
	info, err := os.Stat(s)
	if os.IsNotExist(err) {
		// If the supplied string is not a path to a file,
		// assume it is the pass and return it
		return s, nil
	}
	if os.IsNotExist(err) || info.IsDir() {
		// If the supplied string is a directory,
		// assume it is the pass and return it
		// along with an approptiate error.
		return s, fmt.Errorf("supplied path is a directory")
	}
	file, err := os.Open(s)
	if err != nil {
		return s, err
	}
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	var txtlines []string

	for scanner.Scan() {
		txtlines = append(txtlines, scanner.Text())
		if len(txtlines)-idx == 1 {
			break
		}
	}
	file.Close()

	if len(txtlines) <= idx {
		return s, fmt.Errorf("requested pass index not present in file: %s", s)
	}

	return txtlines[idx], nil
}
