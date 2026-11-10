package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/bogem/id3v2/v2"
)

const lookupSyntax = "l1ner lookup <file.mp3>"

func runLookup(args []string, out, errOut io.Writer, search recordingSearch) int {
	options, err := parseSongPath("lookup", args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(out, "Usage: "+lookupSyntax)
		fmt.Fprintln(out, "Send the MP3's embedded title and artist to MusicBrainz and show recording candidates.")
		return 0
	}
	if err != nil {
		reportError(errOut, err)
		fmt.Fprintln(errOut, "Usage: "+lookupSyntax)
		return 2
	}
	if err := lookupFile(options.path, out, search); err != nil {
		reportError(errOut, err)
		return 1
	}
	return 0
}

type songOptions struct {
	path               string
	fromAudio, release bool
}

func parseSongPath(command string, args []string) (songOptions, error) {
	var options songOptions
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	if command == "preview" || command == "fix" {
		flags.BoolVar(&options.fromAudio, "from-audio", false, "use AcoustID fingerprint matches")
		flags.BoolVar(&options.release, "release", false, "choose a MusicBrainz release")
	}
	if err := flags.Parse(args); err != nil {
		return options, err
	}
	if flags.NArg() != 1 {
		if command == "fix" {
			return options, errors.New("expected one MP3 file or directory path")
		}
		return options, errors.New("expected one MP3 file path")
	}
	options.path = flags.Arg(0)
	return options, nil
}

func lookupFile(path string, out io.Writer, search recordingSearch) error {
	tags, err := readLookupTags(path)
	if err != nil {
		return err
	}
	title, artist := tags.searchTerms()
	matches, err := search(context.Background(), title, artist)
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

type songTags struct {
	titles, artists []string
	release         map[string][]string
}

func (tags songTags) searchTerms() (title, artist string) {
	return strings.TrimSpace(tags.titles[0]), strings.TrimSpace(tags.artists[0])
}

func readLookupTags(path string) (songTags, error) {
	entries, err := inspect(path)
	if err != nil {
		return songTags{}, err
	}
	return lookupTagsFromEntries(entries)
}

func lookupTagsFromEntries(entries []entry) (songTags, error) {
	titles, err := lookupTag(entries, "TIT2", "title")
	if err != nil {
		return songTags{}, err
	}
	artists, err := lookupTag(entries, "TPE1", "artist")
	if err != nil {
		return songTags{}, err
	}
	return songTags{titles: titles, artists: artists}, nil
}

func lookupTag(entries []entry, id, label string) ([]string, error) {
	var normalized string
	var values []string
	for _, value := range songTagValues(entries, id) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if normalized != "" && normalized != trimmed {
			return nil, fmt.Errorf("conflicting %s tags (%s)", label, id)
		}
		normalized = trimmed
		values = append(values, value)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("missing %s tag (%s)", label, id)
	}
	return values, nil
}

func songTagValues(entries []entry, id string) []string {
	var values []string
	seen := make(map[string]bool)
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
			if !seen[part] {
				values = append(values, part)
				seen[part] = true
			}
		}
	}
	return values
}
