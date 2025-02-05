//go:build !windows

package media

import (
	"bufio"
	"context"
	"encoding/base32"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/livepeer/go-livepeer/clog"
	"golang.org/x/sys/unix"
)

const (
	waitTimeout         = 20 * time.Second
	fileCleanupInterval = time.Hour
	fileCleanupMaxAge   = 4 * time.Hour
	outFileSuffix       = ".ts"
)

type MediaSegmenter struct {
	Workdir        string
	MediaMTXClient *MediaMTXClient
}

func (ms *MediaSegmenter) RunSegmentation(ctx context.Context, in string, segmentHandler SegmentHandler) {
	outFilePattern := filepath.Join(ms.Workdir, randomString()+"-%d"+outFileSuffix)
	completionSignal := make(chan bool, 1)
	procCtx, procCancel := context.WithCancel(context.Background()) // parent ctx is a short lived http request
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer procCancel()
		processSegments(ctx, segmentHandler, outFilePattern, completionSignal)
	}()

	retryCount := 0
	for {
		err := backoff.Retry(func() error {
			streamExists, err := ms.MediaMTXClient.StreamExists()
			if err != nil {
				return fmt.Errorf("StreamExists check failed: %w", err)
			}
			if !streamExists {
				clog.Errorf(ctx, "input stream does not exist")
				return fmt.Errorf("input stream does not exist")
			}
			return nil
		}, backoff.WithMaxRetries(newExponentialBackOff(), 3))
		if err != nil {
			clog.Errorf(ctx, "Stopping segmentation in=%s err=%s", in, err)
			break
		}
		clog.Infof(ctx, "Starting segmentation. in=%s retryCount=%d", in, retryCount)
		cmd := exec.CommandContext(procCtx, "ffmpeg",
			"-i", in,
			"-c:a", "copy",
			"-c:v", "copy",
			"-f", "segment",
			outFilePattern,
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			clog.Errorf(ctx, "Error receiving RTMP: %v", err)
			break
		}
		clog.Infof(ctx, "Segmentation stopped, will retry. retryCount=%d ffmpeg output: %s", retryCount, output)
		time.Sleep(5 * time.Second)
		retryCount++
	}
	completionSignal <- true
	clog.Infof(ctx, "sent completion signal, now waiting")
	wg.Wait()
}

func newExponentialBackOff() *backoff.ExponentialBackOff {
	backOff := backoff.NewExponentialBackOff()
	backOff.InitialInterval = 500 * time.Millisecond
	backOff.MaxInterval = 5 * time.Second
	backOff.Reset()
	return backOff
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

func processSegments(ctx context.Context, segmentHandler SegmentHandler, outFilePattern string, completionSignal <-chan bool) {

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
			clog.Infof(ctx, "Completion signal received. Closing current segment to trigger EOF.")
			currentSegment.Close()
		}
		isComplete = true
		pipeCompletion <- true
		clog.Infof(ctx, "Got completion signal")
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
			clog.Errorf(ctx, "Error opening pipe pipeName=%s err=%s", pipeName, err)
			cleanUpPipe(pipeName)
			cleanUpPipe(nextPipeName)
			break
		}

		mu.Lock()
		currentSegment = file
		mu.Unlock()

		// Handle the reading process
		readSegment(ctx, segmentHandler, file, pipeName)

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

func readSegment(ctx context.Context, segmentHandler SegmentHandler, file *os.File, pipeName string) {
	defer file.Close()
	reader := bufio.NewReader(file)
	writer := NewMediaWriter()
	segmentHandler(writer.MakeReader())
	io.Copy(writer, reader)
	writer.Close()
}

func randomString() string {
	// Create a random 4-byte string encoded as base32, trimming padding
	b := make([]byte, 4)
	for i := range b {
		b[i] = byte(rand.Intn(256))
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(b), "=")
}

// StartFileCleanup starts a goroutine to periodically remove any old temporary files accidentally left behind
func StartFileCleanup(ctx context.Context, workDir string) {
	go func() {
		ticker := time.NewTicker(fileCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := cleanUpLocalTmpFiles(ctx, workDir, "*"+outFileSuffix, fileCleanupMaxAge); err != nil {
					clog.Errorf(ctx, "Error cleaning up segment files: %v", err)
				}
			}
		}
	}()
}

func cleanUpLocalTmpFiles(ctx context.Context, dir string, filenamePattern string, maxAge time.Duration) error {
	filesRemoved := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Mode().IsRegular() {
			if match, _ := filepath.Match(filenamePattern, info.Name()); match {
				if time.Since(info.ModTime()) > maxAge {
					err = os.Remove(path)
					if err != nil {
						return fmt.Errorf("error removing file %s: %w", path, err)
					}
					filesRemoved++
				}
			}
		}
		return nil
	})
	clog.Infof(ctx, "Segment file cleanup removed %d files", filesRemoved)
	return err
}
