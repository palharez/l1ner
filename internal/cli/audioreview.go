package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/palharez/l1ner/internal/identify"
)

type audioCandidateSearch func(context.Context, *os.File, string) ([]identify.Candidate, error)

func reviewFileFromAudio(command, path string, out io.Writer, search audioCandidateSearch, choose menuSelector) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	if command == "fix" {
		decision, fixErr := fixSong(ctx, path, out, nil, search, choose)
		err = finishSingleFix(out, decision, fixErr)
	} else {
		err = previewFromAudio(ctx, path, out, search, choose)
	}
	if errors.Is(err, context.Canceled) {
		return errPreviewCanceled
	}
	return err
}

func previewFromAudio(ctx context.Context, path string, out io.Writer, search audioCandidateSearch, choose menuSelector) error {
	if !strings.EqualFold(filepath.Ext(path), ".mp3") {
		return fmt.Errorf("expected an MP3 file: %s", path)
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	entries, err := inspectOpenMP3(file)
	if err != nil {
		return err
	}
	review, err := searchAndReviewAudio(ctx, out, path, file, tagsForAudioReview(entries), search, choose, "Accept proposal (preview only)")
	return finishPreview(out, review, err)
}

func tagsForAudioReview(entries []entry) songTags {
	return songTags{titles: songTagValues(entries, "TIT2"), artists: songTagValues(entries, "TPE1")}
}

func searchAndReviewAudio(ctx context.Context, out io.Writer, path string, file *os.File, tags songTags, search audioCandidateSearch, choose menuSelector, applyLabel string) (songReview, error) {
	key, err := acoustIDApplicationKey()
	if err != nil {
		return songReview{}, err
	}
	candidates, err := search(ctx, file, key)
	if err != nil {
		return songReview{}, err
	}
	if err := ctx.Err(); err != nil {
		return songReview{}, err
	}
	var matches []recording
	for _, match := range candidates {
		if strings.TrimSpace(match.Title) == "" || strings.TrimSpace(match.Artist) == "" {
			continue
		}
		matches = append(matches, recording{id: match.RecordingID, title: match.Title, artist: match.Artist, acoustIDScore: &match.Score})
	}
	if len(matches) == 0 {
		if _, err := fmt.Fprintf(out, "File: %s\nNo AcoustID recording candidates with both title and artist found.\n", safe(path)); err != nil {
			return songReview{}, reviewUIError{err}
		}
		return songReview{decision: reviewUnresolved}, nil
	}
	return reviewSong(ctx, out, path, tags, matches, choose, applyLabel)
}
