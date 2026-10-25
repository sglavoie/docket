package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// runRootForLogging runs args through executeCommand against rootCmd with the
// real logging seams, under a throwaway HOME, and returns that HOME.
func runRootForLogging(t *testing.T, args ...string) string {
	t.Helper()
	home := t.TempDir()
	runRootForLoggingIn(t, home, args...)
	return home
}

// runRootForLoggingIn is runRootForLogging under a HOME the caller prepared.
func runRootForLoggingIn(t *testing.T, home string, args ...string) {
	t.Helper()
	t.Setenv("HOME", home)
	rootCmd.SetArgs(args)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	// printVersion and the generated completion script write to os.Stdout.
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	stdout := os.Stdout
	os.Stdout = devNull
	t.Cleanup(func() {
		os.Stdout = stdout
		_ = devNull.Close()
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		_ = rootCmd.Flags().Set("version", "false")
	})
	if err := executeCommand(rootCmd); err != nil {
		t.Fatalf("docket %s: %v", strings.Join(args, " "), err)
	}
}

// A logging command adopts a tt-era ~/.tt before it opens its log or reads the
// active-workspace pointer, so logging opens under ~/.docket and the pointer
// follows the workspace it named.
func TestLoggingCommandAdoptsLegacyLocalRoot(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, ".tt")
	pointer := filepath.Join("runtime", "active-workspace.json")
	if err := os.MkdirAll(filepath.Join(legacy, "runtime"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, pointer), []byte(`{"schema_version":1,"data_dir":"`+legacy+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	runRootForLoggingIn(t, home, "workspace", "location")

	if _, err := os.Lstat(legacy); !os.IsNotExist(err) {
		t.Fatalf("%s survived adoption (lstat err = %v)", legacy, err)
	}
	adopted := filepath.Join(home, ".docket")
	data, err := os.ReadFile(filepath.Join(adopted, pointer))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"data_dir": "`+adopted+`"`) {
		t.Fatalf("pointer was not rewritten to %s: %s", adopted, data)
	}
	// An empty log file is removed at close, but its directory stays.
	if info, err := os.Stat(filepath.Join(adopted, "logs")); err != nil || !info.IsDir() {
		t.Fatalf("logging did not open under %s (stat err = %v)", adopted, err)
	}
}

// A command that never logs must not create ~/.docket/logs, so --version and shell
// completion leave HOME untouched.
func TestNonLoggingCommandsCreateNoLogDirectory(t *testing.T) {
	for _, args := range [][]string{
		{"--version"},
		{"completion", "zsh"},
		{"help"},
		{cobra.ShellCompRequestCmd, "doc"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			home := runRootForLogging(t, args...)
			if _, err := os.Stat(filepath.Join(home, ".docket")); !os.IsNotExist(err) {
				t.Fatalf("docket %s created %s (stat err = %v)", strings.Join(args, " "), filepath.Join(home, ".docket"), err)
			}
		})
	}
}

func TestCommandLogsSkipsOnlyNonLoggingCommands(t *testing.T) {
	for _, tc := range []struct {
		path []string
		want bool
	}{
		{[]string{"doctor"}, true},
		{[]string{"conflict", "list"}, true},
		{[]string{"help"}, false},
		{[]string{"completion"}, false},
		{[]string{"completion", "zsh"}, false},
		{[]string{"completion", "bash"}, false},
		{[]string{cobra.ShellCompRequestCmd}, false},
		{[]string{cobra.ShellCompNoDescRequestCmd}, false},
	} {
		t.Run(strings.Join(tc.path, "_"), func(t *testing.T) {
			rootCmd.InitDefaultHelpCmd()
			rootCmd.InitDefaultCompletionCmd()
			cmd, _, err := rootCmd.Find(tc.path)
			if err != nil {
				t.Fatalf("Find(%v): %v", tc.path, err)
			}
			if got := commandLogs(cmd); got != tc.want {
				t.Errorf("commandLogs(%s) = %v, want %v", cmd.CommandPath(), got, tc.want)
			}
		})
	}
}

// The TUI path logs, so the bare root command opens logging before runRoot.
func TestRootPreRunOpensLoggingForTheOutliner(t *testing.T) {
	originalOpen := openLogging
	t.Cleanup(func() { openLogging = originalOpen })
	opened := false
	openLogging = func() { opened = true }

	if err := rootCmd.PersistentPreRunE(rootCmd, nil); err != nil {
		t.Fatalf("PersistentPreRunE: %v", err)
	}
	if !opened {
		t.Fatal("the root pre-run hook should open logging for the Outliner")
	}
}
