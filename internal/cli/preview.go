package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
)

const previewSyntax = "l1ner preview [--from-audio] [--release] <file.mp3>"

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
	reviewChooseRelease
)

type songReview struct {
	decision reviewDecision
	proposal songProposal
}

type reviewSearches struct {
	recording recordingSearch
	audio     audioCandidateSearch
	releases  releaseSearch
}

func runSongReview(command string, args []string, in io.Reader, out, errOut io.Writer, searches reviewSearches, batchAudio audioCandidateSearch) int {
	syntax := previewSyntax
	help := "Search MusicBrainz with the MP3's embedded title and artist and review a proposed change.\nUse --from-audio to search AcoustID. See 'l1ner identify --help' for setup."
	canceled := "Preview canceled."
	if command == "fix" {
		syntax = fixSyntax
		help = "Review and apply approved title and artist changes for one MP3 or the MP3s directly inside a directory.\nUse --from-audio to look up recordings with AcoustID. See 'l1ner identify --help' for setup."
		canceled = "Fix canceled."
	}
	options, err := parseSongPath(command, args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(out, "Usage: "+syntax)
		fmt.Fprintln(out, help)
		fmt.Fprintln(out, "Use --release to choose an album edition and review its album and track tags.")
		return 0
	}
	if err != nil {
		reportError(errOut, err)
		fmt.Fprintln(errOut, "Usage: "+syntax)
		return 2
	}
	path := options.path
	isDirectory := false
	if command == "fix" || options.fromAudio {
		info, err := os.Stat(path)
		isDirectory = err == nil && info.IsDir()
	}
	if command == "preview" && options.fromAudio && isDirectory {
		reportError(errOut, errors.New("--from-audio requires one MP3 file"))
		return 2
	}
	choose, ok := terminalMenu(in, out)
	if !ok {
		reportError(errOut, fmt.Errorf("%s requires a terminal for input and output", command))
		return 1
	}
	if !options.release {
		searches.releases = nil
	}
	if !options.fromAudio {
		searches.audio = nil
	}
	if isDirectory {
		if options.fromAudio {
			searches.audio = batchAudio
		}
		return runFixDirectory(path, out, errOut, searches, choose)
	}
	err = reviewSingleSong(command, path, out, searches, choose)
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

func reviewSingleSong(command, path string, out io.Writer, searches reviewSearches, choose menuSelector) error {
	ctx := context.Background()
	if searches.audio != nil || searches.releases != nil {
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer stop()
	}
	if searches.audio == nil && searches.releases != nil {
		searches.recording = paceRecordingSearch(searches.recording)
	}
	var err error
	if command == "fix" {
		decision, fixErr := fixSong(ctx, path, out, searches, choose)
		err = finishSingleFix(out, decision, fixErr)
	} else {
		err = previewFile(ctx, path, out, searches, choose)
	}
	if errors.Is(err, context.Canceled) {
		return errPreviewCanceled
	}
	return err
}

func previewFile(ctx context.Context, path string, out io.Writer, searches reviewSearches, choose menuSelector) error {
	if searches.audio != nil {
		return previewFromAudio(ctx, path, out, searches, choose)
	}
	entries, err := inspect(path)
	if err != nil {
		return err
	}
	tags, err := tagsForReview(entries, false, searches.releases != nil)
	if err != nil {
		return err
	}
	review, err := searchAndReview(ctx, out, path, tags, searches, choose, "Accept proposal (preview only)")
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

func searchAndReview(ctx context.Context, out io.Writer, path string, tags songTags, searches reviewSearches, choose menuSelector, applyLabel string) (songReview, error) {
	title, artist := tags.searchTerms()
	matches, err := searches.recording(ctx, title, artist)
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
	return reviewSong(ctx, out, path, tags, matches, searches.releases, choose, applyLabel)
}

func reviewSong(ctx context.Context, out io.Writer, path string, tags songTags, matches []recording, releases releaseSearch, choose menuSelector, applyLabel string) (songReview, error) {
	if _, err := fmt.Fprintf(out, "File: %s\n", safe(path)); err != nil {
		return songReview{}, reviewUIError{err}
	}
	options := recordingOptions(matches)
	options = append(options, menuOption{label: "Skip song"}, menuOption{label: "Quit"})
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
		var review songReview
		if releases == nil {
			review, err = reviewProposal(ctx, out, proposal, choose, applyLabel)
		} else {
			review, err = reviewRelease(ctx, out, proposal, tags.release, releases, choose, applyLabel)
		}
		if err != nil {
			return songReview{}, err
		}
		if review.decision != reviewChooseAgain {
			return review, nil
		}
	}
}

func reviewProposal(ctx context.Context, out io.Writer, proposal songProposal, choose menuSelector, applyLabel string) (songReview, error) {
	if err := displayProposal(out, proposal); err != nil {
		return songReview{}, reviewUIError{err}
	}
	if !proposal.changed() {
		return songReview{decision: reviewUnchanged, proposal: proposal}, nil
	}
	actions := []menuOption{
		reviewSkip:        {label: "Skip song"},
		reviewAccept:      {label: applyLabel},
		reviewChooseAgain: {label: "Choose another recording"},
		reviewQuit:        {label: "Quit"},
	}
	releaseAction := len(actions)
	if proposal.releaseID != "" {
		actions = append(actions, menuOption{label: "Choose another release"})
	}
	action, err := choose(ctx, "Review this proposal", actions, int(reviewSkip))
	decision := reviewDecision(action)
	if action == releaseAction && proposal.releaseID != "" {
		decision = reviewChooseRelease
	}
	return songReview{decision: decision, proposal: proposal}, err
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
	releaseID         string
	releaseFields     []proposalField
}

type proposalField struct {
	id, label string
	tag       tagComparison
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
	return slices.ContainsFunc(proposal.fields(), func(field proposalField) bool { return field.tag.changed() })
}

func (proposal songProposal) fields() []proposalField {
	return append([]proposalField{
		{id: "TIT2", label: "Title", tag: proposal.title},
		{id: "TPE1", label: "Artist", tag: proposal.artist},
	}, proposal.releaseFields...)
}

func displayProposal(out io.Writer, proposal songProposal) error {
	var text strings.Builder
	fmt.Fprintf(&text, "\nFile: %s\nMusicBrainz recording: %s\n", safe(proposal.path), safe(proposal.recordingID))
	if proposal.releaseID != "" {
		fmt.Fprintf(&text, "MusicBrainz release: %s\n", safe(proposal.releaseID))
	}
	text.WriteByte('\n')
	columns := tabwriter.NewWriter(&text, 0, 0, 2, ' ', 0)
	fmt.Fprintln(columns, "Field\tCurrent\tProposed\tStatus")
	for _, field := range proposal.fields() {
		writeTagComparison(columns, field.label, field.tag)
	}
	if err := columns.Flush(); err != nil {
		return err
	}
	if !proposal.changed() {
		if proposal.releaseID == "" {
			text.WriteString("\nThe title and artist already agree.\n")
		} else {
			text.WriteString("\nThe proposed tags already agree.\n")
		}
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
