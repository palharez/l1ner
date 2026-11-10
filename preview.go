package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"golang.org/x/term"
)

const previewSyntax = "l1ner preview <file.mp3>"

type menuSelector func(question string, options []string, selected int) (int, error)

type reviewDecision int

const (
	reviewSkip reviewDecision = iota
	reviewAccept
	reviewChooseAgain
	reviewQuit
	reviewUnchanged
)

type songReview struct {
	decision reviewDecision
	proposal songProposal
}

func runPreview(args []string, in io.Reader, out, errOut io.Writer, search func(title, artist string) ([]recording, error)) int {
	path, err := parseSongPath("preview", args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(out, "Usage: "+previewSyntax)
		fmt.Fprintln(out, "Search MusicBrainz with the MP3's embedded title and artist and review a proposed change.")
		return 0
	}
	if err != nil {
		reportError(errOut, err)
		fmt.Fprintln(errOut, "Usage: "+previewSyntax)
		return 2
	}
	input, inputIsFile := in.(*os.File)
	output, outputIsFile := out.(*os.File)
	if !inputIsFile || !outputIsFile || !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
		reportError(errOut, errors.New("preview requires a terminal for input and output"))
		return 1
	}
	choose := func(question string, options []string, selected int) (int, error) {
		return selectTerminalMenu(input, output, question, options, selected)
	}
	if err := previewFile(path, out, search, choose); err != nil {
		if errors.Is(err, errPreviewCanceled) {
			fmt.Fprintln(errOut, "Preview canceled.")
			return 130
		}
		reportError(errOut, err)
		return 1
	}
	return 0
}

func previewFile(path string, out io.Writer, search func(title, artist string) ([]recording, error), choose menuSelector) error {
	tags, err := readLookupTags(path)
	if err != nil {
		return err
	}
	title, artist := tags.searchTerms()
	matches, err := search(title, artist)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		_, err := fmt.Fprintf(out, "File: %s\nNo MusicBrainz recording candidates found.\n", safe(path))
		return err
	}
	review, err := reviewSong(out, path, tags, matches, choose)
	if err != nil {
		return err
	}
	switch review.decision {
	case reviewAccept:
		_, err = fmt.Fprintf(out, "Accepted for this preview: %s (recording %s).\n", safe(review.proposal.path), safe(review.proposal.recordingID))
	case reviewSkip:
		_, err = fmt.Fprintln(out, "Skipped song.")
	case reviewQuit:
		_, err = fmt.Fprintln(out, "Preview ended.")
	}
	return err
}

func reviewSong(out io.Writer, path string, tags songTags, matches []recording, choose menuSelector) (songReview, error) {
	options, err := displayPreviewCandidates(out, path, matches)
	if err != nil {
		return songReview{}, err
	}
	options = append(options, "Skip song", "Quit")
	actions := []string{
		reviewSkip:        "Skip song",
		reviewAccept:      "Accept proposal (preview only)",
		reviewChooseAgain: "Choose another recording",
		reviewQuit:        "Quit",
	}
	selected := 0
	for {
		selected, err = choose("Choose a recording", options, selected)
		if err != nil {
			return songReview{}, err
		}
		if selected == len(matches) {
			return songReview{decision: reviewSkip}, nil
		}
		if selected == len(matches)+1 {
			return songReview{decision: reviewQuit}, nil
		}
		proposal := proposeSong(path, tags, matches[selected])
		if err := displayProposal(out, proposal); err != nil {
			return songReview{}, err
		}
		if !proposal.changed() {
			return songReview{decision: reviewUnchanged, proposal: proposal}, nil
		}
		action, err := choose("Review this proposal", actions, int(reviewSkip))
		if err != nil {
			return songReview{}, err
		}
		if reviewDecision(action) != reviewChooseAgain {
			return songReview{decision: reviewDecision(action), proposal: proposal}, nil
		}
	}
}

func displayPreviewCandidates(out io.Writer, path string, matches []recording) ([]string, error) {
	var text strings.Builder
	fmt.Fprintf(&text, "File: %s\nMusicBrainz recording candidates:\n", safe(path))
	options := make([]string, 0, len(matches))
	for i, match := range matches {
		label := fmt.Sprintf("%d. %s — %s", i+1, safe(match.title), safe(match.artist))
		if match.disambiguation != "" {
			label += " · " + safe(match.disambiguation)
		}
		if match.lengthMS > 0 {
			seconds := match.lengthMS / 1000
			label += fmt.Sprintf(" · %d:%02d", seconds/60, seconds%60)
		}
		options = append(options, label)
		fmt.Fprintf(&text, "%s\n   Recording ID: %s\n", label, safe(match.id))
	}
	_, err := io.WriteString(out, text.String())
	return options, err
}

type songProposal struct {
	path, recordingID string
	title, artist     tagComparison
}

type tagComparison struct {
	current  []string
	proposed string
}

func (tag tagComparison) changed() bool {
	return slices.ContainsFunc(tag.current, func(value string) bool { return value != tag.proposed })
}

func proposeSong(path string, tags songTags, match recording) songProposal {
	return songProposal{
		path: path, recordingID: match.id,
		title:  tagComparison{current: tags.titles, proposed: match.title},
		artist: tagComparison{current: tags.artists, proposed: match.artist},
	}
}

func (proposal songProposal) changed() bool {
	return proposal.title.changed() || proposal.artist.changed()
}

func displayProposal(out io.Writer, proposal songProposal) error {
	var text strings.Builder
	fmt.Fprintf(&text, "\nFile: %s\nMusicBrainz recording: %s\n\n", safe(proposal.path), safe(proposal.recordingID))
	columns := tabwriter.NewWriter(&text, 0, 0, 2, ' ', 0)
	fmt.Fprintln(columns, "Field\tCurrent\tProposed\tStatus")
	writeTagComparison(columns, "Title", proposal.title)
	writeTagComparison(columns, "Artist", proposal.artist)
	if err := columns.Flush(); err != nil {
		return err
	}
	if !proposal.changed() {
		text.WriteString("\nThe title and artist already agree.\n")
	}
	_, err := io.WriteString(out, text.String())
	return err
}

func writeTagComparison(out io.Writer, label string, tag tagComparison) {
	values := make([]string, 0, len(tag.current))
	for _, value := range tag.current {
		values = append(values, strconv.Quote(value))
	}
	status := "same"
	if tag.changed() {
		status = "changed"
	}
	fmt.Fprintf(out, "%s\t%s\t%q\t%s\n", label, strings.Join(values, ", "), tag.proposed, status)
}
