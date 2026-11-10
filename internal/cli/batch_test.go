package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func batchFile(t *testing.T, root, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o444); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDirectoryInspection(t *testing.T) {
	root := t.TempDir()
	first := batchFile(t, root, "a.mp3", fixture(3, frame(3, "TIT2", []byte{0, 'A'})))
	bad := batchFile(t, root, "b.mp3", fixture(2))
	last := batchFile(t, root, "c.MP3", []byte{0xff, 0xfb, 0x90, 0})
	ignored := batchFile(t, root, "notes.txt", []byte("not audio"))
	nested := batchFile(t, root, "album.mp3/nested.mp3", fixture(4))
	paths := []string{first, bad, last, ignored, nested}
	before := make(map[string]fileSnapshot)
	for _, path := range paths {
		before[path] = inspectionSnapshot(t, path)
	}

	var out, errOut bytes.Buffer
	if code := run([]string{"inspect", root}, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d; want 1", code)
	}
	want := "File: " + first + "\n\nTitle [TIT2]: A\n\n" +
		"File: " + last + "\n\nNo metadata found\n" +
		"Summary: 2 inspected, 1 failed, 0 scan errors\n"
	if out.String() != want {
		t.Errorf("stdout = %q; want %q", out.String(), want)
	}
	wantError := "Error: " + bad + ": unsupported tag version: ID3v2.2\n"
	if errOut.String() != wantError {
		t.Errorf("stderr = %q; want %q", errOut.String(), wantError)
	}
	for _, path := range paths {
		if after := inspectionSnapshot(t, path); after != before[path] {
			t.Errorf("input changed: %s", path)
		}
	}
}

func TestRecursiveDirectoryInspection(t *testing.T) {
	root := t.TempDir()
	first := batchFile(t, root, "album/one.mp3", fixture(4))
	second := batchFile(t, root, "album/sub/two.MP3", fixture(3))
	last := batchFile(t, root, "z.mp3", fixture(3))
	for _, path := range []string{first, second, last} {
		before := inspectionSnapshot(t, path)
		t.Cleanup(func() {
			if after := inspectionSnapshot(t, path); after != before {
				t.Errorf("input changed: %s", path)
			}
		})
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"inspect", "--recursive", root}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d; stderr = %q", code, errOut.String())
	}
	want := "File: " + first + "\n\nNo metadata found\n\n" +
		"File: " + second + "\n\nNo metadata found\n\n" +
		"File: " + last + "\n\nNo metadata found\n" +
		"Summary: 3 inspected, 0 failed, 0 scan errors\n"
	if out.String() != want || errOut.Len() != 0 {
		t.Errorf("stdout = %q; want %q; stderr = %q", out.String(), want, errOut.String())
	}
}

func TestDirectorySymlinks(t *testing.T) {
	root := t.TempDir()
	file := batchFile(t, root, "real.mp3", fixture(3))
	for _, link := range []struct{ target, name string }{
		{file, "linked.mp3"}, {root, "loop"}, {filepath.Join(root, "missing"), "broken.mp3"},
	} {
		if err := os.Symlink(link.target, filepath.Join(root, link.name)); err != nil {
			t.Fatal(err)
		}
	}
	scan := scanDirectory(root, true)
	if !reflect.DeepEqual(scan.paths, []string{file}) || len(scan.errors) != 0 {
		t.Fatalf("scan = %+v; want only %s", scan, file)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"inspect", "--recursive", root}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d; stderr = %s", code, &errOut)
	}
	if !strings.HasSuffix(out.String(), "Summary: 1 inspected, 0 failed, 0 scan errors\n") {
		t.Errorf("unexpected summary: %s", &out)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{alias, alias + string(os.PathSeparator)} {
		out.Reset()
		errOut.Reset()
		if code := run([]string{"inspect", path}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "directory path is a symbolic link") {
			t.Errorf("symlink root %q: exit code = %d; stderr = %s", path, code, &errOut)
		}
	}
}

func TestEmptyDirectoryInspection(t *testing.T) {
	root := t.TempDir()
	batchFile(t, root, "notes.txt", []byte("ignored"))
	var out, errOut bytes.Buffer
	if code := run([]string{"inspect", root}, &out, &errOut); code != 0 {
		t.Fatal(code)
	}
	want := "No MP3 files found.\nSummary: 0 inspected, 0 failed, 0 scan errors\n"
	if out.String() != want || errOut.Len() != 0 {
		t.Errorf("stdout = %q; want %q; stderr = %q", out.String(), want, errOut.String())
	}
}

func TestUnreadableDirectoryContinues(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read directories without permission")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o700) })
	good := batchFile(t, root, "z.mp3", fixture(3))
	var out, errOut bytes.Buffer
	code := run([]string{"inspect", "--recursive", root}, &out, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), locked) {
		t.Errorf("exit code = %d; stderr = %s", code, &errOut)
	}
	if !strings.Contains(out.String(), "File: "+good) || !strings.HasSuffix(out.String(), "Summary: 1 inspected, 0 failed, 1 scan errors\n") {
		t.Errorf("stdout = %s", &out)
	}
}

func TestBatchOutputFailure(t *testing.T) {
	root := t.TempDir()
	batchFile(t, root, "one.mp3", fixture(3))
	writeErr := errors.New("output closed")
	var errOut bytes.Buffer
	if code := run([]string{"inspect", root}, failingWriter{err: writeErr}, &errOut); code != 1 {
		t.Errorf("exit code = %d; want 1", code)
	}
	if got := errOut.String(); got != "Error: output closed\n" {
		t.Errorf("stderr = %q", got)
	}
}

func TestDirectoryArguments(t *testing.T) {
	file := save(t, fixture(3))
	for _, args := range [][]string{
		{"inspect", "--recursive", file},
		{"inspect", "--unknown", "music"},
		{"inspect", "--recursive"},
	} {
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code != 2 || out.Len() != 0 {
			t.Errorf("%v: exit code = %d; stdout = %q; stderr = %q", args, code, out.String(), errOut.String())
		}
	}
}
