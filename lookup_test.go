package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
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
				tags, err := readLookupTags(path)
				if tc.wantError != "" {
					if err == nil || err.Error() != tc.wantError {
						t.Errorf("readLookupTags error = %v, want %q", err, tc.wantError)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if title, artist := tags.searchTerms(); title != "Title" || artist != "Artist" {
					t.Errorf("search terms = (%q, %q), want (Title, Artist)", title, artist)
				}
			})
		}
	}
}

func TestLookupCommand(t *testing.T) {
	path := save(t, fixture(4,
		frame(4, "TIT2", []byte("\x03Example")),
		frame(4, "TPE1", []byte("\x03An Artist")),
		frame(4, "TALB", []byte("\x03Local Album")),
	))
	before := inspectionSnapshot(t, path)
	client := testMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("query"), `recording:"Example" AND artist:"An Artist"`; got != want {
			t.Errorf("query = %q, want %q", got, want)
		}
		io.WriteString(w, `{"recordings":[
			{"id":"studio","title":"Example\u001b[2J","artist-credit":[{"name":"An Artist"}]},
			{"id":"live","title":"Example\u001b[2J","artist-credit":[{"name":"An Artist"}]}
		]}`)
	})
	var out, errOut bytes.Buffer
	code := runWithLookup([]string{"lookup", path}, &out, &errOut, client.search)
	if code != 0 || errOut.Len() != 0 {
		t.Fatalf("lookup exit = %d, stderr = %q", code, &errOut)
	}
	want := "File: " + path + "\nSearch: Example — An Artist\n\nMusicBrainz recording candidates:\n" +
		"1. Example\\x1b[2J — An Artist\n   Recording ID: studio\n" +
		"2. Example\\x1b[2J — An Artist\n   Recording ID: live\n"
	if out.String() != want {
		t.Errorf("lookup stdout = %q, want %q", &out, want)
	}
	if after := inspectionSnapshot(t, path); after != before {
		t.Errorf("file changed: before %+v, after %+v", before, after)
	}
}

func TestLookupWithoutRequest(t *testing.T) {
	var requests atomic.Int32
	client := testMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		io.WriteString(w, `{"recordings":[]}`)
	})
	missingArtist := save(t, fixture(3, frame(3, "TIT2", []byte("\x00Title"))))
	conflictingTitles := save(t, fixture(4,
		frame(4, "TIT2", []byte("\x03Title\x00Other")),
		frame(4, "TPE1", []byte("\x03Artist")),
	))
	for _, tc := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{"missing path", []string{"lookup"}, 2, "expected one MP3 file path"},
		{"multiple paths", []string{"lookup", "a.mp3", "b.mp3"}, 2, "expected one MP3 file path"},
		{"unsupported flag", []string{"lookup", "--json", "a.mp3"}, 2, "flag provided but not defined"},
		{"missing artist", []string{"lookup", missingArtist}, 1, "missing artist tag (TPE1)"},
		{"conflicting titles", []string{"lookup", conflictingTitles}, 1, "conflicting title tags (TIT2)"},
		{"help", []string{"lookup", "--help"}, 0, "embedded title and artist"},
		{"inspect", []string{"inspect", missingArtist}, 0, "Title [TIT2]: Title"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := runWithLookup(tc.args, &out, &errOut, client.search)
			if code != tc.code {
				t.Errorf("exit = %d, want %d", code, tc.code)
			}
			got, unexpected := out.String(), errOut.String()
			if tc.code != 0 {
				got, unexpected = unexpected, got
			}
			if !strings.Contains(got, tc.want) || unexpected != "" {
				t.Errorf("stdout = %q, stderr = %q, want %q", &out, &errOut, tc.want)
			}
		})
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("requests = %d, want 0", got)
	}
}

func TestLookupCommandOutcomes(t *testing.T) {
	path := save(t, fixture(3, frame(3, "TIT2", []byte("\x00Title")), frame(3, "TPE1", []byte("\x00Artist"))))
	for _, tc := range []struct {
		name   string
		status int
		write  error
		code   int
		want   string
	}{
		{"no candidates", http.StatusOK, nil, 0, "No MusicBrainz recording candidates found."},
		{"service unavailable", http.StatusServiceUnavailable, nil, 1, "MusicBrainz is busy (HTTP 503)"},
		{"write failure", http.StatusOK, errors.New("output closed"), 1, "output closed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := testMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				io.WriteString(w, `{"recordings":[]}`)
			})
			var out, errOut bytes.Buffer
			var writer io.Writer = &out
			if tc.write != nil {
				writer = failingWriter{err: tc.write}
			}
			code := runWithLookup([]string{"lookup", path}, writer, &errOut, client.search)
			if code != tc.code {
				t.Errorf("exit = %d, want %d", code, tc.code)
			}
			got, unexpected := out.String(), errOut.String()
			if tc.code != 0 {
				got, unexpected = unexpected, got
			}
			if !strings.Contains(got, tc.want) || unexpected != "" {
				t.Errorf("stdout = %q, stderr = %q, want %q", &out, &errOut, tc.want)
			}
		})
	}
}
