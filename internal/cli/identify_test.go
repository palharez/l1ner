package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/palharez/l1ner/internal/identify"
)

func TestIdentifyArgumentsAndConfiguration(t *testing.T) {
	t.Setenv("L1NER_ACOUSTID_KEY", "")
	for _, tc := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{"help", []string{"--help"}, 0, acoustIDRegistrationURL},
		{"invalid arguments", []string{"one.mp3", "two.mp3"}, 2, "expected one MP3 file path"},
		{"missing key", []string{"song.mp3"}, 1, "set L1NER_ACOUSTID_KEY to the application API key from " + acoustIDRegistrationURL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := Run(append([]string{"identify"}, tc.args...), strings.NewReader(""), &out, &errOut)
			if code != tc.code || !strings.Contains(out.String()+errOut.String(), tc.want) {
				t.Errorf("exit %d, stdout %q, stderr %q; want exit %d and %q", code, &out, &errOut, tc.code, tc.want)
			}
			if (tc.code == 0 && errOut.Len() != 0) || (tc.code != 0 && out.Len() != 0) {
				t.Errorf("unexpected output stream, stdout %q, stderr %q", &out, &errOut)
			}
		})
	}
}

func TestIdentifyCandidates(t *testing.T) {
	t.Setenv("L1NER_ACOUSTID_KEY", " test-app-key ")
	var out, errOut bytes.Buffer
	search := func(_ context.Context, path, key string) ([]identify.Candidate, error) {
		if path != "my song.mp3" || key != "test-app-key" {
			t.Errorf("unexpected search arguments %q and %q", path, key)
		}
		return []identify.Candidate{
			{RecordingID: "first-id", Title: "Song\x1b", Artist: "Artist", Score: 0.98},
			{RecordingID: "second-id", Score: 0.91},
		}, nil
	}
	code := runIdentify([]string{"my song.mp3"}, &out, &errOut, search)
	want := "File: my song.mp3\n\nMusicBrainz recording candidates from AcoustID:\n" +
		"1. Song\\x1b — Artist\n   Recording ID: first-id\n   AcoustID score: 0.9800\n" +
		"2. Unknown title — Unknown artist\n   Recording ID: second-id\n   AcoustID score: 0.9100\n"
	if code != 0 || out.String() != want || errOut.Len() != 0 {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 0, stdout %q and empty stderr", code, &out, &errOut, want)
	}
}

func TestIdentifyOutcomes(t *testing.T) {
	t.Setenv("L1NER_ACOUSTID_KEY", "test-app-key")
	for _, tc := range []struct {
		name      string
		searchErr error
		writeErr  error
		code      int
		want      string
	}{
		{"empty results", nil, nil, 0, "No MusicBrainz recording candidates found."},
		{"provider error", errors.New("service unavailable"), nil, 1, "Error: service unavailable"},
		{"output error", nil, errors.New("output closed"), 1, "Error: output closed"},
		{"canceled", context.Canceled, nil, 130, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			var writer io.Writer = &out
			if tc.writeErr != nil {
				writer = failingWriter{err: tc.writeErr}
			}
			search := func(context.Context, string, string) ([]identify.Candidate, error) {
				return nil, tc.searchErr
			}
			code := runIdentify([]string{"song.mp3"}, writer, &errOut, search)
			if code != tc.code || !strings.Contains(out.String()+errOut.String(), tc.want) {
				t.Errorf("exit %d, stdout %q, stderr %q; want exit %d and %q", code, &out, &errOut, tc.code, tc.want)
			}
		})
	}
}
