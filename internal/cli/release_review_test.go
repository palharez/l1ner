package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestReleaseReviewTerminal(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		directory     bool
	}{
		{name: "preview", command: "preview"},
		{name: "fix a file", command: "fix"},
		{name: "fix a directory", command: "fix", directory: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			song := filepath.Join(dir, "a.mp3")
			other := filepath.Join(dir, "b.mp3")
			unrelated := frame(3, "XABC", []byte{4, 5, 6})
			original := fixture(3, frame(3, "TIT2", []byte("\x00Song")), frame(3, "TPE1", []byte("\x00Artist")),
				frame(3, "TALB", []byte("\x00Old album")), frame(3, "TRCK", []byte("\x009/12")), unrelated)
			if err := os.WriteFile(song, original, 0o600); err != nil {
				t.Fatal(err)
			}
			input := song
			if tc.directory {
				input = dir
				if err := os.WriteFile(other, original, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var releaseLookups []string
			searches := reviewSearches{
				recording: func(context.Context, string, string) ([]recording, error) {
					return []recording{{id: "recording", title: "Song", artist: "Artist"}}, nil
				},
				releases: func(_ context.Context, id string) ([]releaseTrack, error) {
					releaseLookups = append(releaseLookups, id)
					return []releaseTrack{
						{id: "album-id", album: "First album", albumArtist: "Artist", date: "1960", disc: 1, track: 6},
						{id: "compilation-id", album: "Compilation", albumArtist: "Various Artists", date: "2000", disc: 2, track: 9},
					}, nil
				},
			}
			master, terminal := previewTestTerminal(t)
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			var errOut bytes.Buffer
			exit := make(chan int, 1)
			go func() {
				exit <- runSongReview(tc.command, []string{"--release", input}, terminal, terminal, &errOut, searches, nil)
			}()
			send := func(keys string) {
				t.Helper()
				if _, err := io.WriteString(master, keys); err != nil {
					t.Fatal(err)
				}
			}
			readPreviewOutput(t, ctx, master, "? Choose a recording")
			send("\r")
			readPreviewOutput(t, ctx, master, "  Quit")
			send("\r")
			first := readPreviewOutput(t, ctx, master, "Choose another release")
			for _, want := range []string{"MusicBrainz release: album-id", `"First album"`, `"6"`} {
				if !strings.Contains(first, want) {
					t.Errorf("first proposal omitted %q: %s", want, first)
				}
			}
			send("\x1b[A\r")
			readPreviewOutput(t, ctx, master, "  Quit")
			send("\x1b[B\r")
			second := readPreviewOutput(t, ctx, master, "❯ Skip song")
			for _, want := range []string{"MusicBrainz release: compilation-id", `"Compilation"`, `"Various Artists"`, `"9"`, `"2"`} {
				if !strings.Contains(second, want) {
					t.Errorf("second proposal omitted %q: %s", want, second)
				}
			}
			assertFileBytes(t, song, original)
			assertNoReleaseBackup(t, song)
			send("\x1b[B\r")
			if tc.directory {
				readPreviewOutput(t, ctx, master, "Song 2 of 2")
				readPreviewOutput(t, ctx, master, "? Choose a recording")
				send("\r")
				readPreviewOutput(t, ctx, master, "  Quit")
				send("\r")
				readPreviewOutput(t, ctx, master, "❯ Skip song")
				send("\r")
				readPreviewOutput(t, ctx, master, "Summary: 1 applied, 1 skipped, 0 unchanged, 0 unresolved, 0 failed, 0 pending")
			}
			select {
			case code := <-exit:
				if code != 0 || errOut.Len() != 0 {
					t.Fatalf("exit = %d, stderr = %q", code, &errOut)
				}
			case <-ctx.Done():
				t.Fatal("review did not exit")
			}
			wantLookups := []string{"recording"}
			if tc.directory {
				wantLookups = append(wantLookups, "recording")
				assertFileBytes(t, other, original)
				assertNoReleaseBackup(t, other)
			}
			if !slices.Equal(releaseLookups, wantLookups) {
				t.Errorf("release lookups = %v, want %v", releaseLookups, wantLookups)
			}
			if tc.command == "preview" {
				assertFileBytes(t, song, original)
				assertNoReleaseBackup(t, song)
				return
			}
			assertFileBytes(t, song+backupSuffix, original)
			updated, err := os.ReadFile(song)
			if err != nil {
				t.Fatal(err)
			}
			header, body, err := readTag(bytes.NewReader(updated), int64(len(updated)))
			if err != nil || len(header) != tagHeaderSize || header[3] != 3 {
				t.Fatalf("unexpected edited tag: %x (%v)", header, err)
			}
			if !bytes.Contains(body, unrelated) || !bytes.Equal(updated[len(header)+len(body):], original[len(original)-4:]) {
				t.Error("unrelated frame or audio changed")
			}
			entries, err := parseFrames(header, body)
			if err != nil {
				t.Fatal(err)
			}
			for id, want := range map[string]string{"TIT2": "Song", "TPE1": "Artist", "TALB": "Compilation", "TPE2": "Various Artists", "TRCK": "9", "TPOS": "2"} {
				if got := songTagValues(entries, id); !slices.Equal(got, []string{want}) {
					t.Errorf("%s = %v, want %q", id, got, want)
				}
			}
		})
	}
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, want) {
		t.Errorf("%s differs from the expected bytes: %v", path, err)
	}
}

func assertNoReleaseBackup(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path + backupSuffix); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("unexpected backup for %s: %v", path, err)
	}
}
