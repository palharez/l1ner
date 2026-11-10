package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

type fixBatchSummary struct {
	applied, skipped, unchanged, unresolved, failed, pending int
}

func (s *fixBatchSummary) record(decision reviewDecision) {
	switch decision {
	case reviewAccept:
		s.applied++
	case reviewSkip:
		s.skipped++
	case reviewUnchanged:
		s.unchanged++
	case reviewUnresolved:
		s.unresolved++
	}
	s.pending--
}

func runFixDirectory(path string, out, errOut io.Writer, search recordingSearch, choose menuSelector) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	scan := scanDirectory(path, false)
	for _, err := range scan.errors {
		if reportError(errOut, err) != nil {
			return 1
		}
	}
	if len(scan.paths) == 0 && len(scan.errors) == 0 {
		if _, err := fmt.Fprintln(out, "No MP3 files found."); err != nil {
			reportError(errOut, err)
			return 1
		}
	}
	summary, err := fixBatch(ctx, scan.paths, out, errOut, paceBatchSearch(search), choose)
	code := 0
	switch {
	case errors.Is(err, errPreviewCanceled) || errors.Is(err, context.Canceled):
		if errors.Is(err, errPreviewCanceled) && err != errPreviewCanceled {
			reportError(errOut, err)
		} else {
			fmt.Fprintln(errOut, "Fix canceled.")
		}
		code = 130
	case err != nil:
		reportError(errOut, err)
		code = 1
	case summary.failed > 0 || len(scan.errors) > 0:
		code = 1
	}
	if _, err := fmt.Fprintf(out, "\nSummary: %d applied, %d skipped, %d unchanged, %d unresolved, %d failed, %d pending\n",
		summary.applied, summary.skipped, summary.unchanged, summary.unresolved, summary.failed, summary.pending); err != nil {
		reportError(errOut, err)
		return 1
	}
	return code
}

func fixBatch(ctx context.Context, paths []string, out, errOut io.Writer, search recordingSearch, choose menuSelector) (fixBatchSummary, error) {
	summary := fixBatchSummary{pending: len(paths)}
	for i, path := range paths {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		if _, err := fmt.Fprintf(out, "\nSong %d of %d\n", i+1, len(paths)); err != nil {
			return summary, err
		}
		decision, err := fixSong(ctx, path, out, search, choose)
		if err == nil {
			if decision == reviewQuit {
				_, err := fmt.Fprintln(out, "Review ended.")
				return summary, err
			}
			summary.record(decision)
			continue
		}
		if decision == reviewAccept || decision == reviewSkip {
			// A completed decision still counts if its confirmation could not print.
			summary.record(decision)
		}
		var uiErr reviewUIError
		if errors.Is(err, errPreviewCanceled) || errors.Is(err, context.Canceled) || errors.As(err, &uiErr) {
			return summary, err
		}
		summary.failed++
		summary.pending--
		if err := reportError(errOut, fmt.Errorf("%s: %w", path, err)); err != nil {
			return summary, err
		}
	}
	return summary, ctx.Err()
}
