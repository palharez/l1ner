package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func saveFixFixture(t *testing.T) (string, []byte) {
	t.Helper()
	original := fixture(3, frame(3, "TIT2", []byte("\x00title")), frame(3, "TPE1", []byte("\x00Artist")), frame(3, "XABC", []byte{3, 4, 5}))
	original = append(original, []byte("audio data and trailing bytes")...)
	path := filepath.Join(t.TempDir(), "sample.mp3")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	return path, original
}

func TestFixReplacesOnlyApprovedTagsAndBacksUp(t *testing.T) {
	path, original := saveFixFixture(t)
	before := inspectionSnapshot(t, path)
	target, err := openFixTarget(path, false, false)
	if err != nil {
		t.Fatal(err)
	}
	defer target.close()
	proposal := proposeSong(path, target.tags, recording{id: "studio", title: "A longer Title", artist: "New Artist"})
	result, err := target.apply(t.Context(), proposal)
	if err != nil || result.syncWarning != nil || result.backupPath != path+backupSuffix {
		t.Fatalf("apply = (%+v, %v)", result, err)
	}
	if snapshot := inspectionSnapshot(t, result.backupPath); snapshot != before {
		t.Errorf("backup snapshot = %+v, want %+v", snapshot, before)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	header, body, err := readTag(bytes.NewReader(updated), int64(len(updated)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(updated[len(header)+len(body):], original[len(target.header)+len(target.body):]) {
		t.Error("audio or trailing data changed")
	}
	if !bytes.Contains(body, frame(3, "XABC", []byte{3, 4, 5})) || inspectionSnapshot(t, path).mode != before.mode {
		t.Error("unrelated frame bytes or file permissions changed")
	}
	tags, err := readLookupTags(path)
	if err != nil {
		t.Fatal(err)
	}
	if title, artist := tags.searchTerms(); title != "A longer Title" || artist != "New Artist" {
		t.Errorf("edited tags = (%q, %q)", title, artist)
	}
	if err := os.Rename(result.backupPath, path); err != nil {
		t.Fatal(err)
	}
	if after := inspectionSnapshot(t, path); after != before {
		t.Errorf("restored snapshot = %+v, want %+v", after, before)
	}
	assertNoFixStages(t, path)
}

func TestFixHandlesConcurrentChanges(t *testing.T) {
	for _, change := range []string{"backup collision", "original edit"} {
		t.Run(change, func(t *testing.T) {
			path, original := saveFixFixture(t)
			target, err := openFixTarget(path, false, false)
			if err != nil {
				t.Fatal(err)
			}
			defer target.close()
			wantError := "MP3 changed since review began"
			if change == "backup collision" {
				wantError = "publish backup"
				if err := os.WriteFile(path+backupSuffix, []byte("prior backup"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				original[len(original)-1] = '!'
				if err := os.WriteFile(path, original, 0o640); err != nil {
					t.Fatal(err)
				}
			}
			proposal := proposeSong(path, target.tags, recording{title: "Title", artist: "Artist"})
			if _, err := target.apply(t.Context(), proposal); err == nil || !strings.Contains(err.Error(), wantError) {
				t.Errorf("apply error = %v, want %q", err, wantError)
			}
			if current, err := os.ReadFile(path); err != nil || !bytes.Equal(current, original) {
				t.Errorf("source changed during failed apply: %v", err)
			}
			if change == "backup collision" {
				if prior, err := os.ReadFile(path + backupSuffix); err != nil || string(prior) != "prior backup" {
					t.Errorf("existing backup was replaced: %v", err)
				}
			}
			assertNoFixStages(t, path)
		})
	}
}

func TestFixRenameFailureKeepsOriginalAndBackup(t *testing.T) {
	path, original := saveFixFixture(t)
	target, err := openFixTarget(path, false, false)
	if err != nil {
		t.Fatal(err)
	}
	defer target.close()
	backup, stage, err := target.stageFile()
	if err != nil {
		t.Fatal(err)
	}
	defer target.dir.Remove(stage)
	defer backup.Close()
	if err := target.copyBackup(backup); err != nil {
		t.Fatal(err)
	}
	if err := finishStage(backup, target.info); err != nil {
		t.Fatal(err)
	}
	if _, err := target.publish(t.Context(), stage, "missing-replacement"); err == nil || !strings.Contains(err.Error(), "replace MP3") {
		t.Fatalf("publish error = %v, want a failed replacement", err)
	}
	for _, file := range []string{path, path + backupSuffix} {
		if data, err := os.ReadFile(file); err != nil || !bytes.Equal(data, original) {
			t.Errorf("%s differs from the original: %v", file, err)
		}
	}
}

func TestFixRejectsUnsafeFileLayouts(t *testing.T) {
	for _, layout := range []string{"symlink", "hard link", "read-only", "locked"} {
		t.Run(layout, func(t *testing.T) {
			path, _ := saveFixFixture(t)
			alias := filepath.Join(filepath.Dir(path), "other.mp3")
			switch layout {
			case "symlink":
				if err := os.Symlink(path, alias); err != nil {
					t.Fatal(err)
				}
				path = alias
			case "hard link":
				if err := os.Link(path, alias); err != nil {
					t.Fatal(err)
				}
			case "read-only":
				if err := os.Chmod(path, 0o440); err != nil {
					t.Fatal(err)
				}
			case "locked":
				target, err := openFixTarget(path, false, false)
				if err != nil {
					t.Fatal(err)
				}
				defer target.close()
			}
			if target, err := openFixTarget(path, false, false); err == nil {
				target.close()
				t.Errorf("opened %s MP3 for editing", layout)
			}
			assertNoFixStages(t, path)
		})
	}
}

func assertNoFixStages(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".l1ner-stage-") {
			t.Errorf("staged file remains: %s", entry.Name())
		}
	}
}
