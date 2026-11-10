package cli

import "io"

func run(args []string, out, errOut io.Writer) int {
	return Run(args, nil, out, errOut)
}
