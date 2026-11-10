package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

const fixSyntax = "l1ner fix <file.mp3>"

func fixFile(path string, out io.Writer, search recordingSearch, choose menuSelector) error {
	target, err := openFixTarget(path)
	if err != nil {
		return err
	}
	defer target.close()
	review, err := searchAndReview(context.Background(), out, path, target.tags, search, choose, "Apply changes to this MP3")
	if err != nil {
		return err
	}
	switch review.decision {
	case reviewAccept:
		return applyReviewedSong(out, target, review.proposal)
	case reviewSkip:
		_, err = fmt.Fprintln(out, "Skipped song.")
	case reviewQuit:
		_, err = fmt.Fprintln(out, "Review ended.")
	}
	return err
}

func applyReviewedSong(out io.Writer, target *fixTarget, proposal songProposal) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	result, err := target.apply(ctx, proposal)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Applied approved tags to %s.\nBackup: %s\n", safe(target.path), safe(result.backupPath)); err != nil {
		return fmt.Errorf("changes applied and backup saved, but output failed: %w", err)
	}
	if result.syncWarning != nil {
		_, err = fmt.Fprintf(out, "Warning: replacement applied, but could not sync its directory: %s\n", safe(result.syncWarning.Error()))
	}
	return err
}
