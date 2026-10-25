package main

import (
	"fmt"
	"os"

	"github.com/sglavoie/docket/internal/deadcodeaudit"
)

func main() {
	findings, err := deadcodeaudit.Run("", os.Args[1:]...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, finding := range findings {
		fmt.Println(finding)
	}
	if len(findings) > 0 {
		os.Exit(1)
	}
}
