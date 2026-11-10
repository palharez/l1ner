package cli

import (
	"context"
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

const previewSyntax = "l1ner preview [--from-audio] <file.mp3>"

type menuSelector func(ctx context.Context, question string, options []menuOption, selected int) (int, error)

// Losing the terminal ends a batch because the next proposal cannot be reviewed.
type reviewUIError struct{ error }

func (err reviewUIError) Unwrap() error { return err.error }

type reviewDecision int

const (
	reviewSkip reviewDecision = iota
	reviewAccept
	reviewChooseAgain
	reviewQuit
	reviewUnchanged
	reviewUnresolved
)

type songReview struct {
	decision reviewDecision
	proposal songProposal
}

func runSongReview(command string, args []string, in io.Reader, out, errOut io.Writer, search recordingSearch, audio audioCandidateSearch) int {
	syntax := previewSyntax
	help := "Search MusicBrainz with the MP3's embedded title and artist and review a proposed change.\nUse --from-audio to search AcoustID instead. See 'l1ner identify --help' for setup."
	canceled := "Preview canceled."
	reviewFile := previewFile
	if command == "fix" {
		syntax = fixSyntax
		help = "Review and apply approved title and artist changes for one MP3 or the MP3s directly inside a directory.\nUse --from-audio for one MP3. See 'l1ner identify --help' for AcoustID setup."
		canceled = "Fix canceled."
		reviewFile = fixFile
	}
	path, fromAudio, err := parseSongPath(command, args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(out, "Usage: "+syntax)
		fmt.Fprintln(out, help)
		return 0
	}
	if err != nil {
		reportError(errOut, err)
		fmt.Fprintln(errOut, "Usage: "+syntax)
		return 2
	}
	isDirectory := false
	if command == "fix" || fromAudio {
		info, err := os.Stat(path)
		isDirectory = err == nil && info.IsDir()
	}
	if fromAudio && isDirectory {
		reportError(errOut, errors.New("--from-audio requires one MP3 file"))
		return 2
	}
	input, inputIsFile := in.(*os.File)
	output, outputIsFile := out.(*os.File)
	if !inputIsFile || !outputIsFile || !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
		reportError(errOut, fmt.Errorf("%s requires a terminal for input and output", command))
		return 1
	}
	choose := func(ctx context.Context, question string, options []menuOption, selected int) (int, error) {
		choice, err := selectTerminalMenu(ctx, input, output, question, options, selected)
		if err != nil && !errors.Is(err, errPreviewCanceled) {
			return 0, reviewUIError{err}
		}
		return choice, err
	}
	if isDirectory {
		return runFixDirectory(path, out, errOut, search, choose)
	}
	if fromAudio {
		err = reviewFileFromAudio(command, path, out, audio, choose)
	} else {
		err = reviewFile(path, out, search, choose)
	}
	if err != nil {
		if errors.Is(err, errPreviewCanceled) {
			if err == errPreviewCanceled {
				fmt.Fprintln(errOut, canceled)
			} else {
				reportError(errOut, err)
			}
			return 130
		}
		reportError(errOut, err)
		return 1
	}
	return 0
}

func previewFile(path string, out io.Writer, search recordingSearch, choose menuSelector) error {
	tags, err := readLookupTags(path)
	if err != nil {
		return err
	}
	review, err := searchAndReview(context.Background(), out, path, tags, search, choose, "Accept proposal (preview only)")
	return finishPreview(out, review, err)
}

func finishPreview(out io.Writer, review songReview, err error) error {
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

func searchAndReview(ctx context.Context, out io.Writer, path string, tags songTags, search recordingSearch, choose menuSelector, applyLabel string) (songReview, error) {
	title, artist := tags.searchTerms()
	matches, err := search(ctx, title, artist)
	if err != nil {
		return songReview{}, err
	}
	if err := ctx.Err(); err != nil {
		return songReview{}, err
	}
	if len(matches) == 0 {
		if _, err := fmt.Fprintf(out, "File: %s\nNo MusicBrainz recording candidates found.\n", safe(path)); err != nil {
			return songReview{}, reviewUIError{err}
		}
		return songReview{decision: reviewUnresolved}, nil
	}
	return reviewSong(ctx, out, path, tags, matches, choose, applyLabel)
}

func reviewSong(ctx context.Context, out io.Writer, path string, tags songTags, matches []recording, choose menuSelector, applyLabel string) (songReview, error) {
	if _, err := fmt.Fprintf(out, "File: %s\n", safe(path)); err != nil {
		return songReview{}, reviewUIError{err}
	}
	options := recordingOptions(matches)
	options = append(options, menuOption{label: "Skip song"}, menuOption{label: "Quit"})
	actions := []menuOption{
		reviewSkip:        {label: "Skip song"},
		reviewAccept:      {label: applyLabel},
		reviewChooseAgain: {label: "Choose another recording"},
		reviewQuit:        {label: "Quit"},
	}
	selected := 0
	for {
		var err error
		selected, err = choose(ctx, "Choose a recording", options, selected)
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
			return songReview{}, reviewUIError{err}
		}
		if !proposal.changed() {
			return songReview{decision: reviewUnchanged, proposal: proposal}, nil
		}
		action, err := choose(ctx, "Review this proposal", actions, int(reviewSkip))
		if err != nil {
			return songReview{}, err
		}
		if reviewDecision(action) != reviewChooseAgain {
			return songReview{decision: reviewDecision(action), proposal: proposal}, nil
		}
	}
}

func recordingOptions(matches []recording) []menuOption {
	options := make([]menuOption, 0, len(matches)+2)
	for i, match := range matches {
		label := fmt.Sprintf("%d. %s — %s", i+1, safe(match.title), safe(match.artist))
		details := []string{"Recording ID: " + safe(match.id)}
		if match.acoustIDScore != nil {
			details = append(details, fmt.Sprintf("AcoustID score: %.4f", *match.acoustIDScore))
		}
		if match.disambiguation != "" {
			details = append(details, "Version: "+safe(match.disambiguation))
		}
		if match.lengthMS > 0 {
			seconds := match.lengthMS / 1000
			label += fmt.Sprintf(" · %d:%02d", seconds/60, seconds%60)
		}
		options = append(options, menuOption{label: label, details: details})
	}
	return options
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
	if len(tag.current) == 0 {
		return tag.proposed != ""
	}
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
	current := strings.Join(values, ", ")
	if len(values) == 0 {
		current = "(missing)"
		status = "added"
	}
	fmt.Fprintf(out, "%s\t%s\t%q\t%s\n", label, current, tag.proposed, status)
}
