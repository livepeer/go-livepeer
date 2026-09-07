//go:build darwin

package media

import "syscall"

func crossPlatformSelect(nfd int, r, w, e *syscall.FdSet, timeout *syscall.Timeval) (int, error) {
	// On macOS, syscall.Select only returns an error
	err := syscall.Select(nfd, r, w, e, timeout)
	if err != nil {
		return -1, err // Return -1 in case of an error
	}
	// We need to manually count the number of ready descriptors in FdSets
	n := 0
	if r != nil {
		n += countReadyDescriptors(r, nfd)
	}
	if w != nil {
		n += countReadyDescriptors(w, nfd)
	}
	if e != nil {
		n += countReadyDescriptors(e, nfd)
	}
	return n, nil

}

// countReadyDescriptors manually counts the number of ready file descriptors in an FdSet
func countReadyDescriptors(set *syscall.FdSet, nfd int) int {
	count := 0
	for fd := 0; fd < nfd; fd++ {
		if isSet(fd, set) {
			count++
		}
	}
	return count
}

// isSet checks if a file descriptor is set in an FdSet
func isSet(fd int, set *syscall.FdSet) bool {
	return set.Bits[fd/64]&(1<<(uint(fd)%64)) != 0
}
