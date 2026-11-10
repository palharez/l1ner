package identify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type fingerprint struct {
	text     string
	duration int
}

func fingerprintMP3(ctx context.Context, path string) (fingerprint, error) {
	if !strings.EqualFold(filepath.Ext(path), ".mp3") {
		return fingerprint{}, fmt.Errorf("expected an MP3 file: %s", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fingerprint{}, err
	}
	if !info.Mode().IsRegular() {
		return fingerprint{}, fmt.Errorf("not a regular MP3 file: %s", path)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fingerprint{}, err
	}
	return runFPCalc(ctx, absolute, nil)
}

func runFPCalc(ctx context.Context, path string, file *os.File) (fingerprint, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Force the MP3 demuxer so a file with the extension cannot be interpreted as a playlist.
	cmd := exec.CommandContext(ctx, "fpcalc", "-json", "-format", "mp3", path)
	if file != nil {
		cmd.ExtraFiles = []*os.File{file} // The first inherited file is descriptor 3.
	}
	cmd.WaitDelay = time.Second
	stdout := &processOutput{limit: 64 << 10}
	stderr := &processOutput{limit: 1024}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return fingerprint{}, fmt.Errorf("calculate audio fingerprint: %w", ctx.Err())
	}
	if errors.Is(err, exec.ErrNotFound) {
		return fingerprint{}, errors.New("fpcalc was not found on PATH; install Chromaprint's fpcalc utility")
	}
	if err != nil {
		if detail := strings.TrimSpace(stderr.buffer.String()); detail != "" {
			return fingerprint{}, fmt.Errorf("fpcalc failed: %s", detail)
		}
		return fingerprint{}, fmt.Errorf("fpcalc failed: %w", err)
	}
	if stdout.truncated {
		return fingerprint{}, errors.New("fpcalc output exceeded the size limit")
	}
	var output struct {
		Duration    float64 `json:"duration"`
		Fingerprint string  `json:"fingerprint"`
	}
	if err := json.Unmarshal(stdout.buffer.Bytes(), &output); err != nil {
		return fingerprint{}, fmt.Errorf("decode fpcalc output: %w", err)
	}
	if output.Duration < 1 || output.Duration > math.MaxInt32 || strings.TrimSpace(output.Fingerprint) == "" {
		return fingerprint{}, errors.New("fpcalc returned no usable fingerprint or duration")
	}
	return fingerprint{text: output.Fingerprint, duration: int(output.Duration)}, nil
}

type processOutput struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (output *processOutput) Write(data []byte) (int, error) {
	keep := min(len(data), output.limit-output.buffer.Len())
	output.buffer.Write(data[:keep])
	output.truncated = output.truncated || keep < len(data)
	return len(data), nil
}
