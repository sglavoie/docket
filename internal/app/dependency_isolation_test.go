package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// TestDependencyIsolationBackendInventory is the negative half of the soft
// reference contract. Soft references are a live product concept, so the fence
// no longer sweeps the word "dependency"; what it still forbids is every
// identifier a *hard* dependency needed — blocking, cycle and DAG validation,
// completion refusal, transitive walks, ranking, and the old workspace-wide
// reference pruners that silently rewrote a user's set.
//
// The positive half lives in TestDependencyIsolationLeavesLifecycleUntouched
// below and in internal/app/dependencies_test.go: together they say a reference
// is stored, presented, and searched, and does nothing else.
func TestDependencyIsolationBackendInventory(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate dependency retirement audit")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", ".."))
	allowedMigrationFiles := map[string]bool{
		"internal/storage/format/legacy_v4_task_convert.go": true,
		"internal/storage/format/v5_task_codec.go":          true,
		"internal/storage/format/v6_task_convert.go":        true,
		"internal/storage/v5_archive_reintegration.go":      true,
	}
	forbiddenIdentifiers := map[string]bool{
		"AddDependency":                  true,
		"AddDependencyByLocator":         true,
		"ClearDependencies":              true,
		"ClearDependenciesByLocator":     true,
		"ClearedDeps":                    true,
		"CrossProjectRefsCleared":        true,
		"DependsOn":                      true,
		"ErrDependencyBlocked":           true,
		"GetBlockedByTasks":              true,
		"GetBlockingTasks":               true,
		"GetIncompleteDependencies":      true,
		"HasCircularDependency":          true,
		"HasDependency":                  true,
		"HasIncompleteDependencies":      true,
		"IsDependencyBlocked":            true,
		"NewWorkspaceResolver":           true,
		"RemoveDependency":               true,
		"RemoveDependencyByLocator":      true,
		"ReverseDependencies":            true,
		"SetDependencies":                true,
		"SetDependenciesByLocator":       true,
		"WouldCreateDependencyCycle":     true,
		"WorkspaceResolver":              true,
		"addDependency":                  true,
		"addDependencySelection":         true,
		"breakDependencyCycles":          true,
		"checkCrossRefs":                 true,
		"checkDeadDependencies":          true,
		"checkOrphanedCrossRefs":         true,
		"cleanRefs":                      true,
		"clearCrossProjectRefs":          true,
		"clearTaskDependencies":          true,
		"containsTaskReference":          true,
		"deadDependencies":               true,
		"deduplicateTaskReferences":      true,
		"dependsOnTransitive":            true,
		"filterCrossRefs":                true,
		"filterOutRefs":                  true,
		"globalTaskKey":                  true,
		"isValidCrossRef":                true,
		"orphanedCrossProjectReferences": true,
		"orphanedCrossRefs":              true,
		"projectForQualifier":            true,
		"pruneAllOrphanedRefs":           true,
		"pruneOrphanedRefs":              true,
		"referenceIdentity":              true,
		"referencesTask":                 true,
		"removeDependency":               true,
		"removeDependencySelection":      true,
		"removeEquivalentTaskReferences": true,
		"removeTaskReferences":           true,
		"rewriteMovedTaskRefs":           true,
		"setDependencies":                true,
		"taskReferenceIdentity":          true,
		"validateDependencySet":          true,
	}
	scanRoots := []string{
		"cmd",
		"internal/app",
		"internal/config",
		"internal/models",
		"internal/storage",
		"internal/workspace",
	}

	fset := token.NewFileSet()
	var violations []string
	for _, scanRoot := range scanRoots {
		root := filepath.Join(repositoryRoot, filepath.FromSlash(scanRoot))
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			relativePath, relErr := filepath.Rel(repositoryRoot, path)
			if relErr != nil {
				return relErr
			}
			relativePath = filepath.ToSlash(relativePath)
			if allowedMigrationFiles[relativePath] {
				return nil
			}

			file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if parseErr != nil {
				return parseErr
			}
			ast.Inspect(file, func(node ast.Node) bool {
				identifier, ok := node.(*ast.Ident)
				if !ok || !forbiddenIdentifiers[identifier.Name] {
					return true
				}
				position := fset.Position(identifier.Pos())
				violations = append(violations, relativePath+":"+position.String()+": "+identifier.Name)
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", scanRoot, err)
		}
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("dependency behavior returned outside migration code:\n%s", strings.Join(violations, "\n"))
	}
}
