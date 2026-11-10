package main

import (
	"fmt"
	"testing"
)

func TestReadLookupTags(t *testing.T) {
	for _, version := range []byte{3, 4} {
		for _, tc := range []struct {
			name      string
			titles    []string
			artist    string
			wantError string
		}{
			{"identical title", []string{" Title ", "  ", "Title"}, " Artist ", ""},
			{"blank title", []string{"  "}, "Artist", "missing title tag (TIT2)"},
			{"blank artist", []string{"Title"}, "  ", "missing artist tag (TPE1)"},
			{"conflicting titles", []string{"Different", "Title"}, "Artist", "conflicting title tags (TIT2)"},
			{"multiple artists", []string{"Title"}, "Artist\x00Other", "conflicting artist tags (TPE1)"},
		} {
			t.Run(fmt.Sprintf("v2.%d/%s", version, tc.name), func(t *testing.T) {
				frames := [][]byte{
					frame(version, "TPE1", []byte("\x00"+tc.artist)),
					frame(version, "TPE2", []byte("\x00Album artist")),
				}
				for _, title := range tc.titles {
					frames = append(frames, frame(version, "TIT2", []byte("\x00"+title)))
				}
				path := save(t, fixture(version, frames...))
				title, artist, err := readLookupTags(path)
				if tc.wantError != "" {
					if err == nil || err.Error() != tc.wantError {
						t.Errorf("readLookupTags error = %v, want %q", err, tc.wantError)
					}
					return
				}
				if err != nil || title != "Title" || artist != "Artist" {
					t.Errorf("readLookupTags = (%q, %q, %v), want (Title, Artist, nil)", title, artist, err)
				}
			})
		}
	}
}
