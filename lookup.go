package main

import (
	"fmt"
	"strings"

	"github.com/bogem/id3v2/v2"
)

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
