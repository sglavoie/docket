package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/sglavoie/tt/internal/tui"
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

// TestRetiredFilterChordVocabularyStaysOutOfComments audits comments alone, so an
// unrelated identifier that happens to spell a retired chord stays legal; only a
// maintainer being told to press a path dispatch rejects is a failure. The
// retired set is explicit rather than "every f-prefixed pair the keymap does not
// bind", because pairs like fn and fg are ordinary variable names.
func TestRetiredFilterChordVocabularyStaysOutOfComments(t *testing.T) {
	replacements := map[string]string{
		"ft":  "f#",
		"Ft":  "F#",
		"fp0": "f!0",
		"fz":  "f@",
	}
	live := make(map[string]bool)
	for _, binding := range tui.SimplifiedInputStrategy().Bindings {
		if len(binding.Sequence) > 0 {
			live[strings.Join(binding.Sequence, "")] = true
		}
	}
	patterns := make(map[string]*regexp.Regexp, len(replacements))
	for retired, replacement := range replacements {
		if live[retired] {
			t.Errorf("%q is bound again; drop it from the retired filter vocabulary rather than rejecting a live path", retired)
		}
		if !live[replacement] {
			t.Errorf("%q replaces %q but is not bound by SimplifiedInputStrategy", replacement, retired)
		}
		patterns[retired] = regexp.MustCompile(`\b` + regexp.QuoteMeta(retired) + `\b`)
	}

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
		file, parseErr := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
		if parseErr != nil {
			return parseErr
		}
		for _, group := range file.Comments {
			for _, comment := range group.List {
				for retired, pattern := range patterns {
					if pattern.MatchString(comment.Text) {
						t.Errorf(
							"%s:%d: comment names the retired filter chord %q; the live path is %q",
							filepath.ToSlash(path),
							fileSet.Position(comment.Slash).Line,
							retired,
							replacements[retired],
						)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRetiredFocusStackZoomVocabulary keeps the Focus stack from being spelled
// as a zoom again. The section zoom is live product vocabulary, so the retired
// set names the Focus stack's own compounds rather than the bare word, which
// would now reject `sectionZoom` and every honest use around it.
func TestRetiredFocusStackZoomVocabulary(t *testing.T) {
	retired := []string{
		"zoomstack",
		"crosszoom",
		"zoomentry",
		"zoomroot",
		"zoomdepth",
		"zoomcontext",
		"zoomboundary",
		"zoomnotice",
		"zoomcopy",
		"zoomable",
		"emptyzoom",
	}
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
		lowered := strings.ToLower(string(data))
		for _, spelling := range retired {
			if strings.Contains(lowered, spelling) {
				t.Errorf("%s contains retired Focus-stack vocabulary %q", filepath.ToSlash(path), spelling)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCurrentProductDocsExcludeRetiredConcepts(t *testing.T) {
	paths := []string{"README.md", "CLAUDE.md", "docs/architecture.md", "docs/tui-testing/README.md"}
	for _, source := range []struct {
		root      string
		extension string
	}{
		{root: "docs/decisions", extension: ".md"},
		{root: "docs/tui-testing/scenarios", extension: ".json"},
	} {
		err := filepath.WalkDir(source.root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && filepath.Ext(path) == source.extension {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
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
