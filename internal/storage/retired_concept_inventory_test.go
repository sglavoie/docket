package storage

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const retiredConceptInventoryBaseline = "testdata/v5_consolidation/retired_concepts_baseline.txt"

var retiredConceptNames = []string{
	"custom-context",
	"decision",
	"dependency",
	"recurrence",
}

// TestRetiredConceptInventoryBaseline locks every production identifier owned
// by a concept that schema 6 removes from live code. Later retirement sessions
// must regenerate the baseline, making each removal (and any accidental new
// usage) visible as a reviewable static inventory diff.
func TestRetiredConceptInventoryBaseline(t *testing.T) {
	root := findRepoRootForUndoAudit(t)
	counts := collectRetiredConceptIdentifiers(t, root)
	got := renderRetiredConceptInventory(counts)

	if os.Getenv("UPDATE_RETIRED_CONCEPT_BASELINE") == "1" {
		if err := os.WriteFile(retiredConceptInventoryBaseline, []byte(got), 0o644); err != nil {
			t.Fatalf("write retired-concept baseline: %v", err)
		}
		t.Logf("regenerated %s with %d inventory rows", retiredConceptInventoryBaseline, len(counts))
		return
	}

	want, err := os.ReadFile(retiredConceptInventoryBaseline)
	if err != nil {
		t.Fatalf("read retired-concept baseline (regenerate with UPDATE_RETIRED_CONCEPT_BASELINE=1): %v", err)
	}
	if string(want) != got {
		t.Fatalf("retired-concept inventory drifted from %s; inspect the production ownership change and regenerate with:\n  UPDATE_RETIRED_CONCEPT_BASELINE=1 go test ./internal/storage -run TestRetiredConceptInventoryBaseline", retiredConceptInventoryBaseline)
	}
}

func collectRetiredConceptIdentifiers(t *testing.T, root string) map[string]int {
	t.Helper()
	counts := make(map[string]int)
	seenConcept := make(map[string]bool)
	for _, base := range []string{filepath.Join(root, "cmd"), filepath.Join(root, "internal")} {
		err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if parseErr != nil {
				return fmt.Errorf("parse %s: %w", path, parseErr)
			}
			rel := slashPath(t, root, path)
			ast.Inspect(file, func(node ast.Node) bool {
				ident, ok := node.(*ast.Ident)
				if !ok {
					return true
				}
				concept := retiredConceptForIdentifier(ident.Name)
				if concept == "" {
					return true
				}
				seenConcept[concept] = true
				counts[concept+"\t"+rel+"\t"+ident.Name]++
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk retired-concept source inventory: %v", err)
		}
	}
	for _, concept := range retiredConceptNames {
		if !seenConcept[concept] {
			t.Errorf("retired-concept inventory has no production identifiers for %s", concept)
		}
	}
	return counts
}

func retiredConceptForIdentifier(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "customcontext"):
		return "custom-context"
	case strings.Contains(lower, "decision"):
		return "decision"
	case strings.HasPrefix(lower, "depend") || strings.Contains(lower, "dependency") || strings.Contains(lower, "dependencies"):
		return "dependency"
	case strings.Contains(lower, "recurr"):
		return "recurrence"
	default:
		return ""
	}
}

func renderRetiredConceptInventory(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteString("# Generated schema-6 retired-concept identifier inventory.\n")
	builder.WriteString("# Columns: concept, production path, identifier, AST occurrence count.\n")
	builder.WriteString("# Regenerate after reviewing ownership changes with:\n")
	builder.WriteString("#   UPDATE_RETIRED_CONCEPT_BASELINE=1 go test ./internal/storage -run TestRetiredConceptInventoryBaseline\n")
	fmt.Fprintf(&builder, "# Total rows: %d\n", len(keys))
	for _, key := range keys {
		fmt.Fprintf(&builder, "%s\t%d\n", key, counts[key])
	}
	return builder.String()
}
