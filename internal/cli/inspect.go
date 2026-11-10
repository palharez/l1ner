package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bogem/id3v2/v2"
)

const (
	tagHeaderSize   = 10
	frameHeaderSize = 10
	id3v1TagSize    = 128
	maxTagSize      = 64 << 20
)

type entry struct {
	id    string
	frame id3v2.Framer
}

func inspect(path string) ([]entry, error) {
	if !strings.EqualFold(filepath.Ext(path), ".mp3") {
		return nil, fmt.Errorf("expected an MP3 file: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", path)
	}

	header, body, err := readTag(file, info.Size())
	if err != nil || header == nil {
		return nil, err
	}
	return parseFrames(header, body)
}

func readTag(file io.ReaderAt, fileSize int64) ([]byte, []byte, error) {
	header := make([]byte, tagHeaderSize)
	n, err := file.ReadAt(header, 0)
	if err != nil && err != io.EOF {
		return nil, nil, err
	}
	if n < 3 || string(header[:3]) != "ID3" {
		return nil, nil, checkUntaggedFile(file, fileSize, header[:n])
	}

	bodySize, err := validateTagHeader(header[:n], fileSize)
	if err != nil {
		return nil, nil, err
	}
	body := make([]byte, bodySize)
	if bodySize > 0 {
		if _, err := file.ReadAt(body, tagHeaderSize); err != nil {
			return nil, nil, err
		}
	}
	return header, body, nil
}

func validateTagHeader(header []byte, fileSize int64) (int, error) {
	if len(header) < tagHeaderSize {
		return 0, fmt.Errorf("malformed ID3 header: truncated")
	}
	if header[3] != 3 && header[3] != 4 {
		return 0, fmt.Errorf("unsupported tag version: ID3v2.%d", header[3])
	}
	// The reader does not reliably handle tag transformations or extended headers.
	if header[5] != 0 {
		return 0, fmt.Errorf("unsupported ID3 tag flags: 0x%02x", header[5])
	}
	bodySize, err := syncSize(header[6:tagHeaderSize])
	if err != nil {
		return 0, err
	}
	if int64(bodySize) > fileSize-tagHeaderSize {
		return 0, fmt.Errorf("malformed ID3 tag: truncated body")
	}
	if bodySize > maxTagSize {
		return 0, fmt.Errorf("ID3 tag exceeds the %d MiB inspection limit", maxTagSize>>20)
	}
	return bodySize, nil
}

func checkUntaggedFile(file io.ReaderAt, fileSize int64, prefix []byte) error {
	if fileSize >= id3v1TagSize {
		marker := make([]byte, 3)
		if _, err := file.ReadAt(marker, fileSize-id3v1TagSize); err != nil {
			return err
		}
		if string(marker) == "TAG" {
			return fmt.Errorf("unsupported tag version: ID3v1")
		}
	}
	if len(prefix) < 2 || prefix[0] != 0xff || prefix[1]&0xe0 != 0xe0 {
		return fmt.Errorf("input has no recognized MP3 audio or ID3 header")
	}
	return nil
}
