package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

func TestRetiredProductVocabularyOwnership(t *testing.T) {
	retiredIdentifiers := map[string]bool{
		"DashboardCoordinator":     true,
		"JumpDashboardMsg":         true,
		"ComposerPlanner":          true,
		"pinnedTagTab":             true,
		"activePinnedTagFilter":    true,
		"taskIsActionableState":    true,
		"AllCaptureDestinations":   true,
		"GetNextOrder":             true,
		"SetNextOrder":             true,
		"SystemInboxProjectID":     true,
		"StateReview":              true,
		"TargetStateForLegacy":     true,
		"RepairSystemInboxProject": true,
		"ProjectBoundary":          true,
		"BoundaryID":               true,
		"ActiveBoundaryID":         true,
		"ActiveBoundaryName":       true,
		"NormalizeBoundaries":      true,
		"AssignProjectBoundary":    true,
		"ProjectBoundaryID":        true,
		"AllContexts":              true,
		"DefaultContext":           true,
		"RegularContext":           true,
		"RegularContextScope":      true,
		"AllContextScope":          true,
		"ContextScope":             true,
		"ContextScopeSummaries":    true,
		"ContextHubModel":          true,
		"ShowContextHubMsg":        true,
		"IsSnoozed":                true,
		"IsSnoozedAt":              true,
		"ClearSnooze":              true,
		"formatSnoozeIndicator":    true,
		"snoozeWakeWindow":         true,
		"ColSnooze":                true,
		"operationTaskSetSnooze":   true,
		"BulkSnooze":               true,
		"SnoozedUntil":             true,
	}
	migrationIdentifierAllowlist := map[string]map[string]bool{
		"SystemInboxProjectID": {
			"internal/config/legacy_v4_domains.go":         true,
			"internal/config/legacy_v4_domains_convert.go": true,
			"internal/config/v6_conversion_dry_run.go":     true,
		},
		"BoundaryID": {
			"internal/config/v7_conversion_dry_run.go": true,
		},
		// The v4/v5/v6 DTOs keep SnoozedUntil because their on-disk key genuinely
		// is `snoozed`, and frozen golden fixtures name it. The v10 migration and
		// the live model-to-document seam read those DTOs, so they are the only
		// callers outside the codecs themselves. The schema-10 document lift is
		// one of them: it is the only place the deferral value is rewritten from
		// a calendar day to an instant, so it must name the DTO field.
		"SnoozedUntil": {
			"internal/storage/format/legacy_v4_task_convert.go": true,
			"internal/storage/format/v5_task_codec.go":          true,
			"internal/storage/format/v6_task_codec.go":          true,
			"internal/storage/format/v6_task_convert.go":        true,
			"internal/storage/project_document.go":              true,
			"internal/storage/v10_project_document.go":          true,
			"internal/config/v10_migration_activation.go":       true,
			"internal/config/v11_migration_activation.go":       true,
		},
	}
	rejectedInputAllowlist := map[string]bool{
		"internal/app/capture.go": true,
	}
	rejectedInputTokens := []string{"$ready", "$in_progress", "$review"}

	fileSet := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative := filepath.ToSlash(path)
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.Ident:
				if !retiredIdentifiers[value.Name] {
					return true
				}
				if migrationIdentifierAllowlist[value.Name][relative] {
					return true
				}
				t.Errorf("%s:%d: retired product identifier %q escaped its migration boundary", relative, fileSet.Position(value.Pos()).Line, value.Name)
			case *ast.BasicLit:
				if value.Kind != token.STRING {
					return true
				}
				literal, unquoteErr := strconv.Unquote(value.Value)
				if unquoteErr != nil {
					return true
				}
				for _, retired := range rejectedInputTokens {
					if strings.Contains(strings.ToLower(literal), retired) && !rejectedInputAllowlist[relative] {
						t.Errorf("%s:%d: retired input token %q escaped rejected-input guidance", relative, fileSet.Position(value.Pos()).Line, retired)
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRetiredFocusStackZoomVocabulary(t *testing.T) {
	err := filepath.WalkDir("internal", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(strings.ToLower(string(data)), "zoom") {
			t.Errorf("%s contains retired Focus-stack vocabulary %q", filepath.ToSlash(path), "zoom")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCurrentProductDocsExcludeRetiredConcepts(t *testing.T) {
	paths := []string{"README.md", "CLAUDE.md", "docs/architecture.md", "docs/tui-testing/README.md"}
	err := filepath.WalkDir("docs/tui-testing/scenarios", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && filepath.Ext(path) == ".json" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	retired := map[string]bool{
		"dashboard":  true,
		"planner":    true,
		"board":      true,
		"roadmap":    true,
		"analytics":  true,
		"actionable": true,
	}
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		for _, word := range strings.FieldsFunc(strings.ToLower(string(data)), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		}) {
			if retired[word] {
				t.Errorf("%s contains retired product concept %q", path, word)
			}
		}
	}
}
