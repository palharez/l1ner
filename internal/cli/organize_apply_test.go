package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOrganizeApplyTerminal(t *testing.T) {
	root := t.TempDir()
	data := albumFixture("Artist", "Album", "Artist")
	applied := batchFile(t, root, "a.mp3", data)
	skipped := batchFile(t, root, "b.mp3", data)
	pending := batchFile(t, root, "c.mp3", data)
	backup := batchFile(t, root, "a.mp3.l1ner.bak", []byte("tag backup"))
	master, terminal := previewTestTerminal(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var errOut bytes.Buffer
	exit := make(chan int, 1)
	go func() {
		exit <- Run([]string{"organize", "--apply", root}, terminal, terminal, &errOut)
	}()
	readPreviewOutput(t, ctx, master, "❯ Skip song")
	assertFileBytes(t, applied, data)
	journals, err := filepath.Glob(filepath.Join(root, ".l1ner-organize-*.jsonl"))
	if err != nil || len(journals) != 0 {
		t.Errorf("journals before approval = %v, error = %v", journals, err)
	}
	for _, step := range []struct{ keys, wait string }{
		{"\x1b[B\r", "Moved song."},
		{"", "❯ Skip song"},
		{"\r", "Skipped song."},
		{"", "❯ Skip song"},
		{"\x1b[A\r", "Summary: 1 moved, 1 skipped, 0 blocked, 0 failed, 1 pending, 0 scan errors"},
	} {
		if step.keys != "" {
			if _, err := io.WriteString(master, step.keys); err != nil {
				t.Fatal(err)
			}
		}
		readPreviewOutput(t, ctx, master, step.wait)
	}
	select {
	case code := <-exit:
		if code != 0 || errOut.Len() != 0 {
			t.Errorf("exit = %d, stderr = %q", code, &errOut)
		}
	case <-ctx.Done():
		t.Fatal("organization did not exit")
	}
	if _, err := os.Lstat(applied); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("approved source still exists: %v", err)
	}
	assertFileBytes(t, filepath.Join(root, "Artist/Album/a.mp3"), data)
	assertFileBytes(t, skipped, data)
	assertFileBytes(t, pending, data)
	assertFileBytes(t, backup, []byte("tag backup"))
	if journals, err := filepath.Glob(filepath.Join(root, ".l1ner-organize-*.jsonl")); err != nil || len(journals) != 1 {
		t.Errorf("journals after approval = %v, error = %v", journals, err)
	}
}
