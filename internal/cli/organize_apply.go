package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

type albumApplySummary struct {
	moved, skipped, blocked, failed, pending int
}

type albumApplyRun struct {
	workspace *albumWorkspace
	journal   *albumJournal
}

func runOrganizeApply(path string, out, errOut io.Writer, choose menuSelector) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	w, err := openAlbumWorkspace(path)
	if err != nil {
		reportError(errOut, err)
		return 1
	}
	defer w.close()
	plan := planAlbumRoot(w.root)
	for _, err := range plan.scanErrors {
		if reportError(errOut, err) != nil {
			return 1
		}
	}
	run := albumApplyRun{workspace: w}
	summary, err := run.apply(ctx, plan, out, choose)
	if run.journal != nil {
		run.journal.file.Close()
	}
	code := 0
	switch {
	case errors.Is(err, errPreviewCanceled) || errors.Is(err, context.Canceled):
		fmt.Fprintln(errOut, "Organization canceled.")
		code = 130
	case err != nil:
		reportError(errOut, err)
		code = 1
	case summary.failed > 0 || len(plan.scanErrors) > 0:
		code = 1
	}
	if _, err := fmt.Fprintf(out, "\nSummary: %d moved, %d skipped, %d blocked, %d failed, %d pending, %d scan errors\n",
		summary.moved, summary.skipped, summary.blocked, summary.failed, summary.pending, len(plan.scanErrors)); err != nil {
		reportError(errOut, err)
		return 1
	}
	return code
}

func (r *albumApplyRun) apply(ctx context.Context, plan albumPlan, out io.Writer, choose menuSelector) (albumApplySummary, error) {
	summary := albumApplySummary{pending: len(plan.moves)}
	for i, move := range plan.moves {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		source := safe(filepath.Join(plan.root, move.source))
		if _, err := fmt.Fprintf(out, "\nSong %d of %d\nSource: %s\n", i+1, len(plan.moves), source); err != nil {
			return summary, err
		}
		decision, err := r.reviewMove(ctx, move, out, choose)
		if decision == reviewAccept {
			summary.moved++
			summary.pending--
		}
		if err == nil {
			switch decision {
			case reviewQuit:
				_, err := fmt.Fprintln(out, "Organization ended.")
				return summary, err
			case reviewSkip:
				summary.skipped++
				summary.pending--
				if _, err := fmt.Fprintln(out, "Skipped song."); err != nil {
					return summary, err
				}
			case reviewAccept:
				if _, err := fmt.Fprintln(out, "Moved song."); err != nil {
					return summary, err
				}
			}
			continue
		}
		var recoveryErr albumRecoveryError
		var uiErr reviewUIError
		if errors.Is(err, context.Canceled) || errors.Is(err, errPreviewCanceled) ||
			errors.As(err, &recoveryErr) || errors.As(err, &uiErr) {
			return summary, err
		}
		summary.pending--
		var blocked albumMoveBlocked
		label := "Failed"
		if errors.As(err, &blocked) {
			summary.blocked++
			label = "Blocked"
		} else {
			summary.failed++
		}
		if _, err := fmt.Fprintf(out, "%s: %s\n", label, safe(err.Error())); err != nil {
			return summary, err
		}
	}
	return summary, ctx.Err()
}

func (r *albumApplyRun) reviewMove(ctx context.Context, move albumMove, out io.Writer, choose menuSelector) (reviewDecision, error) {
	if move.err != nil {
		return reviewUnresolved, move.err
	}
	if move.blocked != "" {
		return reviewUnresolved, albumMoveBlocked{errors.New(move.blocked)}
	}
	song, err := r.workspace.reviewSong(move)
	if err != nil {
		return reviewUnresolved, err
	}
	defer song.file.Close()
	destination := filepath.Join(r.workspace.root.Name(), move.destination)
	if _, err := fmt.Fprintf(out, "Destination: %s\n", safe(destination)); err != nil {
		return reviewUnresolved, reviewUIError{err}
	}
	options := []menuOption{{label: "Skip song"}, {label: "Apply move"}, {label: "Quit"}}
	choice, err := choose(ctx, "Move this MP3?", options, 0)
	if err != nil {
		return reviewUnresolved, err
	}
	switch choice {
	case 0:
		return reviewSkip, nil
	case 1:
		// Apply is an explicit selection. Enter on the default skips the song.
	case 2:
		return reviewQuit, nil
	default:
		return reviewUnresolved, reviewUIError{errors.New("invalid organization review choice")}
	}
	if err := ctx.Err(); err != nil {
		return reviewUnresolved, err
	}
	if r.journal == nil {
		journal, err := r.workspace.createJournal()
		if err != nil {
			return reviewUnresolved, albumRecoveryError{err}
		}
		r.journal = journal
		if _, err := fmt.Fprintf(out, "Journal: %s\n", safe(journal.path)); err != nil {
			return reviewUnresolved, reviewUIError{err}
		}
	}
	moved, err := r.workspace.moveSong(ctx, song, r.journal)
	if moved {
		return reviewAccept, err
	}
	return reviewUnresolved, err
}
