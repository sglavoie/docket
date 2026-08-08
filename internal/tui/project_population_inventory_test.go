package tui

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

// projectPopulationInventory classifies every production caller of a runtime
// project population by the file that reads it.
//
//   - AvailableProjects (and the AppModel availableProjects wrapper) is every
//     non-archived project regardless of its tags. A surface that must keep
//     admitting a parked project — Notes owners, capture and %project
//     resolution, task-move destinations, project administration — reads it.
//   - TaskWorkspaceProjects (and the AppModel taskWorkspaceProjects wrapper) is
//     the base task workspace: non-archived and not parked. Every task
//     projection and context scope reads it, which is what leaves a parked
//     project no route into the Outliner.
//   - ActiveProjects was the undifferentiated population that predates the
//     split. It is retired: no caller may resolve project availability by a name
//     that does not say which of the two populations it means.
//
// A caller added, moved, or retired must be classified here deliberately.
var projectPopulationInventory = map[string]string{
	"internal/tui/app_init.go:taskWorkspaceProjects":                 "startup Today/Next hydration is a task projection",
	"internal/tui/app_manage_hub.go:availableProjects":               "Notes owner refresh after a workspace lifecycle change",
	"internal/tui/app_pinned_tasks_data.go:taskWorkspaceProjects":    "pinned task collection is a task projection",
	"internal/tui/app_projects.go:AvailableProjects":                 "availableProjects wrapper over the parked-inclusive population",
	"internal/tui/app_projects.go:TaskWorkspaceProjects":             "taskWorkspaceProjects wrapper over the base task workspace",
	"internal/tui/app_task_composer.go:availableProjects":            "composer project completion and %project resolution",
	"internal/tui/core_input_operations.go:availableProjects":        "quick capture destinations",
	"internal/tui/cross_project_outliner.go:AvailableProjects":       "scope chrome counts the parked projects the task projection is hiding",
	"internal/tui/cross_scope.go:AvailableProjects":                  "unscoped Outliner selection is a task projection",
	"internal/tui/cross_scope.go:TaskWorkspaceProjects":              "scoped Outliner selection is a task projection",
	"internal/tui/feature_runtime.go:AvailableProjects":              "mutation snapshots must resolve every non-archived project",
	"internal/tui/filter_operations.go:taskWorkspaceProjects":        "tag filter facet is derived from a task projection",
	"internal/tui/filter_project_operations.go:availableProjects":    "the project tag vocabulary must name parked projects so a facet can widen back out",
	"internal/tui/project_tag_vocabulary.go:availableProjects":       "a tag's editable membership includes its parked projects",
	"internal/tui/handler_helpers.go:availableProjects":              "planning prune must not drop references owned by a parked project",
	"internal/tui/handler_view_notes.go:availableProjects":           "Notes owner refresh when reopening the manager",
	"internal/tui/manage_hub.go:AvailableProjects":                   "Notes owners and project-note command scope",
	"internal/tui/workspace_pulse.go:TaskWorkspaceProjects":          "workspace pulse counting is a task projection",
	"internal/tui/operation_target_context.go:taskWorkspaceProjects": "operation command scope IDs follow the context scope",
	"internal/tui/operation_value_provider.go:availableProjects":     "single and bulk task-move destinations",
	"internal/tui/outliner_scope.go:TaskWorkspaceProjects":           "a focus needs the parked exception only while the task projection excludes it",
	"internal/tui/project_hub.go:AvailableProjects":                  "project administration rows and project-level search",
}

// retiredProjectPopulationNames are accessor names collapsed into the named
// populations. They must not reappear anywhere in the repository, including as
// a field or helper name, because the ambiguity this plan removed was one of
// vocabulary as much as behavior.
var retiredProjectPopulationNames = map[string]bool{
	"AllLoadableProjects":   true,
	"allLoadableProjects":   true,
	"ActiveProjects":        true,
	"activeProjects":        true,
	"IsActiveContextMember": true,
}

func TestProjectPopulationCallersAreClassified(t *testing.T) {
	tracked := map[string]bool{
		"AvailableProjects":     true,
		"TaskWorkspaceProjects": true,
		"availableProjects":     true,
		"taskWorkspaceProjects": true,
	}

	observed := make(map[string]bool)
	var retired []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(repositoryRootForInventory(t), func(path string, entry fs.DirEntry, walkErr error) error {
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
		relativePath, relErr := filepath.Rel(repositoryRootForInventory(t), path)
		if relErr != nil {
			return relErr
		}
		relativePath = filepath.ToSlash(relativePath)

		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if ident, ok := node.(*ast.Ident); ok && retiredProjectPopulationNames[ident.Name] {
				retired = append(retired, relativePath+":"+ident.Name)
				return true
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !tracked[selector.Sel.Name] {
				return true
			}
			observed[relativePath+":"+selector.Sel.Name] = true
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan production Go sources: %v", err)
	}

	if len(retired) > 0 {
		sort.Strings(retired)
		t.Errorf("retired project population accessor is still referenced:\n%s", strings.Join(retired, "\n"))
	}

	var unclassified []string
	for caller := range observed {
		if _, ok := projectPopulationInventory[caller]; !ok {
			unclassified = append(unclassified, caller)
		}
	}
	if len(unclassified) > 0 {
		sort.Strings(unclassified)
		t.Errorf("project population caller is not classified in the inventory:\n%s", strings.Join(unclassified, "\n"))
	}

	var stale []string
	for caller := range projectPopulationInventory {
		if !observed[caller] {
			stale = append(stale, caller)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("inventory entry no longer has a production caller:\n%s", strings.Join(stale, "\n"))
	}
}

func repositoryRootForInventory(t *testing.T) string {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate project population inventory")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", ".."))
}
