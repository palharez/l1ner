package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestOrganizeDryRun(t *testing.T) {
	root := t.TempDir()
	batchFile(t, root, "c\x1b.mp3", albumFixture("Artist", "Album", "Artist"))
	batchFile(t, root, "a.MP3", albumFixture("Artist", "Album", "Artist"))
	batchFile(t, root, "b.mp3", fixture(2))
	batchFile(t, root, "nested/track.mp3", albumFixture("Artist", "Album", "Artist"))
	batchFile(t, root, "a.MP3.l1ner.bak", []byte("tag backup"))
	paths, files := snapshotOrganization(t, root)

	var previous string
	for range 2 {
		var out, errOut bytes.Buffer
		code := run([]string{"organize", "--dry-run", root}, &out, &errOut)
		if code != 1 || errOut.Len() != 0 {
			t.Errorf("exit = %d, stderr = %q, want 1 and empty stderr", code, &errOut)
		}
		want := "Planned: " + filepath.Join(root, "a.MP3") + " -> " + filepath.Join(root, "Artist/Album/a.MP3") + "\n" +
			"Failed: " + filepath.Join(root, "b.mp3") + " — unsupported tag version: ID3v2.2\n" +
			"Blocked: " + filepath.Join(root, `c\x1b.mp3`) + " — unsafe source filename\n" +
			"Summary: 1 planned, 1 blocked, 1 failed, 0 scan errors\n"
		if out.String() != want || (previous != "" && previous != out.String()) {
			t.Errorf("stdout = %q, want %q", &out, want)
		}
		previous = out.String()
		gotPaths, gotFiles := snapshotOrganization(t, root)
		if !slices.Equal(gotPaths, paths) || !maps.Equal(gotFiles, files) {
			t.Errorf("preview changed paths or file bytes: paths = %v, want %v", gotPaths, paths)
		}
	}
}

func TestOrganizeArgumentsAndOutcomes(t *testing.T) {
	empty := t.TempDir()
	folder := t.TempDir()
	file := batchFile(t, folder, "missing.mp3", fixture(4))
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(folder, link); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{"help", []string{"--help"}, 0, "organize --dry-run"},
		{"missing flag", []string{folder}, 2, "--dry-run is required"},
		{"missing path", []string{"--dry-run"}, 2, "expected one directory path"},
		{"unknown flag", []string{"--recursive", folder}, 2, "flag provided but not defined"},
		{"file input", []string{"--dry-run", file}, 1, "expected a directory"},
		{"symlink input", []string{"--dry-run", link + "/"}, 1, "directory path is a symbolic link"},
		{"empty folder", []string{"--dry-run", empty}, 0, "No MP3 files found."},
		{"blocker", []string{"--dry-run", folder}, 0, "Summary: 0 planned, 1 blocked, 0 failed, 0 scan errors"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := run(append([]string{"organize"}, tc.args...), &out, &errOut)
			if code != tc.code || !strings.Contains(out.String()+errOut.String(), tc.want) {
				t.Errorf("exit = %d, stdout = %q, stderr = %q, want %d and %q", code, &out, &errOut, tc.code, tc.want)
			}
		})
	}
}

func TestOrganizeOutputFailure(t *testing.T) {
	var errOut bytes.Buffer
	args := []string{"organize", "--dry-run", t.TempDir()}
	code := run(args, failingWriter{err: errors.New("output closed")}, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), "output closed") {
		t.Errorf("exit = %d, stderr = %q", code, &errOut)
	}
}

func snapshotOrganization(t *testing.T, root string) ([]string, map[string]fileSnapshot) {
	t.Helper()
	var paths []string
	files := make(map[string]fileSnapshot)
	err := fs.WalkDir(os.DirFS(root), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		if entry.Type().IsRegular() {
			files[path] = inspectionSnapshot(t, filepath.Join(root, path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths, files
}
