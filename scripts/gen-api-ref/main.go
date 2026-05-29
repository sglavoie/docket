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
	if err := run(os.Stdout); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "gen-api-ref: %v\n", err)
		os.Exit(1)
	}
}

func run(w io.Writer) error {
	root := cmd.RootCmd()

	if _, err := fmt.Fprintln(w, "# tt CLI Reference"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	return printCommand(w, root, "")
}

func printCommand(w io.Writer, c *cobra.Command, parentPath string) error {
	if c.Hidden || isDefaultHelpCommand(c) {
		return nil
	}

	fullPath := c.Name()
	if parentPath != "" {
		fullPath = parentPath + " " + c.Name()
	}

	if _, err := fmt.Fprintf(w, "## %s\n\n", fullPath); err != nil {
		return err
	}

	if c.Short != "" {
		if _, err := fmt.Fprintln(w, c.Short); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "Usage: %s\n", c.UseLine()); err != nil {
		return err
	}

	if len(c.Aliases) > 0 {
		if _, err := fmt.Fprintf(w, "Aliases: %s\n", strings.Join(c.Aliases, ", ")); err != nil {
			return err
		}
	}

	if c.Long != "" && c.Long != c.Short {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, c.Long); err != nil {
			return err
		}
	}

	localFlags := collectFlags(c.LocalFlags())
	if len(localFlags) > 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Flags:"); err != nil {
			return err
		}
		for _, f := range localFlags {
			if _, err := fmt.Fprintf(w, "  %s\n", f); err != nil {
				return err
			}
		}
	}

	inheritedFlags := collectFlags(c.InheritedFlags())
	if len(inheritedFlags) > 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Inherited flags:"); err != nil {
			return err
		}
		for _, f := range inheritedFlags {
			if _, err := fmt.Fprintf(w, "  %s\n", f); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	for _, sub := range c.Commands() {
		if err := printCommand(w, sub, fullPath); err != nil {
			return err
		}
	}
	return nil
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
		if shouldPrintDefault(f) {
			def = fmt.Sprintf(" (default %s)", f.DefValue)
		}
		flags = append(flags, fmt.Sprintf("%s %s%s    %s", name, f.Value.Type(), def, f.Usage))
	})
	return flags
}

func shouldPrintDefault(f *pflag.Flag) bool {
	switch f.DefValue {
	case "", "[]":
		return false
	case "false":
		return f.Value.Type() != "bool"
	case "0":
		switch f.Value.Type() {
		case "count", "duration", "float32", "float64", "int", "int16", "int32", "int64", "int8", "uint", "uint16", "uint32", "uint64", "uint8":
			return false
		}
		return true
	default:
		return true
	}
}
