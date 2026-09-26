package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sglavoie/tt/internal/icons"
)

const defaultDataDirDisplay = "~/.tt (an existing iCloud Drive tt workspace is used if present)"

func successMsg(format string, args ...any) string {
	return icons.Get("success") + " " + fmt.Sprintf(format, args...)
}

func Pluralize(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

// confirmInput is where ConfirmYN reads its answer, a seam a test replaces.
var confirmInput io.Reader = os.Stdin

func ConfirmYN(prompt string, force bool) bool {
	if force {
		return true
	}
	fmt.Print(prompt)
	reader := bufio.NewReader(confirmInput)
	response, err := reader.ReadString('\n')
	if err != nil {
		// No answer arrived, so no newline was echoed: end the prompt's line
		// before whatever the caller prints next.
		fmt.Println()
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
