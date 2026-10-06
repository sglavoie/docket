package engine

import (
	"os/exec"
	"strings"
	"testing"
)

// frontendImportPrefixes are the packages that make a binary a frontend: the
// TUI and its terminal stack, and the CLI.
var frontendImportPrefixes = []string{
	"github.com/sglavoie/tt/internal/tui",
	"github.com/sglavoie/tt/cmd",
	"github.com/spf13/cobra",
	"charm.land/",
	"github.com/charmbracelet/",
}

// TestRuntimeImportsNoFrontend holds the runtime, and the headless tests that
// drive it, to a dependency graph without the TUI or the CLI, so the mobile
// bridge can link it alone.
func TestRuntimeImportsNoFrontend(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "-test", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps -test: %v\n%s", err, out)
	}
	deps := strings.Fields(string(out))
	if len(deps) == 0 {
		t.Fatal("go list reported no dependencies")
	}
	for _, dep := range deps {
		for _, prefix := range frontendImportPrefixes {
			if strings.HasPrefix(dep, prefix) {
				t.Errorf("internal/engine depends on frontend package %s", dep)
			}
		}
	}
}
