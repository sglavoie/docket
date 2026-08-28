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

func TestDependencyAndBlockedPresentationVocabularyIsRetired(t *testing.T) {
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
	// operations, and the vocabulary sweep narrows to blocking alone.
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

	fset := token.NewFileSet()
	var violations []string
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
		relativePath = filepath.ToSlash(relativePath)
		isTUIProduction := strings.HasPrefix(relativePath, "internal/tui/")
		if isTUIProduction && containsRetiredBlockingVocabulary(relativePath) {
			violations = append(violations, relativePath+": production filename")
		}

		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.Ident:
				if retiredIdentifiers[node.Name] && !allowedMigrationFiles[relativePath] {
					position := fset.Position(node.Pos())
					violations = append(violations, relativePath+":"+strconv.Itoa(position.Line)+": "+node.Name)
					return true
				}
				if isTUIProduction && containsRetiredBlockingVocabulary(node.Name) {
					position := fset.Position(node.Pos())
					violations = append(violations, relativePath+":"+strconv.Itoa(position.Line)+": "+node.Name)
				}
			case *ast.BasicLit:
				if !isTUIProduction || node.Kind != token.STRING {
					return true
				}
				value, unquoteErr := strconv.Unquote(node.Value)
				if unquoteErr != nil {
					value = node.Value
				}
				if containsRetiredBlockingVocabulary(value) {
					position := fset.Position(node.Pos())
					violations = append(violations, relativePath+":"+strconv.Itoa(position.Line)+": "+value)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan production Go sources: %v", err)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("retired dependency panels or blocking vocabulary returned outside migration code:\n%s", strings.Join(violations, "\n"))
	}
}

// containsRetiredBlockingVocabulary matches the words a hard dependency needed
// and a soft one cannot have: nothing in the product blocks on a reference, so
// no filename, identifier, or string in the TUI may say that it does.
func containsRetiredBlockingVocabulary(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "blocked") || strings.Contains(lower, "blocker")
}
