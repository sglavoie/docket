package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/icons"
)

const defaultDataDirDisplay = "~/.tt, or an existing iCloud Drive tt workspace"

func successMsg(format string, args ...any) string {
	return icons.Get("success") + " " + fmt.Sprintf(format, args...)
}

func Pluralize(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

// confirmInput is where confirmPrompt reads its answer, a seam a test replaces.
var confirmInput io.Reader = os.Stdin

// confirmation is how a y/N prompt ended. A declined prompt is the operator's
// answer, while no answer at all — EOF, a closed or non-interactive stdin — is
// what a script that forgot --force produces, so callers can tell them apart.
type confirmation int

const (
	confirmNoAnswer confirmation = iota
	confirmDeclined
	confirmAccepted
)

// confirmPrompt asks prompt on stdout and reads one line from confirmInput.
// force answers yes without asking. Only "y" or "yes" accept; any other line,
// the empty one included, declines, since the prompt's default is No.
func confirmPrompt(prompt string, force bool) confirmation {
	if force {
		return confirmAccepted
	}
	fmt.Print(prompt)
	reader := bufio.NewReader(confirmInput)
	response, err := reader.ReadString('\n')
	if err != nil {
		// No answer arrived, so no newline was echoed: end the prompt's line
		// before whatever the caller prints next.
		fmt.Println()
		return confirmNoAnswer
	}
	response = strings.TrimSpace(strings.ToLower(response))
	if response == "y" || response == "yes" {
		return confirmAccepted
	}
	return confirmDeclined
}

// noArgs is the positional-argument validator for every command that takes
// none. cobra.NoArgs calls any stray word an unknown command and, because it
// replaces cobra's legacy root check, drops the "Did you mean" suggestions. On
// a command with subcommands the word most likely is a mistyped subcommand, so
// it is refused as one with those suggestions; a leaf command says plainly that
// it takes no arguments.
func noArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	if cmd.HasAvailableSubCommands() {
		return usageRefusal(cmd, fmt.Errorf("unknown command %q for %q%s",
			args[0], cmd.CommandPath(), commandSuggestions(cmd, args[0])))
	}
	return usageRefusal(cmd, fmt.Errorf("%s takes no arguments; got %q", cmd.CommandPath(), args[0]))
}

// exactlyOneArg is cobra.ExactArgs(1) naming what the argument is, so the
// refusal says what to supply instead of "accepts 1 arg(s)".
func exactlyOneArg(what string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		switch {
		case len(args) == 0:
			return usageRefusal(cmd, fmt.Errorf("%s requires a %s", cmd.CommandPath(), what))
		case len(args) > 1:
			return usageRefusal(cmd, fmt.Errorf("%s takes one %s; got %d arguments", cmd.CommandPath(), what, len(args)))
		}
		return nil
	}
}

// atMostOneArg is cobra.MaximumNArgs(1) naming what the argument is.
func atMostOneArg(what string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			return usageRefusal(cmd, fmt.Errorf("%s takes at most one %s; got %d arguments", cmd.CommandPath(), what, len(args)))
		}
		return nil
	}
}

// commandSuggestions renders cobra's own "Did you mean this?" block for a
// mistyped subcommand, or nothing when no subcommand is close.
func commandSuggestions(cmd *cobra.Command, typed string) string {
	if cmd.DisableSuggestions {
		return ""
	}
	if cmd.SuggestionsMinimumDistance <= 0 {
		cmd.SuggestionsMinimumDistance = 2
	}
	suggestions := cmd.SuggestionsFor(typed)
	if len(suggestions) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\nDid you mean this?\n")
	for _, suggestion := range suggestions {
		fmt.Fprintf(&b, "\t%s\n", suggestion)
	}
	return b.String()
}

// argsRefusal wraps a cobra positional-argument validator so its refusal reads
// like every other command-line refusal: the error and a pointer to --help,
// rather than the error buried under the whole usage block.
func argsRefusal(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validate(cmd, args); err != nil {
			return usageRefusal(cmd, err)
		}
		return nil
	}
}

// usageRefusal is the one shape a command-line refusal cobra raises before a
// command runs takes: it silences the usage dump cobra would print and ends
// the error with the --help pointer instead.
func usageRefusal(cmd *cobra.Command, err error) error {
	cmd.SilenceUsage = true
	message := strings.TrimRight(err.Error(), "\n")
	separator := "\n"
	if strings.Contains(message, "\n") {
		separator = "\n\n"
	}
	return fmt.Errorf("%s%sRun '%s --help' for usage", message, separator, cmd.CommandPath())
}

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
