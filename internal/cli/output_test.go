package cli

import (
	"bytes"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/bogem/id3v2/v2"
)

func TestDisplayOrderingAndRepeatedValues(t *testing.T) {
	entries := []entry{
		{id: "TPE1", frame: id3v2.TextFrame{Text: "First artist"}},
		{id: "TXXX", frame: id3v2.UserDefinedTextFrame{Description: "Mood", Value: "Calm"}},
		{id: "TIT2", frame: id3v2.TextFrame{Text: "A song"}},
		{id: "TPE1", frame: id3v2.TextFrame{Text: "Second artist"}},
		{id: "TALB", frame: id3v2.TextFrame{Text: "An album"}},
		{id: "TXXX", frame: id3v2.UserDefinedTextFrame{Description: "Mood", Value: "Bright"}},
	}
	want := "File: music/song.mp3\n\n" +
		"Album [TALB]: An album\n" +
		"Title [TIT2]: A song\n" +
		"Artist [TPE1]: First artist\n" +
		"Artist [TPE1]: Second artist\n" +
		"TXXX [Mood]: Calm\n" +
		"TXXX [Mood]: Bright\n"

	var out bytes.Buffer
	if err := display(&out, "music/song.mp3", entries); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != want {
		t.Errorf("display output mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestDisplayTextAndTerminalEscaping(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		entry entry
		want  string
	}{
		{
			name:  "Unicode and long text remain complete",
			path:  "music/Canção 日本語.mp3",
			entry: entry{id: "TIT2", frame: id3v2.TextFrame{Text: strings.Repeat("Canção 日本語 🎵 ", 100)}},
			want:  "File: music/Canção 日本語.mp3\n\nTitle [TIT2]: " + strings.Repeat("Canção 日本語 🎵 ", 100) + "\n",
		},
		{
			name:  "control characters in path label and value",
			path:  "music/\x1b[2J\n.mp3",
			entry: entry{id: "TXXX", frame: id3v2.UserDefinedTextFrame{Description: "Mood\r\x1b[31m", Value: "A\nB\tC\x00\x7f\u009b\u202e"}},
			want:  "File: music/\\x1b[2J\\n.mp3\n\nTXXX [Mood\\r\\x1b[31m]: A\\nB\\tC\\x00\\x7f\\u009b\\u202e\n",
		},
		{
			name:  "ordinary punctuation stays readable",
			path:  "music/song.mp3",
			entry: entry{id: "TIT2", frame: id3v2.TextFrame{Text: `It's "live": C:\music`}},
			want:  "File: music/song.mp3\n\nTitle [TIT2]: It's \"live\": C:\\music\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := display(&out, tt.path, []entry{tt.entry}); err != nil {
				t.Fatal(err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("display output mismatch\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestDisplayMetadataSummaries(t *testing.T) {
	tests := []struct {
		name  string
		entry entry
		line  string
	}{
		{
			name: "artwork",
			entry: entry{id: "APIC", frame: id3v2.PictureFrame{
				MimeType: "image/jpeg", PictureType: 3, Description: "Front cover", Picture: []byte("secret image data"),
			}},
			line: "Artwork [APIC]: image/jpeg, 17 bytes; picture type 3; description: Front cover",
		},
		{
			name:  "unknown binary data",
			entry: entry{id: "XABC", frame: id3v2.UnknownFrame{Body: []byte{0, 27, 255}}},
			line:  "XABC: unparsed data, 3 bytes",
		},
		{
			name:  "binary identifier",
			entry: entry{id: "UFID", frame: id3v2.UFIDFrame{OwnerIdentifier: "example.org", Identifier: []byte{0, 1, 2, 3}}},
			line:  "UFID: owner: example.org; binary identifier, 4 bytes",
		},
		{
			name:  "comment with language and description",
			entry: entry{id: "COMM", frame: id3v2.CommentFrame{Language: "eng", Description: "Notes", Text: "Recorded live"}},
			line:  "COMM [eng; Notes]: Recorded live",
		},
		{
			name:  "lyrics with language and description",
			entry: entry{id: "USLT", frame: id3v2.UnsynchronisedLyricsFrame{Language: "por", ContentDescriptor: "Original", Lyrics: "Olá\nmundo"}},
			line:  `USLT [por; Original]: Olá\nmundo`,
		},
		{
			name:  "rating and play count",
			entry: entry{id: "POPM", frame: id3v2.PopularimeterFrame{Email: "listener@example.org", Rating: 128, Counter: big.NewInt(42)}},
			line:  "POPM: email: listener@example.org; rating: 128; play count: 42",
		},
		{
			name:  "ordinary URL",
			entry: entry{id: "WOAR", frame: id3v2.UnknownFrame{Body: []byte("https://example.org/artist")}},
			line:  "WOAR: https://example.org/artist",
		},
		{
			name:  "custom URL remains an unparsed summary",
			entry: entry{id: "WXXX", frame: id3v2.UnknownFrame{Body: []byte{0, 'x', 0, 'y'}}},
			line:  "WXXX: unparsed data, 4 bytes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := display(&out, "song.mp3", []entry{tt.entry}); err != nil {
				t.Fatal(err)
			}
			want := "File: song.mp3\n\n" + tt.line + "\n"
			if got := out.String(); got != want {
				t.Errorf("display output mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}
}

func TestDisplayAbsentMetadata(t *testing.T) {
	for _, entries := range [][]entry{nil, {}} {
		var out bytes.Buffer
		if err := display(&out, "song.mp3", entries); err != nil {
			t.Fatal(err)
		}
		if got, want := out.String(), "File: song.mp3\n\nNo metadata found\n"; got != want {
			t.Errorf("got %q; want %q", got, want)
		}
	}
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestDisplayWriteFailure(t *testing.T) {
	writeErr := errors.New("output closed")
	if err := display(failingWriter{err: writeErr}, "song.mp3", nil); !errors.Is(err, writeErr) {
		t.Errorf("got %v; want output error %v", err, writeErr)
	}
}

func TestRunOutputFailure(t *testing.T) {
	path := save(t, fixture(3))
	writeErr := errors.New("output closed\x1b[2J")
	var errOut bytes.Buffer
	code := run([]string{"inspect", path}, failingWriter{err: writeErr}, &errOut)
	if code != 1 {
		t.Errorf("exit code = %d; want 1", code)
	}
	if got, want := errOut.String(), "Error: output closed\\x1b[2J\n"; got != want {
		t.Errorf("stderr = %q; want %q", got, want)
	}
}
