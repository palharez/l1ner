package main

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
)

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
