package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func tagsForReview(entries []entry, fromAudio, withRelease bool) (songTags, error) {
	var tags songTags
	if fromAudio {
		tags = songTags{titles: songTagValues(entries, "TIT2"), artists: songTagValues(entries, "TPE1")}
	} else {
		var err error
		tags, err = lookupTagsFromEntries(entries)
		if err != nil {
			return songTags{}, err
		}
	}
	if withRelease {
		tags.release = make(map[string][]string)
		for _, id := range []string{"TALB", "TPE2", "TRCK", "TPOS"} {
			tags.release[id] = songTagValues(entries, id)
		}
	}
	return tags, nil
}

func reviewRelease(ctx context.Context, out io.Writer, base songProposal, current map[string][]string, search releaseSearch, choose menuSelector, applyLabel string) (songReview, error) {
	matches, err := search(ctx, base.recordingID)
	if err != nil {
		return songReview{}, err
	}
	if err := ctx.Err(); err != nil {
		return songReview{}, err
	}
	if len(matches) == 0 {
		if _, err := fmt.Fprintln(out, "No MusicBrainz release tracks found for this recording."); err != nil {
			return songReview{}, reviewUIError{err}
		}
		return songReview{decision: reviewUnresolved}, nil
	}
	options := releaseOptions(matches)
	actions := []reviewDecision{reviewChooseAgain, reviewSkip, reviewQuit}
	selected := 0
	for {
		selected, err = choose(ctx, "Choose a release", options, selected)
		if err != nil {
			return songReview{}, err
		}
		if selected >= len(matches) {
			return songReview{decision: actions[selected-len(matches)]}, nil
		}
		proposal := proposeRelease(base, current, matches[selected])
		review, err := reviewProposal(ctx, out, proposal, choose, applyLabel)
		if err != nil || review.decision != reviewChooseRelease {
			return review, err
		}
	}
}

func releaseOptions(matches []releaseTrack) []menuOption {
	options := make([]menuOption, 0, len(matches)+3)
	for _, match := range matches {
		label := fmt.Sprintf("Disc %d, track %d · %s", match.disc, match.track, safe(match.album))
		var details []string
		for _, value := range []string{match.date, match.country, match.format} {
			if value != "" {
				details = append(details, safe(value))
			}
		}
		if len(details) > 0 {
			details = []string{strings.Join(details, " · ")}
		}
		details = append(details, "Release ID: "+safe(match.id))
		if match.albumArtist != "" {
			details = append(details, "Album artist: "+safe(match.albumArtist))
		}
		options = append(options, menuOption{label: label, details: details})
	}
	return append(options, menuOption{label: "Choose another recording"}, menuOption{label: "Skip song"}, menuOption{label: "Quit"})
}

func proposeRelease(base songProposal, current map[string][]string, match releaseTrack) songProposal {
	base.releaseID = match.id
	for _, field := range []struct{ id, label, value string }{
		{"TALB", "Album", match.album},
		{"TPE2", "Album artist", match.albumArtist},
		{"TRCK", "Track number", strconv.Itoa(match.track)},
		{"TPOS", "Disc number", strconv.Itoa(match.disc)},
	} {
		if strings.TrimSpace(field.value) != "" {
			base.releaseFields = append(base.releaseFields, proposalField{
				id: field.id, label: field.label, tag: tagComparison{current: current[field.id], proposed: field.value},
			})
		}
	}
	return base
}
