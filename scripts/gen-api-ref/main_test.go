package main

import (
	"bytes"
	"errors"
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
	if err := printCommand(&out, root, ""); err != nil {
		t.Fatalf("printCommand() returned error: %v", err)
	}

	got := out.String()
	if strings.Contains(got, "## tt help") {
		t.Fatalf("printCommand() included Cobra default help command:\n%s", got)
	}
	if !strings.Contains(got, "## tt add") {
		t.Fatalf("printCommand() did not include normal subcommand:\n%s", got)
	}
}

func TestPrintCommandReturnsWriteError(t *testing.T) {
	wantErr := errors.New("write failed")
	cmd := &cobra.Command{Use: "tt"}

	err := printCommand(errorWriter{err: wantErr}, cmd, "")
	if !errors.Is(err, wantErr) {
		t.Fatalf("printCommand() error = %v, want %v", err, wantErr)
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

func TestCollectFlagsSkipsDeprecatedFlags(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("current", "", "current flag")
	flags.String("old", "", "deprecated flag")
	flag := flags.Lookup("old")
	flag.Deprecated = "use --current instead"

	got := collectFlags(flags)
	if len(got) != 1 {
		t.Fatalf("collectFlags() returned %d flags, want 1: %#v", len(got), got)
	}
	if strings.Contains(got[0], "--old") {
		t.Fatalf("collectFlags() included deprecated flag: %#v", got)
	}
	if want := "--current string    current flag"; got[0] != want {
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

func TestCollectFlagsKeepsStringDefaultsThatLookZeroValued(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("priority", "0", "default priority label")
	flags.String("archived", "false", "default archived label")

	got := collectFlags(flags)
	if len(got) != 2 {
		t.Fatalf("collectFlags() returned %d flags, want 2: %#v", len(got), got)
	}

	want := []string{
		"--archived string (default false)    default archived label",
		"--priority string (default 0)    default priority label",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("collectFlags()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCollectFlagsSuppressesBoolAndNumericZeroDefaults(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.Bool("verbose", false, "show verbose output")
	flags.Int("limit", 0, "maximum items")

	got := collectFlags(flags)
	if len(got) != 2 {
		t.Fatalf("collectFlags() returned %d flags, want 2: %#v", len(got), got)
	}

	want := []string{
		"--limit int    maximum items",
		"--verbose bool    show verbose output",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("collectFlags()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}
