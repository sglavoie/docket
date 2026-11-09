package cmd

import (
	"strings"
	"testing"
)

// --appearance names one of the two built-in themes, and anything else is
// refused before any workspace is opened.
func TestRootRefusesAnUnknownAppearance(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() { appearance = "" })
	// Another test may have left --version set on the shared root command.
	if err := rootCmd.Flags().Set("version", "false"); err != nil {
		t.Fatal(err)
	}
	appearance = "sepia"
	err := runRoot(rootCmd, nil)
	if err == nil || !strings.Contains(err.Error(), `unknown --appearance "sepia"`) {
		t.Fatalf("runRoot = %v, want the --appearance refusal", err)
	}
}
