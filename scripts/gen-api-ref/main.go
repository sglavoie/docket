// gen-api-ref generates a compact CLI reference document from the tt Cobra command tree.
// Output is written to stdout as LLM-friendly markdown.
package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/sglavoie/tt/cmd"
)

func main() {
	root := cmd.RootCmd()
	// Force all init() to run by finding all commands
	root.InitDefaultHelpCmd()

	fmt.Println("# tt CLI Reference")
	fmt.Println()
	printCommand(root, "")
}

func printCommand(c *cobra.Command, parentPath string) {
	if c.Hidden {
		return
	}

	fullPath := c.Name()
	if parentPath != "" {
		fullPath = parentPath + " " + c.Name()
	}

	fmt.Printf("## %s\n\n", fullPath)

	if c.Short != "" {
		fmt.Println(c.Short)
		fmt.Println()
	}

	fmt.Printf("Usage: %s\n", c.UseLine())

	if len(c.Aliases) > 0 {
		fmt.Printf("Aliases: %s\n", strings.Join(c.Aliases, ", "))
	}

	if c.Long != "" && c.Long != c.Short {
		fmt.Println()
		fmt.Println(c.Long)
	}

	localFlags := collectFlags(c.LocalFlags())
	if len(localFlags) > 0 {
		fmt.Println()
		fmt.Println("Flags:")
		for _, f := range localFlags {
			fmt.Printf("  %s\n", f)
		}
	}

	inheritedFlags := collectFlags(c.InheritedFlags())
	if len(inheritedFlags) > 0 {
		fmt.Println()
		fmt.Println("Inherited flags:")
		for _, f := range inheritedFlags {
			fmt.Printf("  %s\n", f)
		}
	}

	fmt.Println()

	for _, sub := range c.Commands() {
		printCommand(sub, fullPath)
	}
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
		if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
			def = fmt.Sprintf(" (default %s)", f.DefValue)
		}
		flags = append(flags, fmt.Sprintf("%s %s%s    %s", name, f.Value.Type(), def, f.Usage))
	})
	return flags
}
