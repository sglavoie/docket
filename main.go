package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/sglavoie/docket/cmd"
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

	// An error may choose its own status: doctor exits 2 for "issues found" so
	// a script can tell it apart from "could not run", which is always 1.
	var coded interface{ ExitCode() int }
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	return 1
}
