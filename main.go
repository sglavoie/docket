package main

import (
	"fmt"
	"io"
	"os"

	"github.com/sglavoie/tt/cmd"
)

func main() {
	os.Exit(run(cmd.Execute, os.Stderr))
}

func run(execute func() error, stderr io.Writer) int {
	err := execute()
	if err == nil {
		return 0
	}

	if stderr != nil && err.Error() != "" {
		_, _ = fmt.Fprintln(stderr, err)
	}

	return 1
}
