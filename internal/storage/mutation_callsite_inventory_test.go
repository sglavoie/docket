package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// mutationInventoryBaseline is a committed, machine-generated inventory of every
// direct storage mutation callsite in cmd/ and internal/tui/. It is the
// mutation-callsite baseline the consolidation program measures itself against:
// as CLI and TUI mutations move behind the shared application layer, this list
// must shrink, and every change to it surfaces in review as a diff.
//
// Unlike the hand-maintained allowlist in undo_raw_save_audit_test.go, this file
// is regenerated from the AST, so it can never silently drift out of sync with
// the code. Regenerate it with:
//
//	UPDATE_MUTATION_BASELINE=1 go test ./internal/storage/ -run TestMutationCallsiteInventoryBaseline
const mutationInventoryBaseline = "testdata/mutation_callsites_baseline.txt"

// TestMutationCallsiteInventoryBaseline generates the direct-mutation inventory
// from source and compares it against the committed baseline. It reuses the same
// AST collector as the raw-save allowlist audit so the two stay grounded in one
// definition of "direct storage mutation."
func TestMutationCallsiteInventoryBaseline(t *testing.T) {
	root := findRepoRootForUndoAudit(t)
	calls := collectRawProjectSaveCalls(t, root, []string{
		filepath.Join(root, "cmd"),
		filepath.Join(root, "internal", "tui"),
	})

	keys := make([]string, 0, len(calls))
	for _, call := range calls {
		keys = append(keys, call.Key)
	}
	sort.Strings(keys)

	got := renderMutationInventory(keys)

	if os.Getenv("UPDATE_MUTATION_BASELINE") == "1" {
		if err := os.WriteFile(mutationInventoryBaseline, []byte(got), 0o644); err != nil {
			t.Fatalf("write mutation baseline: %v", err)
		}
		t.Logf("regenerated %s with %d callsites", mutationInventoryBaseline, len(keys))
		return
	}

	wantBytes, err := os.ReadFile(mutationInventoryBaseline)
	if err != nil {
		t.Fatalf("read mutation baseline (regenerate with UPDATE_MUTATION_BASELINE=1): %v", err)
	}
	if string(wantBytes) != got {
		t.Fatalf("mutation-callsite inventory drifted from %s.\n"+
			"Route new mutations through the application layer, or regenerate the baseline with:\n"+
			"  UPDATE_MUTATION_BASELINE=1 go test ./internal/storage/ -run TestMutationCallsiteInventoryBaseline",
			mutationInventoryBaseline)
	}
}

func renderMutationInventory(keys []string) string {
	var b strings.Builder
	b.WriteString("# Generated mutation-callsite inventory — direct storage mutation calls in cmd/ and internal/tui/.\n")
	b.WriteString("# Do not edit by hand. Regenerate with:\n")
	b.WriteString("#   UPDATE_MUTATION_BASELINE=1 go test ./internal/storage/ -run TestMutationCallsiteInventoryBaseline\n")
	fmt.Fprintf(&b, "# Total callsites: %d\n", len(keys))
	for _, key := range keys {
		b.WriteString(key)
		b.WriteString("\n")
	}
	return b.String()
}
