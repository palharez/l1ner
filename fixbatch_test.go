package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFixDirectoryTerminal(t *testing.T) {
	dir := t.TempDir()
	files := []struct{ name, title string }{
		{"a-apply.MP3", "lowercase title"},
		{"b-skip.mp3", "skip title"},
		{"c-fail.mp3", "service failure"},
		{"d-unchanged.mp3", "Already matches"},
		{"e-unresolved.mp3", "no candidates"},
		{"f-quit.mp3", "quit title"},
		{"g-pending.mp3", "pending title"},
	}
	originals := make(map[string][]byte)
	// Create files in reverse order to check lexical review order.
	for i := len(files) - 1; i >= 0; i-- {
		file := files[i]
		path := filepath.Join(dir, file.name)
		originals[path] = writeBatchSong(t, path, file.title)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o700); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "subdir", "nested.mp3")
	originals[nested] = writeBatchSong(t, nested, "nested title")
	if err := os.Symlink(filepath.Join(dir, files[0].name), filepath.Join(dir, "aa-alias.mp3")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("ignore"), 0o600); err != nil {
		t.Fatal(err)
	}
	queries := make(chan string, len(files)+1)
	client := testMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		queries <- query
		switch {
		case strings.Contains(query, `"service failure"`):
			w.WriteHeader(http.StatusServiceUnavailable)
		case strings.Contains(query, `"no candidates"`):
			io.WriteString(w, `{"recordings":[]}`)
		case strings.Contains(query, `"Already matches"`):
			io.WriteString(w, `{"recordings":[{"id":"recording","title":"Already matches","artist-credit":[{"name":"Artist"}]}]}`)
		default:
			io.WriteString(w, `{"recordings":[{"id":"recording","title":"Corrected title","artist-credit":[{"name":"Artist"}]}]}`)
		}
	})
	master, terminal := previewTestTerminal(t)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	var errOut bytes.Buffer
	exit := make(chan int, 1)
	go func() {
		exit <- runCommand([]string{"fix", dir}, terminal, terminal, &errOut, client.search)
	}()
	for _, step := range []struct{ wait, keys string }{
		{"Song 1 of 7", ""},
		{"? Choose a recording", "\r"},
		{"❯ Skip song", "\x1b[B\r"},
		{"Applied approved tags", ""},
		{"Song 2 of 7", ""},
		{"? Choose a recording", "\r"},
		{"❯ Skip song", "\r"},
		{"Skipped song.", ""},
		{"Song 3 of 7", ""},
		{"Song 4 of 7", ""},
		{"? Choose a recording", "\r"},
		{"The title and artist already agree.", ""},
		{"Song 5 of 7", ""},
		{"No MusicBrainz recording candidates found.", ""},
		{"Song 6 of 7", ""},
		{"? Choose a recording", "\x1b[A\r"},
		{"Review ended.", ""},
		{"Summary: 1 applied, 1 skipped, 1 unchanged, 1 unresolved, 1 failed, 2 pending", ""},
	} {
		readPreviewOutput(t, ctx, master, step.wait)
		if step.keys != "" {
			if _, err := io.WriteString(master, step.keys); err != nil {
				t.Fatal(err)
			}
		}
	}
	select {
	case code := <-exit:
		if code != 1 || !strings.Contains(errOut.String(), "c-fail.mp3") || !strings.Contains(errOut.String(), "HTTP 503") {
			t.Errorf("exit = %d, stderr = %q, want one service failure", code, &errOut)
		}
	case <-ctx.Done():
		t.Fatal("batch did not exit")
	}
	if got := len(queries); got != 6 {
		t.Fatalf("requests = %d, want 6", got)
	}
	for _, file := range files[:6] {
		if got, want := <-queries, fmt.Sprintf(`recording:"%s" AND artist:"Artist"`, file.title); got != want {
			t.Errorf("query = %q, want %q", got, want)
		}
	}
	assertBatchFiles(t, originals, filepath.Join(dir, files[0].name))
}

func writeBatchSong(t *testing.T, path, title string) []byte {
	t.Helper()
	data := fixture(3, frame(3, "TIT2", []byte("\x00"+title)), frame(3, "TPE1", []byte("\x00Artist")))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return data
}

func assertBatchFiles(t *testing.T, originals map[string][]byte, applied string) {
	t.Helper()
	for path, original := range originals {
		if path == applied {
			if backup, err := os.ReadFile(path + backupSuffix); err != nil || !bytes.Equal(backup, original) {
				t.Errorf("backup differs from original: %v", err)
			}
			tags, err := readLookupTags(path)
			if err != nil {
				t.Fatal(err)
			}
			if title, artist := tags.searchTerms(); title != "Corrected title" || artist != "Artist" {
				t.Errorf("edited tags = (%q, %q)", title, artist)
			}
			continue
		}
		if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, original) {
			t.Errorf("%s changed without approval: %v", path, err)
		}
		if _, err := os.Lstat(path + backupSuffix); !os.IsNotExist(err) {
			t.Errorf("%s has a backup without approval: %v", path, err)
		}
	}
}

func TestFixDirectoryEmptyOrSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	search := func(context.Context, string, string) ([]recording, error) {
		t.Fatal("empty or symlink directory reached MusicBrainz")
		return nil, nil
	}
	var out, errOut bytes.Buffer
	if code := runFixDirectory(dir, &out, &errOut, search, nil); code != 0 || !strings.Contains(out.String(), "No MP3 files found.") || errOut.Len() != 0 {
		t.Errorf("empty directory exit = %d, stdout = %q, stderr = %q", code, &out, &errOut)
	}
	out.Reset()
	if code := runFixDirectory(link, &out, &errOut, search, nil); code != 1 || !strings.Contains(errOut.String(), "directory path is a symbolic link") {
		t.Errorf("symlink directory exit = %d, stdout = %q, stderr = %q", code, &out, &errOut)
	}
}

func TestFixBatchCancellationKeepsEarlierApproval(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	originals := make(map[string][]byte)
	for _, title := range []string{"first", "second", "third"} {
		path := filepath.Join(dir, title+".mp3")
		paths = append(paths, path)
		originals[path] = writeBatchSong(t, path, title)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	search := func(ctx context.Context, title, _ string) ([]recording, error) {
		switch title {
		case "first":
			return []recording{{id: "recording", title: "Corrected title", artist: "Artist"}}, nil
		case "second":
			cancel()
			return nil, ctx.Err()
		default:
			t.Fatalf("looked up %q after canceling the batch", title)
			return nil, nil
		}
	}
	choose := func(_ context.Context, question string, _ []menuOption, _ int) (int, error) {
		if question == "Review this proposal" {
			return int(reviewAccept), nil
		}
		return 0, nil
	}
	var out, errOut bytes.Buffer
	summary, err := fixBatch(ctx, paths, &out, &errOut, search, choose)
	if !errors.Is(err, context.Canceled) || summary != (fixBatchSummary{applied: 1, pending: 2}) || errOut.Len() != 0 {
		t.Errorf("summary = %+v, error = %v, stderr = %q", summary, err, &errOut)
	}
	assertBatchFiles(t, originals, paths[0])
}
