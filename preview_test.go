package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDisplaySongProposal(t *testing.T) {
	for _, tc := range []struct {
		name        string
		titles      []string
		wantChanged bool
		want        []string
	}{
		{"different title", []string{"song"}, true, []string{`"song"`, `"Song"`, "changed"}},
		{"spacing in a repeated tag", []string{"Song", " Song "}, true, []string{`"Song", " Song "`, "changed"}},
		{"same tags", []string{"Song"}, false, []string{"The title and artist already agree."}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frames := [][]byte{frame(4, "TPE1", []byte("\x03Artist"))}
			for _, title := range tc.titles {
				frames = append(frames, frame(4, "TIT2", []byte("\x03"+title)))
			}
			path := save(t, fixture(4, frames...))
			tags, err := readLookupTags(path)
			if err != nil {
				t.Fatal(err)
			}
			proposal := proposeSong(path, tags, recording{id: "example-id", title: "Song", artist: "Artist"})
			if proposal.changed() != tc.wantChanged {
				t.Errorf("proposal.changed() = %t, want %t", proposal.changed(), tc.wantChanged)
			}
			var output bytes.Buffer
			if err := displayProposal(&output, proposal); err != nil {
				t.Fatal(err)
			}
			for _, want := range append(tc.want, path, "example-id", "Current", "Proposed") {
				if !strings.Contains(output.String(), want) {
					t.Errorf("preview = %q, missing %q", output.String(), want)
				}
			}
		})
	}
}
