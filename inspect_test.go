package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
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

func TestFrameValidationAndNestedData(t *testing.T) {
	invalidID := frame(3, "bad!", []byte{0, 'A'})
	zeroSize := frame(3, "TIT2", nil)
	overflow := frame(3, "TIT2", []byte{0, 'A'})
	binary.BigEndian.PutUint32(overflow[4:8], 100)
	flagged := frame(3, "TIT2", []byte{0, 'A'})
	flagged[9] = 0x80
	cases := []struct {
		name    string
		body    []byte
		code    int
		message string
	}{
		{"short frame header", []byte("TIT2"), 1, "frame header: truncated"},
		{"invalid identifier", invalidID, 1, "malformed ID3 frame identifier"},
		{"zero size", zeroSize, 1, "malformed TIT2 frame size"},
		{"oversized frame", overflow, 1, "malformed TIT2 frame size"},
		{"frame flags", flagged, 1, "unsupported TIT2 frame format flags"},
		{"invalid padding", []byte{0, 0, 1}, 1, "malformed ID3 padding"},
		{"valid padding", []byte{0, 0, 0}, 0, "No metadata found"},
		{"chapter", frame(3, "CHAP", []byte{1, 2, 3}), 0, "CHAP: unparsed data, 3 bytes"},
		{"chapter contents", frame(3, "CTOC", []byte{1, 2}), 0, "CTOC: unparsed data, 2 bytes"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			data := fixture(3, tt.body)
			path := save(t, data)
			var out, errOut bytes.Buffer
			code := run([]string{"inspect", path}, &out, &errOut)
			if code != tt.code || !strings.Contains(out.String()+errOut.String(), tt.message) {
				t.Fatalf("code %d; stdout=%s stderr=%s", code, &out, &errOut)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, after) {
				t.Fatal("input changed")
			}
		})
	}
}
