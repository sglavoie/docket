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
	if err := execute(); err != nil {
		if stderr != nil {
			fmt.Fprintln(stderr, err)
		}
		return 1
	}

	return 0
}
