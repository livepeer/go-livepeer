package common

import (
	"bufio"
	"fmt"
	"os"
)

// ReadFromFile attempts to read a file at the supplied location.
// If it fails, then the original supplied string will be returned to the caller.
// A valid string will always be returned, regardless of whether an error occurred.
func ReadFromFile(s string) (string, error) {
	info, err := os.Stat(s)
	if err !=  nil {
		return s, err
	}
	if os.IsNotExist(err) {
		// If the supplied string is not a path to a file,
		// assume it is the pass and return it
		return s, nil
	}
	if info.IsDir() {
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

	scanner.Scan()
	txtline := scanner.Text()
	file.Close()

	if len(txtline) == 0 {
		return s, fmt.Errorf("supplied file is empty")
	}

	return txtline, nil
}
