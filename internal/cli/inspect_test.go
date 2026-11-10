package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Build fixtures directly, independently of the metadata library.
func fixture(version byte, frames ...[]byte) []byte {
	body := bytes.Join(frames, nil)
	h := []byte{'I', 'D', '3', version, 0, 0, 0, 0, 0, 0}
	n := len(body)
	for i := 9; i >= 6; i-- {
		h[i] = byte(n & 127)
		n >>= 7
	}
	return append(append(h, body...), 0xff, 0xfb, 0x90, 0x00)
}
func frame(version byte, id string, payload []byte) []byte {
	h := make([]byte, 10)
	copy(h, id)
	binary.BigEndian.PutUint32(h[4:8], uint32(len(payload)))
	if version == 4 {
		n := len(payload)
		for i := 7; i >= 4; i-- {
			h[i] = byte(n & 127)
			n >>= 7
		}
	}
	return append(h, payload...)
}
func save(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.mp3")
	if err := os.WriteFile(path, data, 0o444); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestInspectionAndIntegrity(t *testing.T) {
	for _, v := range []byte{3, 4} {
		data := fixture(v, frame(v, "TIT2", []byte{0, 'H', 'e', 'l', 'l', 'o'}), frame(v, "TPE1", []byte{0, 'A'}), frame(v, "TPE1", []byte{0, 'B'}), frame(v, "TXXX", []byte{0, 'K', 0, 'V'}), frame(v, "APIC", append([]byte{0, 'i', 'm', 'a', 'g', 'e', '/', 'j', 'p', 'e', 'g', 0, 3, 0}, 1, 2, 3)), frame(v, "XABC", []byte{1, 2}), frame(v, "COMM", []byte{0, 'e', 'n', 'g', 0, 'x', 27, '[', '2', 'J'}))
		path := save(t, data)
		var out, errOut bytes.Buffer
		if code := run([]string{"inspect", path}, &out, &errOut); code != 0 {
			t.Fatalf("v%d: code %d: %s", v, code, errOut.String())
		}
		for _, want := range []string{"Title [TIT2]: Hello", "Artist [TPE1]: A", "Artist [TPE1]: B", "TXXX [K]: V", "image/jpeg, 3 bytes", "XABC: unparsed data, 2 bytes", `x\x1b[2J`} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("missing %q in %s", want, out.String())
			}
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if sha256.Sum256(data) != sha256.Sum256(after) {
			t.Fatal("input changed")
		}
	}
}
func TestInputOutcomes(t *testing.T) {
	legacy := make([]byte, 128)
	copy(legacy, "TAG")
	truncated := fixture(3, frame(3, "TIT2", []byte{0, 'A'}))[:12]
	badSize := fixture(4, frame(4, "TIT2", []byte{0, 'A'}))
	badSize[14] = 128
	flagged := fixture(3)
	flagged[5] = 0x40
	cases := []struct {
		name    string
		data    []byte
		code    int
		message string
	}{
		{"no tags", []byte{255, 251, 144, 0}, 0, "No metadata found"},
		{"empty tag", fixture(3), 0, "No metadata found"},
		{"legacy", legacy, 1, "unsupported tag version: ID3v1"},
		{"v2.2", fixture(2), 1, "unsupported tag version: ID3v2.2"},
		{"truncated", truncated, 1, "truncated body"},
		{"invalid size", badSize, 1, "synchsafe size"},
		{"flags", flagged, 1, "unsupported ID3 tag flags"},
		{"invalid file", []byte("not music"), 1, "no recognized MP3"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			path := save(t, tt.data)
			var out, errOut bytes.Buffer
			code := run([]string{"inspect", path}, &out, &errOut)
			if code != tt.code || !strings.Contains(out.String()+errOut.String(), tt.message) {
				t.Fatalf("code %d; stdout=%s stderr=%s", code, &out, &errOut)
			}
			if code != 0 && out.Len() != 0 {
				t.Fatal("partial success output on failure")
			}
		})
	}
}
func TestArgumentsAndReadErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"inspect"}, {"unknown", "x.mp3"}, {"inspect", "a.mp3", "b.mp3"}} {
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code != 2 {
			t.Fatalf("%v: %d", args, code)
		}
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"--help"}, &out, &errOut); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out.String(), "Audio identification is optional.") {
		t.Errorf("global help should mark identification as optional, got %q", &out)
	}
	for _, path := range []string{filepath.Join(t.TempDir(), "missing.mp3"), t.TempDir() + ".mp3"} {
		if code := run([]string{"inspect", path}, &out, &errOut); code != 1 {
			t.Fatal(code)
		}
	}
	path := save(t, fixture(3))
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o600) })
	if os.Geteuid() != 0 {
		if code := run([]string{"inspect", path}, &out, &errOut); code != 1 {
			t.Fatal("unreadable file accepted")
		}
	}
}

func TestMalformedFrames(t *testing.T) {
	for _, version := range []byte{3, 4} {
		t.Run(fmt.Sprintf("ID3v2.%d", version), func(t *testing.T) {
			for _, tt := range malformedFrameCases(version) {
				t.Run(tt.name, func(t *testing.T) {
					// A valid preceding frame must not produce partial success output.
					data := fixture(version, frame(version, "TIT2", []byte{0, 'A'}), tt.body)
					path := save(t, data)
					assertInspectionOutcome(t, path, 1, "Error: "+tt.message+"\n")
				})
			}
		})
	}
}

type malformedFrameCase struct {
	name    string
	body    []byte
	message string
}

func malformedFrameCases(version byte) []malformedFrameCase {
	cases := []malformedFrameCase{
		{"zero size", frame(version, "TIT2", nil), "malformed TIT2 frame size"},
		{"invalid padding", []byte{0, 0, 1}, "malformed ID3 padding"},
		{"frame after padding", append([]byte{0, 0}, frame(version, "TIT2", []byte{0, 'B'})...), "malformed ID3 padding"},
	}
	complete := frame(version, "TIT2", []byte{0, 'A'})
	for length := 1; length < 10; length++ {
		cases = append(cases, malformedFrameCase{
			name: fmt.Sprintf("truncated header with %d bytes", length),
			body: complete[:length], message: "malformed ID3 frame header: truncated",
		})
	}
	for _, id := range []string{"tIT2", "TI!2", "TI 2", "TI\x002", "TI\xff2"} {
		cases = append(cases, malformedFrameCase{
			name: fmt.Sprintf("invalid identifier %q", id),
			body: frame(version, id, []byte{0, 'A'}), message: "malformed ID3 frame identifier",
		})
	}
	sizes := []uint32{3, 0x0fffffff}
	if version == 3 {
		sizes = append(sizes, 0x80000000)
	}
	for _, size := range sizes {
		oversized := frame(version, "TIT2", []byte{0, 'A'})
		binary.BigEndian.PutUint32(oversized[4:8], size)
		if version == 4 {
			// Encode independently of the production size helpers.
			for i := 7; i >= 4; i-- {
				oversized[i] = byte(size & 127)
				size >>= 7
			}
		}
		cases = append(cases, malformedFrameCase{
			name: fmt.Sprintf("frame extends beyond tag %x", oversized[4:8]),
			body: oversized, message: "malformed TIT2 frame size",
		})
	}
	if version == 4 {
		for offset := 4; offset < 8; offset++ {
			invalid := frame(version, "TIT2", []byte{0, 'A'})
			invalid[offset] |= 0x80
			cases = append(cases, malformedFrameCase{
				name: fmt.Sprintf("non-synchsafe size byte %d", offset-4),
				body: invalid, message: "malformed ID3 synchsafe size",
			})
		}
	}
	for bit := 0; bit < 8; bit++ {
		flagged := frame(version, "TIT2", []byte{0, 'A'})
		flagged[9] = 1 << bit
		cases = append(cases, malformedFrameCase{
			name: fmt.Sprintf("unsupported format flag 0x%02x", flagged[9]),
			body: flagged, message: fmt.Sprintf("unsupported TIT2 frame format flags: 0x%02x", flagged[9]),
		})
	}
	return cases
}

func TestPaddingAndNestedData(t *testing.T) {
	for _, version := range []byte{3, 4} {
		cases := []struct {
			name string
			body []byte
			line string
		}{
			{"only padding", []byte{0, 0, 0}, "No metadata found"},
			{"trailing padding", append(frame(version, "TIT2", []byte{0, 'A'}), 0, 0, 0), "Title [TIT2]: A"},
			{"chapter", frame(version, "CHAP", []byte{1, 2, 3}), "CHAP: unparsed data, 3 bytes"},
			{"chapter contents", frame(version, "CTOC", []byte{1, 2}), "CTOC: unparsed data, 2 bytes"},
		}
		for _, tt := range cases {
			t.Run(fmt.Sprintf("ID3v2.%d/%s", version, tt.name), func(t *testing.T) {
				path := save(t, fixture(version, tt.body))
				assertInspectionOutcome(t, path, 0, "File: "+path+"\n\n"+tt.line+"\n")
			})
		}
	}
}

func TestTagSizeLimit(t *testing.T) {
	// Use the documented 64 MiB boundary independently of maxTagSize.
	const limit = 64 << 20
	for _, version := range []byte{3, 4} {
		for _, bodySize := range []int{limit - 1, limit, limit + 1} {
			t.Run(fmt.Sprintf("ID3v2.%d/%d bytes", version, bodySize), func(t *testing.T) {
				path := sparseTag(t, version, bodySize)
				code := 0
				want := "File: " + path + "\n\nNo metadata found\n"
				if bodySize > limit {
					code = 1
					want = "Error: ID3 tag exceeds the 64 MiB inspection limit\n"
				}
				assertInspectionOutcome(t, path, code, want)
			})
		}
	}
}

func sparseTag(t *testing.T, version byte, bodySize int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "large.mp3")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	header := []byte{'I', 'D', '3', version, 0, 0, 0, 0, 0, 0}
	size := bodySize
	for i := 9; i >= 6; i-- {
		header[i] = byte(size & 127)
		size >>= 7
	}
	if _, err := file.Write(header); err != nil {
		t.Fatal(err)
	}
	// A sparse, zero-filled body exercises real file-size checks without a large fixture allocation.
	if err := file.Truncate(int64(len(header) + bodySize)); err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte{0xff, 0xfb, 0x90, 0}, int64(len(header)+bodySize)); err != nil {
		t.Fatal(err)
	}
	if err := file.Chmod(0o444); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertInspectionOutcome(t *testing.T, path string, wantCode int, wantOutput string) {
	t.Helper()
	before := inspectionSnapshot(t, path)
	var out, errOut bytes.Buffer
	code := run([]string{"inspect", path}, &out, &errOut)
	if code != wantCode {
		t.Errorf("exit code = %d; want %d", code, wantCode)
	}
	wantStdout, wantStderr := wantOutput, ""
	if wantCode != 0 {
		wantStdout, wantStderr = "", wantOutput
	}
	if got := out.String(); got != wantStdout {
		t.Errorf("stdout = %q; want %q", got, wantStdout)
	}
	if got := errOut.String(); got != wantStderr {
		t.Errorf("stderr = %q; want %q", got, wantStderr)
	}
	if after := inspectionSnapshot(t, path); before != after {
		t.Errorf("input changed: before %+v; after %+v", before, after)
	}
}

type fileSnapshot struct {
	size int64
	mode os.FileMode
	sum  [sha256.Size]byte
}

func inspectionSnapshot(t *testing.T, path string) fileSnapshot {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		t.Fatal(err)
	}
	return fileSnapshot{size: info.Size(), mode: info.Mode(), sum: [sha256.Size]byte(hash.Sum(nil))}
}
