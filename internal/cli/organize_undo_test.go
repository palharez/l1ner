package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrganizeUndoRetry(t *testing.T) {
	root := t.TempDir()
	data := albumFixture("Artist", "Album", "Track Artist")
	first := batchFile(t, root, "a.mp3", data)
	second := batchFile(t, root, "b.mp3", data)
	backup := batchFile(t, root, "a.mp3.l1ner.bak", []byte("original tag backup"))
	// The artist folder already exists. Undo may remove the album folder it creates.
	if err := os.Mkdir(filepath.Join(root, "Artist"), 0o700); err != nil {
		t.Fatal(err)
	}
	journal := applyTestAlbumFolder(t, root)
	conflict := []byte("occupied original path")
	batchFile(t, root, "a.mp3", conflict)
	args := []string{"organize", "--undo", journal}
	var out, errOut bytes.Buffer
	if code := run(args, &out, &errOut); code != 1 ||
		!strings.Contains(out.String(), "1 restored, 0 already resolved, 1 blocked, 0 failed") ||
		!strings.Contains(errOut.String(), "original path is occupied") {
		t.Fatalf("partial undo exit = %d, stdout = %q, stderr = %q", code, &out, &errOut)
	}
	assertFileBytes(t, first, conflict)
	assertFileBytes(t, second, data)
	assertFileBytes(t, filepath.Join(root, "Artist/Album/a.mp3"), data)
	assertFileBytes(t, backup, []byte("original tag backup"))
	if err := os.Remove(first); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	if code := run(args, &out, &errOut); code != 0 ||
		!strings.Contains(out.String(), "1 restored, 1 already resolved, 0 blocked, 0 failed") || errOut.Len() != 0 {
		t.Fatalf("retry exit = %d, stdout = %q, stderr = %q", code, &out, &errOut)
	}
	assertFileBytes(t, first, data)
	assertFileBytes(t, backup, []byte("original tag backup"))
	if _, err := os.Stat(filepath.Join(root, "Artist/Album")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("album folder after undo: %v", err)
	}
	if info, err := os.Stat(filepath.Join(root, "Artist")); err != nil || !info.IsDir() {
		t.Errorf("preexisting artist folder was lost: %v", err)
	}
	nextJournal := applyTestAlbumFolder(t, root)
	if nextJournal == journal {
		t.Fatal("second apply reused the old journal")
	}
	out.Reset()
	if code := run(args, &out, &errOut); code != 0 ||
		!strings.Contains(out.String(), "0 restored, 2 already resolved, 0 blocked, 0 failed") {
		t.Errorf("repeated undo exit = %d, stdout = %q, stderr = %q", code, &out, &errOut)
	}
	assertFileBytes(t, filepath.Join(root, "Artist/Album/a.mp3"), data)
	assertFileBytes(t, filepath.Join(root, "Artist/Album/b.mp3"), data)
}

func TestOrganizeUndoInterruptedJournal(t *testing.T) {
	root := t.TempDir()
	data := albumFixture("Artist", "Album", "Track Artist")
	source := batchFile(t, root, "a.mp3", data)
	second := batchFile(t, root, "b.mp3", data)
	var path string
	func() {
		w, err := openAlbumWorkspace(root)
		if err != nil {
			t.Fatal(err)
		}
		defer w.close()
		plan := planAlbumRoot(w.root)
		journal, err := w.createJournal()
		if err != nil {
			t.Fatal(err)
		}
		defer journal.file.Close()
		path = journal.path
		first, err := w.reviewSong(plan.moves[0])
		if err != nil {
			t.Fatal(err)
		}
		defer first.file.Close()
		if moved, err := w.moveSong(t.Context(), first, journal); !moved || err != nil {
			t.Fatalf("move = %v, %v", moved, err)
		}
		song, err := w.reviewSong(plan.moves[1])
		if err != nil {
			t.Fatal(err)
		}
		defer song.file.Close()
		digest, err := hashFileSection(song.file, 0, song.info.Size())
		if err != nil {
			t.Fatal(err)
		}
		record := albumJournalMove{Source: song.move.source, Destination: song.move.destination, File: albumIdentity(song.info), SHA256: digest}
		if _, err := journal.record(albumJournalEntry{Move: &record}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(journal.file, `{"move":{"source":`); err != nil {
			t.Fatal(err)
		}
	}()
	var out, errOut bytes.Buffer
	if code := run([]string{"organize", "--undo", path}, &out, &errOut); code != 0 ||
		!strings.Contains(out.String(), "1 restored, 1 already resolved, 0 blocked, 0 failed") || errOut.Len() != 0 {
		t.Fatalf("interrupted undo exit = %d, stdout = %q, stderr = %q", code, &out, &errOut)
	}
	assertFileBytes(t, source, data)
	assertFileBytes(t, second, data)
	if _, err := os.Stat(filepath.Join(root, "Artist")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("created artist folder after undo: %v", err)
	}
}

func TestOrganizeUndoRejectsChangedSong(t *testing.T) {
	root := t.TempDir()
	source := batchFile(t, root, "song.mp3", albumFixture("Artist", "Album", "Artist"))
	path := applyTestAlbumFolder(t, root)
	destination := filepath.Join(root, "Artist/Album/song.mp3")
	changed := []byte("new contents")
	if err := os.Chmod(destination, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"organize", "--undo", path}, &out, &errOut); code != 1 ||
		!strings.Contains(errOut.String(), "MP3 contents differ from the journal") {
		t.Errorf("undo exit = %d, stdout = %q, stderr = %q", code, &out, &errOut)
	}
	assertFileBytes(t, destination, changed)
	if _, err := os.Lstat(source); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("original path after blocked undo: %v", err)
	}
}

func TestOrganizeUndoValidatesJournalBeforeMoving(t *testing.T) {
	root := t.TempDir()
	data := albumFixture("Artist", "Album", "Artist")
	source := batchFile(t, root, "song.mp3", data)
	path := applyTestAlbumFolder(t, root)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.WriteString(file, `{"folder":{"path":"../other","file":{"device":1,"inode":1}}}`+"\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"organize", "--undo", path}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "invalid organization journal entry") {
		t.Errorf("undo exit = %d, stdout = %q, stderr = %q", code, &out, &errOut)
	}
	assertFileBytes(t, filepath.Join(root, "Artist/Album/song.mp3"), data)
	if _, err := os.Lstat(source); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("original path after invalid journal: %v", err)
	}
}

func applyTestAlbumFolder(t *testing.T, root string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	choose := func(context.Context, string, []menuOption, int) (int, error) { return 1, nil }
	if code := runOrganizeApply(root, &out, &errOut, choose); code != 0 {
		t.Fatalf("apply exit = %d, stdout = %q, stderr = %q", code, &out, &errOut)
	}
	for line := range strings.SplitSeq(out.String(), "\n") {
		if path, ok := strings.CutPrefix(line, "Journal: "); ok {
			return path
		}
	}
	t.Fatalf("apply did not print its journal path: %q", &out)
	return ""
}
