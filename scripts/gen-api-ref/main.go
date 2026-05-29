// gen-api-ref generates a compact CLI reference document from the tt Cobra command tree.
// Output is written to stdout as LLM-friendly markdown.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/sglavoie/tt/cmd"
)

func main() {
	root := cmd.RootCmd()

	fmt.Fprintln(os.Stdout, "# tt CLI Reference")
	fmt.Fprintln(os.Stdout)
	printCommand(os.Stdout, root, "")
}

func printCommand(w io.Writer, c *cobra.Command, parentPath string) {
	if c.Hidden || isDefaultHelpCommand(c) {
		return
	}

	fullPath := c.Name()
	if parentPath != "" {
		fullPath = parentPath + " " + c.Name()
	}

	fmt.Fprintf(w, "## %s\n\n", fullPath)

	if c.Short != "" {
		fmt.Fprintln(w, c.Short)
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "Usage: %s\n", c.UseLine())

	if len(c.Aliases) > 0 {
		fmt.Fprintf(w, "Aliases: %s\n", strings.Join(c.Aliases, ", "))
	}

	if c.Long != "" && c.Long != c.Short {
		fmt.Fprintln(w)
		fmt.Fprintln(w, c.Long)
	}

	localFlags := collectFlags(c.LocalFlags())
	if len(localFlags) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Flags:")
		for _, f := range localFlags {
			fmt.Fprintf(w, "  %s\n", f)
		}
	}

	inheritedFlags := collectFlags(c.InheritedFlags())
	if len(inheritedFlags) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Inherited flags:")
		for _, f := range inheritedFlags {
			fmt.Fprintf(w, "  %s\n", f)
		}
	}

	fmt.Fprintln(w)

	for _, sub := range c.Commands() {
		printCommand(w, sub, fullPath)
	}
}

func isDefaultHelpCommand(c *cobra.Command) bool {
	return c.Name() == "help" && c.Use == "help [command]" && c.Short == "Help about any command"
}

func collectFlags(fs *pflag.FlagSet) []string {
	var flags []string
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		name := "--" + f.Name
		if f.Shorthand != "" {
			name += ", -" + f.Shorthand
		}
		def := ""
		if shouldPrintDefault(f.DefValue) {
			def = fmt.Sprintf(" (default %s)", f.DefValue)
		}
		flags = append(flags, fmt.Sprintf("%s %s%s    %s", name, f.Value.Type(), def, f.Usage))
	})
	return flags
}

func shouldPrintDefault(defValue string) bool {
	switch defValue {
	case "", "false", "0", "[]":
		return false
	default:
		return true
	}
}
