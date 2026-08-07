package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A raw `project.Archived` test is the one remaining way a surface can decide
// project availability without naming a population. Every such predicate is
// classified below by the question it actually answers:
//
//   - "availability" — the check admits a parked project on purpose. Archival is
//     the whole rule, and adding a parked-tag condition here would be a bug.
//   - "lifecycle" — the check is about the archived lifecycle itself: rendering
//     the archived badge, gating archive/restore, protecting Unfiled.
//
// A new or moved predicate must be classified here deliberately, so narrowing an
// availability check to the base task workspace can never happen by accident.
var archivePredicateAudit = map[string]string{
	"internal/app/notes.go:ProjectLocationPrecondition":                          "lifecycle: a note transaction refuses an owner archived since the snapshot",
	"internal/config/domain.go:validate":                                         "lifecycle: protected Unfiled can never be archived",
	"internal/config/domain.go:RepairSystemUnfiledProject":                       "lifecycle: protected Unfiled can never be archived",
	"internal/config/domain.go:upsertProject":                                    "lifecycle: protected Unfiled can never be archived",
	"internal/config/runtime.go:AvailableProjects":                               "availability: store-backed population admits every non-archived project",
	"internal/config/runtime_state.go:AvailableProjects":                         "availability: parked projects stay reachable",
	"internal/config/runtime_state.go:AvailableProject":                          "availability: single-project counterpart of AvailableProjects",
	"internal/config/runtime_state.go:TaskWorkspaceProjects":                     "availability: archival is checked first, then the parked tag narrows it",
	"internal/config/runtime_state.go:ArchivedProjects":                          "lifecycle: the archived population itself",
	"internal/tui/app_init.go:NewAppModel":                                       "lifecycle: a restored current project that was archived is dropped",
	"internal/tui/app_projects.go:activeProjectByID":                             "availability: shell cache lookup admits parked projects",
	"internal/tui/app_projects.go:ensureSystemUnfiledProject":                    "lifecycle: bootstrap repair un-archives protected Unfiled",
	"internal/tui/handler_project.go:handleProjectMessages":                      "lifecycle: selecting an archived project asks to restore it",
	"internal/tui/handler_project_bulk_tags.go:planProjectBulkTags":              "lifecycle: an archived project refuses a tag edit",
	"internal/tui/list_surface_adapters.go:ListSourceFromProjects":               "lifecycle: archived rows are bucketed into their own category",
	"internal/tui/notes_view.go:setOwnerProjects":                                "availability: parked owners keep their notes",
	"internal/tui/operation_target.go:entityContextNamesArchivedTask":            "lifecycle: an archived owner refuses every task-changing operation",
	"internal/tui/operation_value_provider.go:taskActionOperationApplicable":     "lifecycle: an archived owner admits read-only operations only",
	"internal/tui/operation_value_provider.go:moveProjectCandidatesForSourceIDs": "availability: parked projects stay move destinations",
	"internal/tui/project_discovery.go:projectPickerDisplayText":                 "lifecycle: archived rows render their lifecycle badge",
	"internal/tui/project_operations.go:projectRequirementUnmet":                 "lifecycle: descriptors require an archived or a live project",
	"internal/tui/project_selector.go:FilterText":                                "lifecycle: archived rows match the archived search term",
	"internal/tui/project_selector.go:NewProjectSelectorModel":                   "availability: parked projects stay move destinations",
	"internal/tui/quick_capture_input.go:initCaches":                             "availability: %project completes parked projects",
	"internal/tui/quick_capture_parser_resolution.go:resolveProject":             "availability: %project resolves parked projects",
	"internal/tui/task_action_sheet.go:taskActionDisabledReason":                 "lifecycle: an archived owner disables writing task actions",
	"internal/tui/task_composer.go:cycleProject":                                 "availability: the composer cycles onto a parked destination",
}

// archiveAuditPackages are the layers whose availability decisions this audit
// governs. Storage, models, and the maintenance CLI carry no population choice.
var archiveAuditPackages = []string{"internal/app", "internal/config", "internal/tui"}

// archiveAuditMigrationPrefixes name the migration-only decoders and dry runs.
// They reason about retired schemas rather than about live availability, so
// their archive checks are outside this classification by design.
var archiveAuditMigrationPrefixes = []string{"legacy_v4", "v5_", "v6_", "v7_", "pre_consolidation", "pre_scope", "pre_simplification"}

func TestRawArchivePredicatesAreClassified(t *testing.T) {
	root := repositoryRootForInventory(t)
	observed := make(map[string]bool)
	fset := token.NewFileSet()

	for _, pkg := range archiveAuditPackages {
		err := filepath.WalkDir(filepath.Join(root, pkg), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if isArchiveAuditMigrationFile(filepath.Base(path)) {
				return nil
			}
			relativePath, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if parseErr != nil {
				return parseErr
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				key := filepath.ToSlash(relativePath) + ":" + fn.Name.Name
				for range archivePredicateReads(fn.Body) {
					observed[key] = true
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", pkg, err)
		}
	}

	var unclassified, stale []string
	for key := range observed {
		if _, ok := archivePredicateAudit[key]; !ok {
			unclassified = append(unclassified, key)
		}
	}
	for key := range archivePredicateAudit {
		if !observed[key] {
			stale = append(stale, key)
		}
	}
	if len(unclassified) > 0 {
		sort.Strings(unclassified)
		t.Errorf("raw Archived predicate is not classified as availability or lifecycle:\n%s",
			strings.Join(unclassified, "\n"))
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("classified Archived predicate no longer exists:\n%s", strings.Join(stale, "\n"))
	}
}

func isArchiveAuditMigrationFile(name string) bool {
	for _, prefix := range archiveAuditMigrationPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// archivePredicateReads returns every `X.Archived` selector the body evaluates
// as a condition. Writes and struct-literal copies are excluded: they propagate
// the flag rather than decide anything with it.
func archivePredicateReads(body *ast.BlockStmt) []*ast.SelectorExpr {
	copied := make(map[ast.Expr]bool)
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.AssignStmt:
			for _, lhs := range typed.Lhs {
				copied[lhs] = true
			}
			for _, rhs := range typed.Rhs {
				// A bare `dst.Archived = src.Archived` propagates the flag; only a
				// larger expression around it is deciding something.
				if _, ok := rhs.(*ast.SelectorExpr); ok {
					copied[rhs] = true
				}
			}
		case *ast.KeyValueExpr:
			copied[typed.Value] = true
		}
		return true
	})

	var reads []*ast.SelectorExpr
	ast.Inspect(body, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Archived" || copied[ast.Expr(selector)] {
			return true
		}
		reads = append(reads, selector)
		return true
	})
	return reads
}
