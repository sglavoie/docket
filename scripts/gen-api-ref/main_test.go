package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestPrintCommandSkipsDefaultHelpCommand(t *testing.T) {
	root := &cobra.Command{Use: "tt"}
	root.AddCommand(&cobra.Command{Use: "add"})
	root.InitDefaultHelpCmd()

	var out bytes.Buffer
	printCommand(&out, root, "")

	got := out.String()
	if strings.Contains(got, "## tt help") {
		t.Fatalf("printCommand() included Cobra default help command:\n%s", got)
	}
	if !strings.Contains(got, "## tt add") {
		t.Fatalf("printCommand() did not include normal subcommand:\n%s", got)
	}
}

func TestCollectFlagsSuppressesEmptySliceDefaults(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.StringSliceP("tag", "T", nil, "tags for the task")

	got := collectFlags(flags)
	if len(got) != 1 {
		t.Fatalf("collectFlags() returned %d flags, want 1: %#v", len(got), got)
	}
	if strings.Contains(got[0], "(default [])") {
		t.Fatalf("collectFlags() included empty slice default: %q", got[0])
	}
	if want := "--tag, -T stringSlice    tags for the task"; got[0] != want {
		t.Fatalf("collectFlags() = %q, want %q", got[0], want)
	}
}

func TestCollectFlagsKeepsMeaningfulDefaults(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("format", "ascii", "output format")

	got := collectFlags(flags)
	if len(got) != 1 {
		t.Fatalf("collectFlags() returned %d flags, want 1: %#v", len(got), got)
	}
	if want := "--format string (default ascii)    output format"; got[0] != want {
		t.Fatalf("collectFlags() = %q, want %q", got[0], want)
	}
}
