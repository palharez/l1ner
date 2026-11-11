package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMoveReviewedAlbumSong(t *testing.T) {
	for _, scenario := range []string{"move", "occupied", "changed source", "symlink parent"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			data := albumFixture("Artist", "Album", "Track Artist")
			source := batchFile(t, root, "song.mp3", data)
			backup := batchFile(t, root, "song.mp3.l1ner.bak", []byte("tag backup"))
			w, err := openAlbumWorkspace(root)
			if err != nil {
				t.Fatal(err)
			}
			defer w.close()
			plan := planAlbumRoot(w.root)
			if len(plan.moves) != 1 {
				t.Fatalf("plan = %+v", plan)
			}
			song, err := w.reviewSong(plan.moves[0])
			if err != nil {
				t.Fatal(err)
			}
			defer song.file.Close()
			journal, err := w.createJournal()
			if err != nil {
				t.Fatal(err)
			}
			defer journal.file.Close()

			switch scenario {
			case "occupied":
				batchFile(t, root, "Artist/Album/song.mp3", []byte("existing destination"))
			case "changed source":
				if err := os.Rename(source, source+".old"); err != nil {
					t.Fatal(err)
				}
				data = []byte("replacement")
				batchFile(t, root, "song.mp3", data)
			case "symlink parent":
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "Artist")); err != nil {
					t.Fatal(err)
				}
			}

			moved, err := w.moveSong(t.Context(), song, journal)
			assertFileBytes(t, backup, []byte("tag backup"))
			if scenario == "move" {
				if !moved || err != nil {
					t.Fatalf("move = %v, %v", moved, err)
				}
				assertFileBytes(t, filepath.Join(root, "Artist/Album/song.mp3"), data)
				if _, err := os.Lstat(source); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("source path after move: %v", err)
				}
				log, err := os.ReadFile(journal.path)
				if err != nil || !bytes.Contains(log, []byte(`"destination":"Artist/Album/song.mp3"`)) {
					t.Errorf("journal = %q, error = %v", log, err)
				}
				return
			}
			var blocked albumMoveBlocked
			if moved || !errors.As(err, &blocked) {
				t.Fatalf("move = %v, %v, want blocked", moved, err)
			}
			assertFileBytes(t, source, data)
			if scenario == "occupied" && !strings.Contains(err.Error(), "already exists") {
				t.Errorf("unexpected occupied-destination error: %v", err)
			}
		})
	}
}
