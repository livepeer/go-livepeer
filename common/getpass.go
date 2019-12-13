package common

import (
	"bufio"
	"fmt"
	"os"
)

func GetPass(s string, idx int) (string, error) {
	info, err := os.Stat(s)
	if os.IsNotExist(err) || info.IsDir() {
		// If the supplied string is a directory or it is not a
		// path to a file, assume it is the pass and return it
		return s, nil
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
