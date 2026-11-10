package main

import (
	"bytes"
	"testing"
)

func FuzzInspectTag(f *testing.F) {
	f.Add([]byte("ID3"))
	f.Add([]byte{0xff, 0xfb, 0x90, 0x00})
	for _, version := range []byte{3, 4} {
		title := frame(version, "TIT2", []byte("\x00Title"))
		f.Add(fixture(version, title,
			frame(version, "TPE1", []byte("\x00Artist A")),
			frame(version, "TPE1", []byte("\x00Artist B")),
		))
		f.Add(fixture(version, title)[:tagHeaderSize+len(title)-1])
		f.Add(fixture(version, title[:len(title)-1]))
		f.Add(fixture(version, frame(version, "TXXX", []byte("\x00Key\x00Value"))))
		f.Add(fixture(version, frame(version, "COMM", []byte("\x00eng\x00Comment"))))
		f.Add(fixture(version, frame(version, "APIC", []byte("\x00image/jpeg\x00\x03\x00\xff\xd8\xff"))))
		f.Add(fixture(version, frame(version, "POPM", []byte("owner\x00\x80\x00\x00\x00\x01"))))
		f.Add(fixture(version, frame(version, "UFID", []byte("owner\x00\x01\x02"))))
		f.Add(fixture(version, frame(version, "CHAP", []byte("chapter\x00\x01"))))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		header, body, err := readTag(bytes.NewReader(data), int64(len(data)))
		if err == nil && header != nil {
			// Malformed inputs may return errors, but the parser must not panic.
			_, _ = parseFrames(header, body)
		}
	})
}
