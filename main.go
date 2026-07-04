package main

import (
	"errors"
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

	code := 1
	var exitErr *cmd.ExitCodeError
	if errors.As(err, &exitErr) {
		code = exitErr.Code
	}

	// Suppress the stderr line for errors with no message (e.g. a command that
	// already printed a machine-readable result on stdout and only needs to
	// signal a non-zero exit code).
	if stderr != nil && err.Error() != "" {
		_, _ = fmt.Fprintln(stderr, err)
	}

	return code
}
