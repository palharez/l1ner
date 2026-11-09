package main

import (
	"fmt"
	"io"
	"os"
)

const usage = "Usage: l1ner inspect <file-path>"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(out, usage)
		return 0
	}
	if len(args) != 2 || args[0] != "inspect" {
		fmt.Fprintln(errOut, usage)
		return 2
	}
	entries, err := inspect(args[1])
	if err == nil {
		err = display(out, args[1], entries)
	}
	if err != nil {
		fmt.Fprintf(errOut, "Error: %s\n", safe(err.Error()))
		return 1
	}
	return 0
}
