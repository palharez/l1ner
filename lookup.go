package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/bogem/id3v2/v2"
)

const lookupSyntax = "l1ner lookup <file.mp3>"

func runLookup(args []string, out, errOut io.Writer, search func(title, artist string) ([]recording, error)) int {
	flags := flag.NewFlagSet("lookup", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(out, "Usage: "+lookupSyntax)
		fmt.Fprintln(out, "Send the MP3's embedded title and artist to MusicBrainz and show recording candidates.")
		return 0
	}
	if err == nil && flags.NArg() != 1 {
		err = errors.New("expected one MP3 file path")
	}
	if err != nil {
		reportError(errOut, err)
		fmt.Fprintln(errOut, "Usage: "+lookupSyntax)
		return 2
	}
	if err := lookupFile(flags.Arg(0), out, search); err != nil {
		reportError(errOut, err)
		return 1
	}
	return 0
}

func lookupFile(path string, out io.Writer, search func(title, artist string) ([]recording, error)) error {
	title, artist, err := readLookupTags(path)
	if err != nil {
		return err
	}
	matches, err := search(title, artist)
	if err != nil {
		return err
	}
	return displayLookup(out, path, title, artist, matches)
}

func displayLookup(out io.Writer, path, title, artist string, matches []recording) error {
	var output strings.Builder
	fmt.Fprintf(&output, "File: %s\nSearch: %s — %s\n\n", safe(path), safe(title), safe(artist))
	if len(matches) == 0 {
		output.WriteString("No MusicBrainz recording candidates found.\n")
	} else {
		output.WriteString("MusicBrainz recording candidates:\n")
		for i, match := range matches {
			fmt.Fprintf(&output, "%d. %s — %s\n   Recording ID: %s\n", i+1, safe(match.title), safe(match.artist), safe(match.id))
		}
	}
	_, err := io.WriteString(out, output.String())
	return err
}

func readLookupTags(path string) (string, string, error) {
	entries, err := inspect(path)
	if err != nil {
		return "", "", err
	}
	title, err := lookupTag(entries, "TIT2", "title")
	if err != nil {
		return "", "", err
	}
	artist, err := lookupTag(entries, "TPE1", "artist")
	if err != nil {
		return "", "", err
	}
	return title, artist, nil
}

func lookupTag(entries []entry, id, label string) (string, error) {
	var value string
	for _, entry := range entries {
		if entry.id != id {
			continue
		}
		text, ok := entry.frame.(id3v2.TextFrame)
		if !ok {
			continue
		}
		// The decoder returns NUL-separated ID3v2.4 values as a single string.
		for part := range strings.SplitSeq(text.Text, "\x00") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if value != "" && value != part {
				return "", fmt.Errorf("conflicting %s tags (%s)", label, id)
			}
			value = part
		}
	}
	if value == "" {
		return "", fmt.Errorf("missing %s tag (%s)", label, id)
	}
	return value, nil
}
