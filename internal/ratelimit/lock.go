// Package ratelimit coordinates outbound request limits across l1ner processes.
package ratelimit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// ErrBusy indicates that a request was not sent because the local limit was reached.
var ErrBusy = errors.New("local rate limit reached")

// Acquire reserves a request. Close the returned file after the request ends.
func Acquire(path, service string, interval time.Duration) (_ *os.File, err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create %s rate limit directory: %w", service, err)
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open %s rate limit file: %w", service, err)
	}
	defer func() {
		if err != nil {
			file.Close()
		}
	}()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("lock %s rate limit file: %w", service, err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s rate limit file: %w", service, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s rate limit file is not a regular file", service)
	}
	if info.Size() != 0 && time.Since(info.ModTime()) < interval {
		return nil, ErrBusy
	}
	// The single byte distinguishes a new file. Its modification time records the request.
	if _, err := file.WriteAt([]byte{1}, 0); err != nil {
		return nil, fmt.Errorf("update %s rate limit file: %w", service, err)
	}
	return file, nil
}
