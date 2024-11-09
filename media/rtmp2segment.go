//go:build !windows

package media

import (
	"bufio"
	"encoding/base32"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/livepeer/lpms/ffmpeg"
	"golang.org/x/sys/unix"
)

var waitTimeout = 20 * time.Second

type MediaSegmenter struct {
	Workdir string
}

func (ms *MediaSegmenter) RunSegmentation(in string, segmentHandler SegmentHandler) {

	outFilePattern := filepath.Join(ms.Workdir, randomString()+"-%d.ts")
	completionSignal := make(chan bool, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		processSegments(segmentHandler, outFilePattern, completionSignal)
	}()
	ffmpeg.FfmpegSetLogLevel(ffmpeg.FFLogWarning)
	ffmpeg.Transcode3(&ffmpeg.TranscodeOptionsIn{
		Fname: in,
	}, []ffmpeg.TranscodeOptions{{
		Oname:        outFilePattern,
		AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
		VideoEncoder: ffmpeg.ComponentOptions{Name: "copy"},
		Muxer:        ffmpeg.ComponentOptions{Name: "segment"},
	}})
	completionSignal <- true
	slog.Info("sent completion signal, now waiting")
	wg.Wait()
}

func createNamedPipe(pipeName string) {
	err := syscall.Mkfifo(pipeName, 0666)
	if err != nil && !os.IsExist(err) {
		slog.Error("Failed to create named pipe", "pipeName", pipeName, "err", err)
	}
}

func cleanUpPipe(pipeName string) {
	err := os.Remove(pipeName)
	if err != nil {
		slog.Error("Failed to remove pipe", "pipeName", pipeName, "err", err)
	}
}

func openNonBlockingWithRetry(name string, timeout time.Duration, completed <-chan bool) (*os.File, error) {
	// Pipes block if there is no writer available

	// Attempt to open the named pipe in non-blocking mode once
	fd, err := syscall.Open(name, syscall.O_RDONLY|syscall.O_NONBLOCK, 0666)
	if err != nil {
		return nil, fmt.Errorf("error opening file in non-blocking mode: %w", err)
	}

	deadline := time.Now().Add(timeout)

	// setFd sets the given file descriptor in the fdSet
	setFd := func(fd int, fdSet *syscall.FdSet) {
		idx := fd / 64
		if idx >= len(fdSet.Bits) {
			// only happens under very weird conditions
			return
		}
		fdSet.Bits[idx] |= 1 << (uint(fd) % 64)
	}

	// isFdSet checks if the given file descriptor is set in the fdSet
	isFdSet := func(fd int, fdSet *syscall.FdSet) bool {
		idx := fd / 64
		if idx >= len(fdSet.Bits) {
			// only happens under very weird conditions
			return false
		}
		return fdSet.Bits[idx]&(1<<(uint(fd)%64)) != 0
	}

	for {
		// Check if completed
		select {
		case <-completed:
			syscall.Close(fd)
			return nil, fmt.Errorf("Completed")
		default:
			// continue
		}
		// Calculate the remaining time until the deadline
		timeLeft := time.Until(deadline)
		if timeLeft <= 0 {
			syscall.Close(fd)
			return nil, fmt.Errorf("timeout waiting for file to be ready: %s", name)
		}

		// Convert timeLeft to a syscall.Timeval for the select call
		tv := syscall.NsecToTimeval((100 * time.Millisecond).Nanoseconds())

		// Set up the read file descriptor set for select
		readFds := &syscall.FdSet{}
		setFd(fd, readFds)

		// Wait using select until the pipe is ready for reading
		n, err := crossPlatformSelect(fd+1, readFds, nil, nil, &tv)
		if err != nil {
			if err == syscall.EINTR {
				continue // Retry if interrupted by a signal
			}
			syscall.Close(fd)
			return nil, fmt.Errorf("select error: %v", err)
		}

		// Check if the file descriptor is ready
		if n > 0 && isFdSet(fd, readFds) {
			// Modify the file descriptor to blocking mode using fcntl
			flags, err := unix.FcntlInt(uintptr(fd), syscall.F_GETFL, 0)
			if err != nil {
				syscall.Close(fd)
				return nil, fmt.Errorf("error getting file flags: %w", err)
			}

			// Clear the non-blocking flag
			flags &^= syscall.O_NONBLOCK
			if _, err := unix.FcntlInt(uintptr(fd), syscall.F_SETFL, flags); err != nil {
				syscall.Close(fd)
				return nil, fmt.Errorf("error setting file to blocking mode: %w", err)
			}

			// Convert the file descriptor to an *os.File to return
			return os.NewFile(uintptr(fd), name), nil
		}
	}
}

func processSegments(segmentHandler SegmentHandler, outFilePattern string, completionSignal <-chan bool) {

	// things protected by the mutex mu
	mu := &sync.Mutex{}
	isComplete := false
	var currentSegment *os.File = nil
	pipeCompletion := make(chan bool, 1)

	// Start a goroutine to wait for the completion signal
	go func() {
		<-completionSignal
		mu.Lock()
		defer mu.Unlock()
		if currentSegment != nil {
			// Trigger EOF on the current segment by closing the file
			slog.Info("Completion signal received. Closing current segment to trigger EOF.")
			currentSegment.Close()
		}
		isComplete = true
		pipeCompletion <- true
		slog.Info("Got completion signal")
	}()

	pipeNum := 0
	createNamedPipe(fmt.Sprintf(outFilePattern, pipeNum))

	for {
		pipeName := fmt.Sprintf(outFilePattern, pipeNum)
		nextPipeName := fmt.Sprintf(outFilePattern, pipeNum+1)

		// Create the next pipe ahead of time
		createNamedPipe(nextPipeName)

		// Open the current pipe for reading
		// Blocks if no writer is available so do some tricks to it
		file, err := openNonBlockingWithRetry(pipeName, waitTimeout, pipeCompletion)
		if err != nil {
			slog.Error("Error opening pipe", "pipeName", pipeName, "err", err)
			cleanUpPipe(pipeName)
			cleanUpPipe(nextPipeName)
			break
		}

		mu.Lock()
		currentSegment = file
		mu.Unlock()

		// Handle the reading process
		readSegment(segmentHandler, file, pipeName)

		// Increment to the next pipe
		pipeNum++

		// Clean up the current pipe after reading
		cleanUpPipe(pipeName)

		mu.Lock()
		if isComplete {
			cleanUpPipe(pipeName)
			cleanUpPipe(nextPipeName)
			mu.Unlock()
			break
		}
		mu.Unlock()

	}
}

func readSegment(segmentHandler SegmentHandler, file *os.File, pipeName string) {
	defer file.Close()

	reader := bufio.NewReader(file)
	firstByteRead := false
	totalBytesRead := int64(0)

	buf := make([]byte, 32*1024)

	// TODO should be explicitly buffered for better management
	interfaceReader, interfaceWriter := io.Pipe()
	defer interfaceWriter.Close()
	segmentHandler(interfaceReader)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if !firstByteRead {
				slog.Debug("First byte read", "pipeName", pipeName)
				firstByteRead = true

			}
			totalBytesRead += int64(n)
			if _, err := interfaceWriter.Write(buf[:n]); err != nil {
				if err != io.EOF {
					slog.Error("Error writing", "pipeName", pipeName, "err", err)
				}
			}
		}
		if n == len(buf) && n < 1024*1024 {
			newLen := int(float64(len(buf)) * 1.5)
			slog.Info("Max buf hit, increasing", "oldSize", humanBytes(int64(len(buf))), "newSize", humanBytes(int64(newLen)))
			buf = make([]byte, newLen)
		}

		if err != nil {
			if err.Error() == "EOF" {
				slog.Debug("Last byte read", "pipeName", pipeName, "totalRead", humanBytes(totalBytesRead))
			} else {
				slog.Error("Error reading", "pipeName", pipeName, "err", err)
			}
			break
		}
	}
}

func randomString() string {
	// Create a random 4-byte string encoded as base32, trimming padding
	b := make([]byte, 4)
	for i := range b {
		b[i] = byte(rand.Intn(256))
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(b), "=")
}

func humanBytes(bytes int64) string {
	var unit int64 = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
