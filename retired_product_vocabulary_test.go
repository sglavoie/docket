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

	"github.com/sglavoie/docket/internal/testutil/gotree"

	"github.com/sglavoie/docket/internal/tui"
)

func TestRetiredProductVocabularyOwnership(t *testing.T) {
	t.Parallel()
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
		// The v5 domain candidate DTOs still carry the retired pointer because
		// the schema-4 catalog on disk genuinely spells it; the v5 split moved
		// that boundary across three files without widening it.
		"SystemInboxProjectID": {
			"internal/config/legacy_v4_domains.go":         true,
			"internal/config/legacy_v4_domains_convert.go": true,
			"internal/config/v5_domain_candidates.go":      true,
			"internal/config/v5_domain_validate.go":        true,
			"internal/config/v6_conversion_convert.go":     true,
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
			"internal/storage/format/v5_task_body.go":           true,
			"internal/storage/format/v5_task_fields.go":         true,
			"internal/storage/format/v5_task_validate.go":       true,
			"internal/storage/format/v6_task_codec.go":          true,
			"internal/storage/format/v6_task_body.go":           true,
			"internal/storage/format/v6_task_fields.go":         true,
			"internal/storage/format/v6_task_validate.go":       true,
			"internal/storage/format/v6_task_convert.go":        true,
			"internal/storage/current_task_conversion.go":       true,
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
			if path != "." && (gotree.Ignored(entry.Name()) || entry.Name() == "vendor") {
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
	t.Parallel()
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
			if path != "." && (gotree.Ignored(entry.Name()) || entry.Name() == "vendor") {
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
	t.Parallel()
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

// TestCurrentProductDocsExcludeRetiredSchemaVocabularyInCLIHelp audits the
// Cobra `Long` and `Short` strings, which are the first product text a new user
// reads and the one body of user-facing prose no documentation test covered.
// The rule is mechanical: a hyphenated schema version named in help output must
// be a live one — schema-18 catalogs or the schema-16 task Markdown they hold.
// Retired versions still belong in diagnostic output such as doctor's pre-v6
// snapshot note, which describes a historical boundary rather than the current
// format, so the scan is deliberately narrowed to help fields.
func TestCurrentProductDocsExcludeRetiredSchemaVocabularyInCLIHelp(t *testing.T) {
	// The pair is not contiguous: catalog schema 18 holds task Markdown at schema
	// 16, so no live help string may name schema 17.
	liveSchemaVersions := map[string]bool{"16": true, "18": true}
	schemaVersionPattern := regexp.MustCompile(`(?i)schema[- ]([0-9]+)`)
	helpFields := map[string]bool{"Long": true, "Short": true, "Example": true}

	var scanned int
	fileSet := token.NewFileSet()
	err := filepath.WalkDir("cmd", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative := filepath.ToSlash(path)
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			pair, ok := node.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := pair.Key.(*ast.Ident)
			if !ok || !helpFields[key.Name] {
				return true
			}
			literal, ok := pair.Value.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, unquoteErr := strconv.Unquote(literal.Value)
			if unquoteErr != nil {
				return true
			}
			scanned++
			for _, match := range schemaVersionPattern.FindAllStringSubmatch(value, -1) {
				if !liveSchemaVersions[match[1]] {
					t.Errorf(
						"%s:%d: command help field %s names retired schema vocabulary %q",
						relative, fileSet.Position(literal.Pos()).Line, key.Name, match[0],
					)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned == 0 {
		t.Fatal("scanned no command help strings; the audit is not reaching cmd/")
	}
}

func TestCurrentProductDocsExcludeRetiredConcepts(t *testing.T) {
	paths := []string{"README.md", "CLAUDE.md", "docs/architecture.md", "docs/tui-testing/README.md"}
	for _, source := range []struct {
		root      string
		extension string
	}{
		{root: "docs/architecture", extension: ".md"},
		{root: "docs/decisions", extension: ".md"},
		{root: "docs/guide", extension: ".md"},
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

// retiredProductNamePattern matches tt as a standalone word: the retired
// product and command name. Formats and markers that only start with it
// (tt-sync/v1, <!-- tt-schema: N -->, [[tt:…]]) and the `.tt` directory are
// on-disk inputs rather than names, so they fall outside the word boundary.
var retiredProductNamePattern = regexp.MustCompile(`(^|[^A-Za-z0-9_./\[~-])tt($|[^A-Za-z0-9_:-])`)

// retiredProductNameAllowed lists, per file, the exact spellings that still
// legitimately say tt. Each one is an input or protocol a later step of the
// rename owns, or a name that never was this product's: removing the entry is
// how that step proves it renamed both sides.
var retiredProductNameAllowed = map[string][]string{
	// The enrollment staging marker is an in-workspace name the workspace
	// migration owns.
	"internal/config/sync_enroll_staging.go": {`"tt enrollment staging\n"`},
	// Homebrew's own tt formula is an unrelated program, and the published
	// cask keeps its name until the release that renames it.
	"README.md": {"Homebrew's own `tt` formula", "brew upgrade --cask tt", "sglavoie/tap/tt", "sglavoie/tt@latest"},
}

// retiredProductNameMigrationHeading marks a Markdown section that describes
// the rename itself, where the old name is the subject rather than a slip.
var retiredProductNameMigrationHeading = regexp.MustCompile(`(?i)^#+ .*migrat`)

// TestRetiredProductNameStaysInMigrationSections keeps tt from creeping back
// after the rename to Docket. The command is spelled docket and the product
// Docket; the old name may only appear where a migration reads it, inside a
// Markdown migration section, or in the allowlist above. Go string literals
// are scanned because they are what users read; comments are not product text.
func TestRetiredProductNameStaysInMigrationSections(t *testing.T) {
	t.Parallel()
	checkRetiredProductNameInGoLiterals(t)
	checkRetiredProductNameInDocs(t)
}

// checkRetiredProductName reports text that still says tt once the spellings
// allowed in this file are removed.
func checkRetiredProductName(t *testing.T, relative string, line int, text string) {
	t.Helper()
	for _, allowed := range retiredProductNameAllowed[relative] {
		text = strings.ReplaceAll(text, allowed, "")
	}
	if retiredProductNamePattern.MatchString(text) {
		t.Errorf("%s:%d: names the retired product tt; say docket for the command or Docket for the product: %q", relative, line, strings.TrimSpace(text))
	}
}

// checkRetiredProductNameInGoLiterals scans every production string literal
// outside the migration and legacy files, which read the old name as input.
func checkRetiredProductNameInGoLiterals(t *testing.T) {
	t.Helper()
	fileSet := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != "." && (gotree.Ignored(entry.Name()) || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if filepath.Ext(path) != ".go" || strings.HasSuffix(name, "_test.go") || strings.Contains(name, "migration") || strings.Contains(name, "legacy") {
			return nil
		}
		relative := filepath.ToSlash(path)
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING {
				checkRetiredProductName(t, relative, fileSet.Position(literal.Pos()).Line, literal.Value)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// checkRetiredProductNameInDocs scans the user-facing docs and scenarios line
// by line, skipping each Markdown section whose heading names a migration.
func checkRetiredProductNameInDocs(t *testing.T) {
	t.Helper()
	docs := []string{"README.md", "docs/architecture.md", "docs/tui-testing/README.md"}
	for _, source := range []struct {
		root      string
		extension string
	}{
		{root: "docs/architecture", extension: ".md"},
		{root: "docs/guide", extension: ".md"},
		{root: "docs/tui-testing/scenarios", extension: ".json"},
	} {
		err := filepath.WalkDir(source.root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && filepath.Ext(path) == source.extension {
				docs = append(docs, filepath.ToSlash(path))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range docs {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		inMigration, migrationLevel := false, 0
		for index, line := range strings.Split(string(data), "\n") {
			if level := len(line) - len(strings.TrimLeft(line, "#")); level > 0 && strings.HasPrefix(line[level:], " ") {
				switch {
				case retiredProductNameMigrationHeading.MatchString(line):
					inMigration, migrationLevel = true, level
				case inMigration && level <= migrationLevel:
					inMigration = false
				}
			}
			if !inMigration {
				checkRetiredProductName(t, path, index+1, line)
			}
		}
	}
}
