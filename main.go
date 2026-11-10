package main

import (
	"os"

	"github.com/palharez/l1ner/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
