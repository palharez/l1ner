package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/bogem/id3v2/v2"
)

// Escape control characters while retaining readable Unicode and full values.
func safe(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			q := strconv.QuoteRuneToASCII(r)
			b.WriteString(q[1 : len(q)-1])
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func display(w io.Writer, path string, entries []entry) error {
	var b strings.Builder
	fmt.Fprintf(&b, "File: %s\n\n", safe(path))
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	if len(entries) == 0 {
		b.WriteString("No metadata found\n")
	}
	for _, e := range entries {
		label, value := describe(e)
		fmt.Fprintf(&b, "%s: %s\n", safe(label), safe(value))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func describe(e entry) (string, string) {
	label := e.id
	names := map[string]string{"TIT2": "Title", "TPE1": "Artist", "TPE2": "Album artist", "TALB": "Album", "TRCK": "Track number", "TPOS": "Disc number", "TDRC": "Date", "TYER": "Year", "TCON": "Genre", "TCOM": "Composer", "APIC": "Artwork"}
	if name, ok := names[e.id]; ok {
		label = name + " [" + e.id + "]"
	}
	switch f := e.frame.(type) {
	case id3v2.TextFrame:
		return label, f.Text
	case id3v2.UserDefinedTextFrame:
		return "TXXX [" + f.Description + "]", f.Value
	case id3v2.CommentFrame:
		return "COMM [" + f.Language + "; " + f.Description + "]", f.Text
	case id3v2.UnsynchronisedLyricsFrame:
		return "USLT [" + f.Language + "; " + f.ContentDescriptor + "]", f.Lyrics
	case id3v2.PictureFrame:
		return label, fmt.Sprintf("%s, %d bytes; picture type %d; description: %s", f.MimeType, len(f.Picture), f.PictureType, f.Description)
	case id3v2.UFIDFrame:
		return label, fmt.Sprintf("owner: %s; binary identifier, %d bytes", f.OwnerIdentifier, len(f.Identifier))
	case id3v2.PopularimeterFrame:
		return label, fmt.Sprintf("email: %s; rating: %d; play count: %v", f.Email, f.Rating, f.Counter)
	case id3v2.UnknownFrame:
		if strings.HasPrefix(e.id, "W") && e.id != "WXXX" {
			return label, string(f.Body)
		}
		return label, fmt.Sprintf("unparsed data, %d bytes", len(f.Body))
	default:
		return label, fmt.Sprintf("%T, %d bytes", f, f.Size())
	}
}
