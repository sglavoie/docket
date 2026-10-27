package cmd

import (
	"strings"
	"testing"
)

// A headless session is reached only through its control socket, so asking
// for one without a socket is refused before any workspace is opened.
func TestHeadlessRequiresAControlSocket(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() {
		headless, controlSocket = false, ""
	})
	// Another test may have left --version set on the shared root command.
	if err := rootCmd.Flags().Set("version", "false"); err != nil {
		t.Fatal(err)
	}
	headless, controlSocket = true, ""
	err := runRoot(rootCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--headless needs --control") {
		t.Fatalf("runRoot = %v, want the --control refusal", err)
	}
}
