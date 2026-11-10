package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

const fixSyntax = "l1ner fix [--from-audio] [--release] <file-or-directory>"

func finishSingleFix(out io.Writer, decision reviewDecision, err error) error {
	if err == nil && decision == reviewQuit {
		_, err = fmt.Fprintln(out, "Review ended.")
	}
	return err
}

func fixSong(ctx context.Context, path string, out io.Writer, searches reviewSearches, choose menuSelector) (reviewDecision, error) {
	target, err := openFixTarget(path, searches.audio != nil, searches.releases != nil)
	if err != nil {
		return reviewUnresolved, err
	}
	defer target.close()
	const applyLabel = "Apply changes to this MP3"
	var review songReview
	if searches.audio == nil {
		review, err = searchAndReview(ctx, out, path, target.tags, searches, choose, applyLabel)
	} else {
		review, err = searchAndReviewAudio(ctx, out, path, target.file, target.tags, searches, choose, applyLabel)
	}
	if err != nil {
		return reviewUnresolved, err
	}
	switch review.decision {
	case reviewAccept:
		return applyReviewedSong(ctx, out, target, review.proposal)
	case reviewSkip:
		if _, err := fmt.Fprintln(out, "Skipped song."); err != nil {
			return reviewSkip, reviewUIError{err}
		}
	}
	return review.decision, nil
}

func applyReviewedSong(ctx context.Context, out io.Writer, target *fixTarget, proposal songProposal) (reviewDecision, error) {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	result, err := target.apply(ctx, proposal)
	if err != nil {
		return reviewUnresolved, err
	}
	if _, err := fmt.Fprintf(out, "Applied approved tags to %s.\nBackup: %s\n", safe(target.path), safe(result.backupPath)); err != nil {
		return reviewAccept, reviewUIError{fmt.Errorf("changes applied and backup saved, but output failed: %w", err)}
	}
	if result.syncWarning != nil {
		if _, err := fmt.Fprintf(out, "Warning: replacement applied, but could not sync its directory: %s\n", safe(result.syncWarning.Error())); err != nil {
			return reviewAccept, reviewUIError{err}
		}
	}
	return reviewAccept, nil
}
