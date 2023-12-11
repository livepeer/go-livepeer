package common

import (
	"fmt"
	"os"
	"strings"
)

// ReadFromFile attempts to read a file at the supplied location.
// If it fails, then the original supplied string will be returned to the caller.
// A valid string will always be returned, regardless of whether an error occurred.
func ReadFromFile(s string) (string, error) {
	info, err := os.Stat(s)
	// Return string as-is if the Stat call returned any error
	if err != nil {
		return s, err
	}
	if info.IsDir() {
		// If the supplied string is a directory, return it along with an appropriate error.
		return s, fmt.Errorf("supplied path is a directory")
	}
	bytes, err := os.ReadFile(s)
	if err != nil {
		return s, err
	}
	txt := strings.TrimSpace(string(bytes))

	if len(txt) <= 0 {
		return s, fmt.Errorf("supplied file is empty")
	}

	return txt, nil
}
