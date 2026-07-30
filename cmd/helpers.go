package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/tterrors"
)

const defaultDataDirDisplay = "~/.tt (an existing iCloud Drive tt workspace is used if present)"

func successMsg(format string, args ...any) string {
	return icons.Get("success") + " " + fmt.Sprintf(format, args...)
}

func NoCurrentProjectError() error {
	return fmt.Errorf("%w. Open the TUI to select a project or pass a project to the command", tterrors.ErrNoCurrentProject)
}

func Pluralize(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func ConfirmYN(prompt string, force bool) bool {
	if force {
		return true
	}
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
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
