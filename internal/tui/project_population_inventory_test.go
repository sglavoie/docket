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

	"github.com/sglavoie/tt/internal/testutil/gotree"
)

// projectPopulationInventory classifies every production caller of a runtime
// project population by the file that reads it.
//
//   - AvailableProjects (and the AppModel availableProjects wrapper) is every
//     non-archived project regardless of its tags. A surface that must keep
//     admitting a parked project — Notes owners, capture and %project
//     resolution, and project administration — reads it.
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
	"cmd/project_export.go:AvailableProjects":                                      "export dependency labels resolve against every non-archived project, and the archived ones are appended beside them because an exported document may name work that has since been archived",
	"internal/tui/lifecycle_controller.go:TaskWorkspaceProjects":                   "--notify reports arrivals in the base task workspace, the population whose rows show them",
	"internal/tui/app_init.go:taskWorkspaceProjects":                               "startup Today/Next hydration is a task projection",
	"internal/tui/app_manage_hub.go:availableProjects":                             "Notes owner refresh after a workspace lifecycle change",
	"internal/tui/app_projects.go:AvailableProjects":                               "availableProjects wrapper over the parked-inclusive population",
	"internal/tui/shell_ports.go:availableProjects":                                "the exported port spelling the clusters below the shell read the parked-inclusive population through",
	"internal/tui/shell_ports.go:taskWorkspaceProjects":                            "the exported port spelling the clusters below the shell read the base task workspace through",
	"internal/tui/app_projects.go:TaskWorkspaceProjects":                           "taskWorkspaceProjects wrapper over the base task workspace",
	"internal/tui/app_task_composer.go:availableProjects":                          "composer project completion and %project resolution",
	"internal/tui/core_input_handlers.go:availableProjects":                        "quick capture destinations",
	"internal/tui/outliner/cross_project_projection.go:AvailableProjects":          "scope chrome counts the parked projects the task projection is hiding",
	"internal/tui/outliner/cross_project_scope.go:AvailableProjects":               "unscoped selection and explicit project filters may admit parked projects",
	"internal/tui/outliner/cross_project_scope.go:TaskWorkspaceProjects":           "scoped Outliner selection and workspace pulse counting are both task projections",
	"internal/engine/repository.go:AvailableProjects":                              "mutation snapshots must resolve every non-archived project",
	"internal/bridge/ops.go:TaskWorkspaceProjects":                                 "query.projects reports the base task workspace as its active population",
	"internal/bridge/ops.go:AvailableProjects":                                     "query.projects reports the parked projects the task workspace excludes, apart from it",
	"internal/engine/reminders.go:TaskWorkspaceProjects":                           "reminders plan over the base task workspace, so parked and archived projects raise none",
	"internal/engine/query.go:AvailableProjects":                                   "a headless query loads every non-archived project, then narrows parked owners out unless a parked clause opens them",
	"internal/tui/filterops/filter_pickers.go:TaskWorkspaceProjects":               "tag filter facet is derived from a task projection",
	"internal/tui/filterops/filter_project_operations.go:AvailableProjects":        "the project tag vocabulary must name parked projects so a facet can widen back out",
	"internal/tui/projectops/project_tag_vocabulary.go:AvailableProjects":          "a tag's editable membership includes its parked projects",
	"internal/tui/handler_helpers.go:availableProjects":                            "planning prune must not drop references owned by a parked project",
	"internal/tui/handler_task_content.go:availableProjects":                       "external-editor %project resolution matches inline edit",
	"internal/tui/handler_view_notes.go:availableProjects":                         "Notes owner refresh when reopening the manager",
	"internal/tui/projecthub/manage_hub.go:AvailableProjects":                      "Notes owners and project-note command scope",
	"internal/tui/notesops/note_search_index.go:AvailableProjects":                 "the Outliner note search index covers the same owners the Notes manager does, so a parked project's notes stay searchable",
	"internal/tui/outline_editor.go:availableProjects":                             "outline ~task resolution matches inline edit, so a dependency on a parked project's task still round-trips",
	"internal/tui/operationhost/operation_target_context.go:TaskWorkspaceProjects": "operation command scope IDs follow the context scope",
	"internal/tui/operation_value_picker_factory.go:AvailableProjects":             "Move to Project destinations match %project, so a parked project stays reachable",
	"internal/tui/outliner/outliner_scope.go:TaskWorkspaceProjects":                "a focus needs the parked exception only while the task projection excludes it",
	"internal/tui/projectops/project_hub_shell.go:AvailableProjects":               "per-project task tallies must cover every project the Hub can list, parked ones included",
	"internal/tui/projecthub/project_hub_surface.go:AvailableProjects":             "project administration rows",
	"internal/tui/projecthub/project_hub_view.go:AvailableProjects":                "project-level search subtitle counts",
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
	root := repositoryRootForInventory(t)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (gotree.Ignored(entry.Name()) || entry.Name() == "vendor") {
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

// repositoryRootForInventory resolves the repository root from this file's own
// source path. It finds go.mod rather than counting two directories up, so the
// inventory's repo-relative keys stay correct at any package depth.
func repositoryRootForInventory(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate the project population inventory")
	}
	return tuiRepositoryRootFrom(t, thisFile)
}
