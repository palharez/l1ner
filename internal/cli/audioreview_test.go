package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/palharez/l1ner/internal/identify"
)

func TestFixFromAudioTerminal(t *testing.T) {
	t.Setenv("L1NER_ACOUSTID_KEY", "test-key")
	audio := []byte{0xff, 0xfb, 0x90, 0, 1, 2, 3}
	unknown := frame(3, "XABC", []byte{4, 5, 6})
	for _, tc := range []struct {
		name     string
		original []byte
		version  byte
		apply    bool
	}{
		{name: "tagless apply", original: audio, version: 4, apply: true},
		{name: "tagless skip", original: audio},
		{name: "missing artist in ID3v2.3", original: append(fixture(3, frame(3, "TIT2", []byte("\x00Wrong title")), unknown), audio[4:]...), version: 3, apply: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "song.mp3")
			if err := os.WriteFile(path, tc.original, 0o640); err != nil {
				t.Fatal(err)
			}
			before := inspectionSnapshot(t, path)
			master, terminal := previewTestTerminal(t)
			var errOut bytes.Buffer
			exit := make(chan int, 1)
			go func() {
				search := func(_ context.Context, file *os.File, _ string) ([]identify.Candidate, error) {
					got, err := io.ReadAll(file)
					if err != nil || !bytes.Equal(got, tc.original) {
						return nil, fmt.Errorf("wrong fingerprint source: %x, %v", got, err)
					}
					return []identify.Candidate{{RecordingID: "id", Title: "Title", Artist: "Artist", Score: 0.99}}, nil
				}
				exit <- runSongReview("fix", []string{"--from-audio", path}, terminal, terminal, &errOut, nil, search, nil)
			}()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			readPreviewOutput(t, ctx, master, "? Choose a recording")
			if _, err := io.WriteString(master, "\r"); err != nil {
				t.Fatal(err)
			}
			readPreviewOutput(t, ctx, master, "❯ Skip song")
			if got := inspectionSnapshot(t, path); got != before {
				t.Errorf("MP3 changed before approval: %+v", got)
			}
			if _, err := os.Lstat(path + backupSuffix); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("backup exists before approval: %v", err)
			}
			keys := "\r"
			if tc.apply {
				keys = "\x1b[B\r"
			}
			if _, err := io.WriteString(master, keys); err != nil {
				t.Fatal(err)
			}
			select {
			case code := <-exit:
				if code != 0 || errOut.Len() != 0 {
					t.Fatalf("exit = %d, stderr = %q", code, &errOut)
				}
			case <-ctx.Done():
				t.Fatal("fix did not exit")
			}
			assertNoFixStages(t, path)
			if !tc.apply {
				if got := inspectionSnapshot(t, path); got != before {
					t.Errorf("skip changed the MP3: %+v", got)
				}
				if _, err := os.Lstat(path + backupSuffix); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("backup exists after skip: %v", err)
				}
				return
			}
			if backup := inspectionSnapshot(t, path+backupSuffix); backup != before {
				t.Errorf("backup differs from the original: %+v", backup)
			}
			updated, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			header, body, err := readTag(bytes.NewReader(updated), int64(len(updated)))
			if err != nil || len(header) != tagHeaderSize || header[3] != tc.version {
				t.Fatalf("unexpected header %x: %v", header, err)
			}
			if !bytes.Equal(updated[len(header)+len(body):], audio) || (tc.version == 3 && !bytes.Contains(body, unknown)) {
				t.Error("audio or unrelated frame changed")
			}
			tags, err := readLookupTags(path)
			if err != nil {
				t.Fatal(err)
			}
			if title, artist := tags.searchTerms(); title != "Title" || artist != "Artist" {
				t.Errorf("edited tags = (%q, %q)", title, artist)
			}
		})
	}
}

func TestAudioPreviewRejectsDirectory(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"preview", "--from-audio", t.TempDir()}, nil, &out, &errOut)
	if code != 2 || out.Len() != 0 || !strings.Contains(errOut.String(), "--from-audio requires one MP3 file") {
		t.Errorf("exit = %d, stdout = %q, stderr = %q", code, &out, &errOut)
	}
}

func TestFixFromAudioWithoutCompleteCandidate(t *testing.T) {
	t.Setenv("L1NER_ACOUSTID_KEY", "test-key")
	path, _ := saveFixFixture(t)
	before := inspectionSnapshot(t, path)
	search := func(context.Context, *os.File, string) ([]identify.Candidate, error) {
		return []identify.Candidate{{RecordingID: "partial", Title: "Song"}}, nil
	}
	choose := func(context.Context, string, []menuOption, int) (int, error) {
		t.Fatal("opened the picker without an eligible candidate")
		return 0, nil
	}
	var out bytes.Buffer
	decision, err := fixSong(t.Context(), path, &out, nil, search, choose)
	if err != nil || decision != reviewUnresolved || !strings.Contains(out.String(), "No AcoustID recording candidates with both title and artist found.") {
		t.Errorf("decision = %v, error = %v, output = %q", decision, err, &out)
	}
	if after := inspectionSnapshot(t, path); after != before {
		t.Errorf("unresolved file changed: %+v", after)
	}
	if _, err := os.Lstat(path + backupSuffix); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("backup exists for an unresolved file: %v", err)
	}
}

func TestPreviewFromAudioTerminal(t *testing.T) {
	t.Setenv("L1NER_ACOUSTID_KEY", "test-key")
	original := []byte{0xff, 0xfb, 0x90, 0}
	path := save(t, original)
	master, terminal := previewTestTerminal(t)
	var errOut bytes.Buffer
	exit := make(chan int, 1)
	go func() {
		audio := func(_ context.Context, file *os.File, key string) ([]identify.Candidate, error) {
			got, err := io.ReadAll(file)
			if err != nil || !bytes.Equal(got, original) || key != "test-key" {
				t.Errorf("fingerprint input = %x, key = %q, error = %v", got, key, err)
			}
			return []identify.Candidate{
				{RecordingID: "partial", Title: "Incomplete", Score: 1},
				{RecordingID: "studio", Title: "Song", Artist: "Artist", Score: 0.99},
				{RecordingID: "live", Title: "Song (Live)", Artist: "Artist", Score: 0.97},
			}, nil
		}
		exit <- runSongReview("preview", []string{"--from-audio", path}, terminal, terminal, &errOut, nil, audio, nil)
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	first := readPreviewOutput(t, ctx, master, "  Quit")
	if !strings.Contains(first, "AcoustID score: 0.9900") || strings.Contains(first, "Incomplete") {
		t.Errorf("unexpected candidates: %q", first)
	}
	if _, err := io.WriteString(master, "\x1b[B\r"); err != nil {
		t.Fatal(err)
	}
	proposal := readPreviewOutput(t, ctx, master, "❯ Skip song")
	for _, want := range []string{"MusicBrainz recording: live", "(missing)", `"Song (Live)"`, "added"} {
		if !strings.Contains(proposal, want) {
			t.Errorf("proposal is missing %q: %q", want, proposal)
		}
	}
	if _, err := io.WriteString(master, "\x1b[B\r"); err != nil {
		t.Fatal(err)
	}
	readPreviewOutput(t, ctx, master, "Accepted for this preview:")
	select {
	case code := <-exit:
		if code != 0 || errOut.Len() != 0 {
			t.Fatalf("exit = %d, stderr = %q", code, &errOut)
		}
	case <-ctx.Done():
		t.Fatal("preview did not exit")
	}
	if current, err := os.ReadFile(path); err != nil || !bytes.Equal(current, original) {
		t.Errorf("preview changed the MP3: %v", err)
	}
}
