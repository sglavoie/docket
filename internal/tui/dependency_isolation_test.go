package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestDependencyIsolationPresentationVocabulary is the negative half of the
// presentation contract. The identifier map fences out the retired
// dependency-management panels, selectors, viewers, styles, filters, and
// operations; the vocabulary sweep rejects "blocker" anywhere in internal/tui
// production code — filename, identifier, or string literal — so no surface can
// *render* a reference as if it gated work even while calling it something else.
//
// "blocked" itself is live vocabulary now, because the search grammar owns the
// structural fact `?blocked` and the row and Peek own its counts. What it may
// never be is bare: a literal reading exactly "blocked" is chrome a reader can
// mistake for lifecycle Waiting, so the sweep rejects that spelling and requires
// the sigil-qualified one.
//
// TestDependencyIsolationPresentationIsStateIndependent below is the positive
// half: the row count and the Peek detail read the stored list and nothing else.
func TestDependencyIsolationPresentationVocabulary(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate dependency retirement audit")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", ".."))
	allowedMigrationFiles := map[string]bool{
		"internal/config/v5_migration_activation.go":        true,
		"internal/storage/format/legacy_v4_task_convert.go": true,
		"internal/storage/format/v5_task_codec.go":          true,
		"internal/storage/format/v6_task_convert.go":        true,
		"internal/storage/legacy_v4_blocker.go":             true,
		"internal/storage/v5_archive_reintegration.go":      true,
	}
	// Soft task references are a live product concept again, so the TUI may name
	// dependencies; what stays retired is hard blocking. The inventory below is
	// therefore the fence for the retired panels, selectors, styles, and
	// operations, and the vocabulary sweep narrows to the blocker spelling alone.
	retiredIdentifiers := map[string]bool{
		"AddDependency":                true,
		"Block":                        true,
		"BlockedBadgeStyle":            true,
		"BlockedByDepsStyle":           true,
		"BlockedRowStyle":              true,
		"BlockedStyle":                 true,
		"ColDependencies":              true,
		"CloseCrossProjectLinkerMsg":   true,
		"CloseDependencyViewerMsg":     true,
		"CountBlockedTasks":            true,
		"ConvertLegacyV4Blockers":      true,
		"CrossProjectLinkCreatedMsg":   true,
		"CrossProjectLinkerModel":      true,
		"DependencySelectorDoneMsg":    true,
		"DependencySelectorModel":      true,
		"DependencyStyle":              true,
		"DependencyViewerModel":        true,
		"DependencyViewerSelectMsg":    true,
		"DependencyViewerViewDepsMsg":  true,
		"DecodeLegacyV4Blockers":       true,
		"DependsOn":                    true,
		"HeroBlocked":                  true,
		"LegacyV4Blocker":              true,
		"LinkTypeBlocker":              true,
		"LinkTypeDependent":            true,
		"ManageDependenciesMsg":        true,
		"NewCrossProjectLinkerModel":   true,
		"NewDependencySelectorModel":   true,
		"NewDependencyViewerModel":     true,
		"RemoveDependency":             true,
		"RetiredCapabilityDependency":  true,
		"SetDependencies":              true,
		"ShowCrossProjectLinkerMsg":    true,
		"ShowDependencyViewerMsg":      true,
		"ValueProviderDependencies":    true,
		"ViewCrossProjectLinker":       true,
		"ViewDependencySelector":       true,
		"ViewDependencyViewer":         true,
		"operationFilterBlocked":       true,
		"operationTaskSetDependencies": true,
		"workspaceFilterResolver":      true,
	}

	// The live half of the same contract. A reference now says four things and
	// only four: it is blocked, it is blocking, it is settled, or it is
	// unreadable. Asserting they are all present is what stops a later rename
	// from quietly emptying the vocabulary the retirement sweep guards.
	liveVocabulary := map[string]bool{
		"blocked":    false,
		"blocking":   false,
		"settled":    false,
		"unreadable": false,
	}
	liveVocabularyFiles := map[string]bool{}

	audit := &dependencyVocabularyAudit{
		fset:                  token.NewFileSet(),
		retiredIdentifiers:    retiredIdentifiers,
		allowedMigrationFiles: allowedMigrationFiles,
		liveVocabulary:        liveVocabulary,
		liveVocabularyFiles:   liveVocabularyFiles,
	}
	err := filepath.WalkDir(repositoryRoot, func(path string, entry fs.DirEntry, walkErr error) error {
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
		relativePath, relErr := filepath.Rel(repositoryRoot, path)
		if relErr != nil {
			return relErr
		}
		return audit.inspectFile(path, filepath.ToSlash(relativePath))
	})
	if err != nil {
		t.Fatalf("scan production Go sources: %v", err)
	}
	if len(audit.violations) > 0 {
		sort.Strings(audit.violations)
		t.Fatalf("retired dependency panels or blocking vocabulary returned outside migration code:\n%s", strings.Join(audit.violations, "\n"))
	}
	if len(liveVocabularyFiles) == 0 {
		t.Fatal("the sweep read no internal/tui production file")
	}
	missing := make([]string, 0, len(liveVocabulary))
	for word, seen := range liveVocabulary {
		if !seen {
			missing = append(missing, word)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("the reference vocabulary lost %s from internal/tui production code", strings.Join(missing, ", "))
	}
}

// dependencyVocabularyAudit carries the sweep's fences and its accumulated
// findings across files, so the test itself stays the contract it states and the
// per-file reading lives in one place.
type dependencyVocabularyAudit struct {
	fset                  *token.FileSet
	retiredIdentifiers    map[string]bool
	allowedMigrationFiles map[string]bool
	liveVocabulary        map[string]bool
	liveVocabularyFiles   map[string]bool
	violations            []string
}

// inspectFile reads one production Go file for both halves of the contract: the
// retired identifiers and the blocker spelling it may not contain, and the live
// words internal/tui must still be able to say.
func (audit *dependencyVocabularyAudit) inspectFile(path string, relativePath string) error {
	isTUIProduction := strings.HasPrefix(relativePath, "internal/tui/")
	if isTUIProduction && containsRetiredBlockingVocabulary(relativePath) {
		audit.violations = append(audit.violations, relativePath+": production filename")
	}
	if isTUIProduction {
		audit.liveVocabularyFiles[relativePath] = true
	}

	file, parseErr := parser.ParseFile(audit.fset, path, nil, parser.SkipObjectResolution)
	if parseErr != nil {
		return parseErr
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.Ident:
			audit.inspectIdentifier(node, relativePath, isTUIProduction)
		case *ast.BasicLit:
			audit.inspectLiteral(node, relativePath, isTUIProduction)
		}
		return true
	})
	return nil
}

func (audit *dependencyVocabularyAudit) inspectIdentifier(node *ast.Ident, relativePath string, isTUIProduction bool) {
	if audit.retiredIdentifiers[node.Name] && !audit.allowedMigrationFiles[relativePath] {
		audit.record(relativePath, node.Pos(), node.Name)
		return
	}
	if !isTUIProduction {
		return
	}
	if containsRetiredBlockingVocabulary(node.Name) {
		audit.record(relativePath, node.Pos(), node.Name)
	}
	noteLiveDependencyVocabulary(node.Name, audit.liveVocabulary)
}

func (audit *dependencyVocabularyAudit) inspectLiteral(node *ast.BasicLit, relativePath string, isTUIProduction bool) {
	if !isTUIProduction || node.Kind != token.STRING {
		return
	}
	value, unquoteErr := strconv.Unquote(node.Value)
	if unquoteErr != nil {
		value = node.Value
	}
	if containsRetiredBlockingVocabulary(value) || rendersBlockedBare(value) {
		audit.record(relativePath, node.Pos(), value)
	}
	noteLiveDependencyVocabulary(value, audit.liveVocabulary)
}

func (audit *dependencyVocabularyAudit) record(relativePath string, pos token.Pos, detail string) {
	position := audit.fset.Position(pos)
	audit.violations = append(audit.violations, relativePath+":"+strconv.Itoa(position.Line)+": "+detail)
}

// noteLiveDependencyVocabulary records that a word the reference surfaces are
// required to keep is still somewhere in production code.
func noteLiveDependencyVocabulary(value string, live map[string]bool) {
	lower := strings.ToLower(value)
	for word := range live {
		if strings.Contains(lower, word) {
			live[word] = true
		}
	}
}

// rendersBlockedBare rejects the one spelling of a live word that reads as a
// lifecycle state. Chrome and search say `?blocked`; a row and Peek say counts.
// A literal that is just the word is neither, so it is the spelling a reader
// would mistake for Waiting.
func rendersBlockedBare(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "blocked")
}

// containsRetiredBlockingVocabulary matches the word a hard dependency needed
// and a soft one cannot have. A "blocker" is a thing that stops work; nothing in
// the product does, so no filename, identifier, or string in the TUI may name
// one.
func containsRetiredBlockingVocabulary(value string) bool {
	return strings.Contains(strings.ToLower(value), "blocker")
}
