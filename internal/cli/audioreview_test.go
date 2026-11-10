package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/palharez/l1ner/internal/identify"
)

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
		exit <- runSongReview("preview", []string{"--from-audio", path}, terminal, terminal, &errOut, nil, audio)
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
