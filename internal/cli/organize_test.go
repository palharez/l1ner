package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func albumFixture(artist, album, trackArtist string) []byte {
	return fixture(4,
		frame(4, "TPE2", []byte("\x03"+artist)),
		frame(4, "TALB", []byte("\x03"+album)),
		frame(4, "TPE1", []byte("\x03"+trackArtist)),
	)
}

func TestPlanAlbumFolders(t *testing.T) {
	root := t.TempDir()
	for _, song := range []struct{ name, artist, album, trackArtist string }{
		{"z-last.MP3", "Various Artists", "Collection", "Other"},
		{"b.mp3", "Second Artist", "Greatest Hits", "Second Artist"},
		{"a.mp3", " First Artist ", "Greatest Hits", "First Artist"},
		{"c.mp3", "Various Artists", "Collection", "Guest"},
	} {
		batchFile(t, root, song.name, albumFixture(song.artist, song.album, song.trackArtist))
	}
	batchFile(t, root, "Various Artists/Collection/other.mp3", albumFixture("Various Artists", "Collection", "Guest"))
	batchFile(t, root, "notes.txt", []byte("notes"))
	if err := os.Symlink("a.mp3", filepath.Join(root, "alias.mp3")); err != nil {
		t.Fatal(err)
	}
	plan, err := planAlbumFolders(root)
	if err != nil || len(plan.scanErrors) != 0 {
		t.Fatalf("plan failed: %v, scan errors %v", err, plan.scanErrors)
	}
	want := []albumMove{
		{source: "a.mp3", destination: "First Artist/Greatest Hits/a.mp3"},
		{source: "b.mp3", destination: "Second Artist/Greatest Hits/b.mp3"},
		{source: "c.mp3", destination: "Various Artists/Collection/c.mp3"},
		{source: "z-last.MP3", destination: "Various Artists/Collection/z-last.MP3"},
	}
	if !slices.Equal(plan.moves, want) {
		t.Errorf("moves = %+v, want %+v", plan.moves, want)
	}
}

func TestPlanAlbumFolderBlockers(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		name, want string
		data       []byte
	}{
		{"missing.mp3", "missing album artist tag (TPE2)", fixture(4, frame(4, "TPE1", []byte("\x03Track artist")))},
		{"conflict.mp3", "conflicting album tags (TALB)", fixture(4, frame(4, "TPE2", []byte("\x03Artist")), frame(4, "TALB", []byte("\x03One")), frame(4, "TALB", []byte("\x03Two")))},
		{"parent.mp3", "unsafe album folder", albumFixture("Artist", "..", "Artist")},
		{"separator.mp3", "unsafe album artist folder", albumFixture("Artist/Other", "Album", "Artist")},
		{"control.mp3", "unsafe album folder", albumFixture("Artist", "Before\x1bAfter", "Artist")},
		{"occupied.mp3", "destination already exists", albumFixture("Artist", "Album", "Artist")},
		{"case.mp3", "case-insensitive collision", albumFixture("Artist", "Album", "Artist")},
		{"link.mp3", "symbolic link", albumFixture("Link Artist", "Album", "Artist")},
		{"file-parent.mp3", "parent is not a directory", albumFixture("File Artist", "Album", "Artist")},
		{"case-artist-a.mp3", "planned paths collide", albumFixture("Folded Artist", "One", "Artist")},
		{"case-artist-b.mp3", "planned paths collide", albumFixture("folded artist", "Two", "Artist")},
		{"Track.MP3", "planned paths collide", albumFixture("Artist", "Album", "Artist")},
		{"track.mp3", "planned paths collide", albumFixture("Artist", "Album", "Artist")},
	}
	want := make(map[string]string)
	for _, tc := range cases {
		batchFile(t, root, tc.name, tc.data)
		want[tc.name] = tc.want
	}
	batchFile(t, root, "Artist/Album/occupied.mp3", []byte("previous file"))
	batchFile(t, root, "Artist/Album/CASE.MP3", []byte("previous file"))
	batchFile(t, root, "File Artist", []byte("ordinary file"))
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "Link Artist")); err != nil {
		t.Fatal(err)
	}
	plan, err := planAlbumFolders(root)
	if err != nil || len(plan.scanErrors) != 0 || len(plan.moves) != len(cases) {
		t.Fatalf("plan = %+v, error = %v", plan, err)
	}
	for _, move := range plan.moves {
		if move.err != nil || !strings.Contains(move.blocked, want[move.source]) {
			t.Errorf("%s blocked = %q, error = %v, want %q", move.source, move.blocked, move.err, want[move.source])
		}
	}
}
