package cli

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestRewriteTagPreservesFrames(t *testing.T) {
	for _, version := range []byte{3, 4} {
		t.Run(fmt.Sprintf("v2.%d", version), func(t *testing.T) {
			artist := frame(version, "TPE1", []byte("\x00Artist"))
			unknown := frame(version, "XABC", []byte{1, 0, 0xff})
			chapter := frame(version, "CHAP", []byte{1, 2, 3})
			original := fixture(version, frame(version, "TIT2", []byte("\x00Title")), unknown, artist,
				frame(version, "TIT2", []byte("\x00 Title ")), chapter, make([]byte, 128))
			header, body, err := readTag(bytes.NewReader(original), int64(len(original)))
			if err != nil {
				t.Fatal(err)
			}
			proposed := "Títuló 世界"
			if version == 4 {
				proposed += " 🎵"
			}
			proposal := proposeSong("sample.mp3", songTags{titles: []string{"Title", " Title "}, artists: []string{"Artist"}}, recording{title: proposed, artist: "Artist"})
			tag, err := rewriteTag(header, body, proposal)
			if err != nil {
				t.Fatal(err)
			}
			if len(tag) != len(header)+len(body) {
				t.Fatal("audio offset changed despite available padding")
			}
			newHeader, newBody, err := readTag(bytes.NewReader(tag), int64(len(tag)))
			if err != nil {
				t.Fatal(err)
			}
			if newHeader[3] != version {
				t.Errorf("tag version = %d, want %d", newHeader[3], version)
			}
			entries, err := parseFrames(newHeader, newBody)
			if err != nil {
				t.Fatal(err)
			}
			var ids, titles []string
			for _, item := range entries {
				ids = append(ids, item.id)
				if item.id == "TIT2" {
					_, value := describe(item)
					titles = append(titles, value)
				}
			}
			if !slices.Equal(ids, []string{"TIT2", "XABC", "TPE1", "TIT2", "CHAP"}) || !slices.Equal(titles, []string{proposed, proposed}) {
				t.Errorf("frame IDs = %v, titles = %q", ids, titles)
			}
			for _, unchanged := range [][]byte{artist, unknown, chapter} {
				if !bytes.Contains(newBody, unchanged) {
					t.Errorf("changed original frame bytes for %q", unchanged[:4])
				}
			}
		})
	}
}

func TestRewriteTagRejectsUnsafeEdits(t *testing.T) {
	protected := frame(3, "TIT2", []byte("\x00Title"))
	protected[8] = 0x20
	for _, tc := range []struct {
		name, title, message string
		frames               [][]byte
	}{
		{"protected", "New title", "protected or flagged TIT2", [][]byte{protected}},
		{"offsets", "A longer title", "cannot grow a tag with a CHAP", [][]byte{frame(3, "TIT2", []byte("\x00Title")), frame(3, "CHAP", []byte{1})}},
		{"unsupported Unicode", "Song 🎵", "outside the Basic Multilingual Plane", [][]byte{frame(3, "TIT2", []byte("\x00Title"))}},
		{"separator", "Title\x00Other", "without control characters", [][]byte{frame(3, "TIT2", []byte("\x00Title"))}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := fixture(3, tc.frames...)
			header, body, err := readTag(bytes.NewReader(original), int64(len(original)))
			if err != nil {
				t.Fatal(err)
			}
			proposal := songProposal{title: tagComparison{current: []string{"Title"}, proposed: tc.title}}
			if _, err := rewriteTag(header, body, proposal); err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Errorf("rewriteTag error = %v, want %q", err, tc.message)
			}
		})
	}
}
