package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
)

const organizeSyntax = "l1ner organize --dry-run <directory>\n       l1ner organize --apply <directory>\n       l1ner organize --undo <journal>"

func runOrganize(args []string, in io.Reader, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("organize", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dryRun := flags.Bool("dry-run", false, "preview album folders")
	apply := flags.Bool("apply", false, "review and move songs into album folders")
	undo := flags.Bool("undo", false, "restore original paths from a journal")
	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		if _, err := fmt.Fprintln(out, "Usage: "+organizeSyntax+"\nPreview album folders, approve moves, or undo an organization run."); err != nil {
			reportError(errOut, err)
			return 1
		}
		return 0
	}
	modes := 0
	for _, selected := range []bool{*dryRun, *apply, *undo} {
		if selected {
			modes++
		}
	}
	if err == nil && modes != 1 {
		err = errors.New("choose one of --dry-run, --apply, or --undo")
	}
	if err == nil && (flags.NArg() != 1 || flags.Arg(0) == "") {
		if *undo {
			err = errors.New("expected one journal path")
		} else {
			err = errors.New("expected one directory path")
		}
	}
	if err != nil {
		reportError(errOut, err)
		fmt.Fprintln(errOut, "Usage: "+organizeSyntax)
		return 2
	}

	if *undo {
		return runOrganizeUndo(flags.Arg(0), out, errOut)
	}
	if *apply {
		choose, ok := terminalMenu(in, out)
		if !ok {
			reportError(errOut, errors.New("organize --apply requires a terminal for input and output"))
			return 1
		}
		return runOrganizeApply(flags.Arg(0), out, errOut, choose)
	}
	plan, err := planAlbumFolders(flags.Arg(0))
	if err != nil {
		reportError(errOut, err)
		return 1
	}
	for _, err := range plan.scanErrors {
		if reportError(errOut, err) != nil {
			return 1
		}
	}
	failed, err := displayAlbumPlan(out, plan)
	if err != nil {
		reportError(errOut, err)
		return 1
	}
	if failed > 0 || len(plan.scanErrors) > 0 {
		return 1
	}
	return 0
}

func displayAlbumPlan(out io.Writer, plan albumPlan) (failed int, err error) {
	if len(plan.moves) == 0 && len(plan.scanErrors) == 0 {
		if _, err := fmt.Fprintln(out, "No MP3 files found."); err != nil {
			return 0, err
		}
	}
	var planned, blocked int
	for _, move := range plan.moves {
		source := safe(filepath.Join(plan.root, move.source))
		switch {
		case move.err != nil:
			failed++
			_, err = fmt.Fprintf(out, "Failed: %s — %s\n", source, safe(move.err.Error()))
		case move.blocked != "":
			blocked++
			_, err = fmt.Fprintf(out, "Blocked: %s — %s\n", source, safe(move.blocked))
		default:
			planned++
			destination := safe(filepath.Join(plan.root, move.destination))
			_, err = fmt.Fprintf(out, "Planned: %s -> %s\n", source, destination)
		}
		if err != nil {
			return failed, err
		}
	}
	_, err = fmt.Fprintf(out, "Summary: %d planned, %d blocked, %d failed, %d scan errors\n",
		planned, blocked, failed, len(plan.scanErrors))
	return failed, err
}
