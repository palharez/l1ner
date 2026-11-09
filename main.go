package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

const usage = "Usage: l1ner inspect [--recursive] <file-or-directory>"

type inspectOptions struct {
	path      string
	recursive bool
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(out, usage)
		return 0
	}
	if len(args) == 0 || args[0] != "inspect" {
		fmt.Fprintln(errOut, usage)
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

func reportError(out io.Writer, err error) {
	fmt.Fprintf(out, "Error: %s\n", safe(err.Error()))
}
