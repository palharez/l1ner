package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/palharez/l1ner/internal/identify"
)

const usage = "Usage: l1ner inspect [--recursive] [--json] <file-or-directory>"
const commandUsage = usage + "\n       " + lookupSyntax + "\n       " + identifySyntax + "\n       " + previewSyntax + "\n       " + fixSyntax

type inspectOptions struct {
	path      string
	recursive bool
	json      bool
}

// Run executes l1ner with the provided terminal streams and returns its exit status.
func Run(args []string, in io.Reader, out, errOut io.Writer) int {
	return runCommand(args, in, out, errOut, searchMusicBrainz)
}

func runCommand(args []string, in io.Reader, out, errOut io.Writer, search recordingSearch) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(out, commandUsage+"\n\nAudio identification is optional. Run 'l1ner identify --help' for setup.")
		return 0
	}
	if len(args) > 0 {
		switch args[0] {
		case "lookup":
			return runLookup(args[1:], out, errOut, search)
		case "identify":
			return runIdentify(args[1:], out, errOut, identify.Search)
		case "preview", "fix":
			return runSongReview(args[0], args[1:], in, out, errOut, search, identify.SearchFile, identify.SearchFileForBatch)
		}
	}
	if len(args) == 0 || args[0] != "inspect" {
		fmt.Fprintln(errOut, commandUsage)
		return 2
	}
	options, err := parseInspectOptions(args[1:])
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(out, usage)
		return 0
	}
	if err != nil {
		reportError(errOut, err)
		fmt.Fprintln(errOut, usage)
		return 2
	}
	return runInspection(options, out, errOut)
}

func runInspection(options inspectOptions, out, errOut io.Writer) int {
	if options.json {
		return runJSONInspection(options, out, errOut)
	}
	info, err := os.Stat(options.path)
	if err != nil {
		reportError(errOut, err)
		return 1
	}
	if info.IsDir() {
		return inspectDirectory(options, out, errOut)
	}
	if options.recursive {
		reportError(errOut, errors.New("--recursive requires a directory"))
		return 2
	}
	if err := inspectFile(options.path, out); err != nil {
		reportError(errOut, err)
		return 1
	}
	return 0
}

func parseInspectOptions(args []string) (inspectOptions, error) {
	var options inspectOptions
	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&options.recursive, "recursive", false, "inspect subdirectories")
	flags.BoolVar(&options.json, "json", false, "write one JSON report to stdout")
	if err := flags.Parse(args); err != nil {
		return options, err
	}
	if flags.NArg() != 1 {
		return options, errors.New("expected one file or directory path")
	}
	options.path = flags.Arg(0)
	return options, nil
}

func inspectFile(path string, out io.Writer) error {
	entries, err := inspect(path)
	if err != nil {
		return err
	}
	return display(out, path, entries)
}

func reportError(out io.Writer, err error) error {
	_, writeErr := fmt.Fprintf(out, "Error: %s\n", safe(err.Error()))
	return writeErr
}
