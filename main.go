package main

import (
	"fmt"
	"os"

	"github.com/sglavoie/tt/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
