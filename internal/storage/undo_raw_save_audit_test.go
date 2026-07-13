package storage

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const (
	rawSaveCLIHelper     = "CLI save helper implementation; undo-aware callers use SaveOptions metadata"
	rawSaveCLIMigration  = "legacy CLI mutation path explicitly allowed while undo coverage is audited; new callsites must use undo metadata"
	rawSaveRepair        = "repair/side-effect save path outside direct user mutation; excluded from user undo history"
	rawSaveTUIHelper     = "Persister.Save implementation backing the undo/redo restore reapply path (reapplySideEffects); user mutations commit through the application-layer transaction boundary"
	rawSaveTUIRecorded   = "sandbox-reset maintenance path: records an undo snapshot before a whole-project clear/reset, not a per-task user-mutation persistence tier"
	rawSaveRestoreResult = "undo/redo restore result save; not a new user mutation"
	rawSaveReviewFixture = "tui-test review-harness fixture save into an isolated throwaway temp workspace; never user-visible data, so undo tracking is out of scope"
	rawSaveAppTestRepo   = "app.Repository Commit fallback used only for non-filesystem test doubles; the production filesystem path commits through the undo-aware transaction journal + workspace ledger, so this raw save never runs against real user data"
)

var rawProjectSaveAllowlist = map[string]string{
	"cmd/app_service.go:(*cliRepository).Commit:storage.SaveProject(cs.Project)":                         rawSaveAppTestRepo,
	"cmd/app_service.go:(*cliRepository).RawSaveProject:storage.SaveProject(project)":                    rawSaveRepair,
	"cmd/doctor.go:fixAutoRepaired:storage.SaveProjectLocked(project)":                                   rawSaveRepair,
	"cmd/doctor.go:fixIssues:storage.SaveProjectLocked(project)":                                         rawSaveRepair,
	"cmd/doctor.go:fixMissingDescriptionMarkers:storage.SaveProjectLocked(project)":                      rawSaveRepair,
	"cmd/project_delete.go:cleanCrossProjectRefs:storage.SaveProjectLocked(p)":                           rawSaveRepair,
	"cmd/project_edit.go:runProjectEdit:storage.SaveProjectLocked(project)":                              rawSaveCLIMigration,
	"cmd/project_edit.go:updateCrossProjectRefs:storage.SaveProjectLocked(p)":                            rawSaveRepair,
	"cmd/project_ops.go:SaveProject:storage.SaveProject(project)":                                        rawSaveCLIHelper,
	"cmd/project_ops.go:processDeferredMovesWithUndo:storage.SaveProject(destProject)":                   rawSaveCLIMigration,
	"cmd/undo.go:reapplySideEffectsAndSave:storage.SaveProject(project)":                                 rawSaveRestoreResult,
	"internal/tui/app_run.go:RunApp:storage.SaveProjectLocked(current)":                                  rawSaveRepair,
	"internal/tui/app_run.go:evaluateWorkflowAtStartup:storage.SaveProjectLocked(project)":               rawSaveRepair,
	"internal/tui/feature_runtime.go:(*tuiAppRepository).Commit:storage.SaveProject(cs.Project)":         rawSaveAppTestRepo,
	"internal/tui/handler_project.go:(*AppModel).executeSandboxReset:storage.SaveProjectLocked(sandbox)": rawSaveTUIRecorded,
	"internal/tui/project_service.go:(*ProjectService).Save:storage.SaveProjectLocked(project)":          rawSaveTUIHelper,
	"internal/tui/shared_task_ops_persist.go:reapplySideEffects:persister.Save(project)":                 rawSaveRestoreResult,
	"internal/tui/uitest_fixtures.go:(*uitestWorkspace).addProject:storage.SaveProject(project)":         rawSaveReviewFixture,
}

func TestRawProjectSaveCallsitesAreAllowlisted(t *testing.T) {
	root := findRepoRootForUndoAudit(t)
	calls := collectRawProjectSaveCalls(t, root, []string{
		filepath.Join(root, "cmd"),
		filepath.Join(root, "internal", "tui"),
	})

	actual := make(map[string]rawProjectSaveCall, len(calls))
	var unexpected []rawProjectSaveCall
	for _, call := range calls {
		actual[call.Key] = call
		if _, ok := rawProjectSaveAllowlist[call.Key]; !ok {
			unexpected = append(unexpected, call)
		}
	}

	var stale []string
	for key := range rawProjectSaveAllowlist {
		if _, ok := actual[key]; !ok {
			stale = append(stale, key)
		}
	}

	if len(unexpected) > 0 || len(stale) > 0 {
		sort.Slice(unexpected, func(i, j int) bool { return unexpected[i].Key < unexpected[j].Key })
		sort.Strings(stale)
		var b strings.Builder
		if len(unexpected) > 0 {
			b.WriteString("unexpected raw project-save callsites; either route through undo-aware save APIs or add a justified allowlist entry:\n")
			for _, call := range unexpected {
				fmt.Fprintf(&b, "\t%q: \"TODO: explain why this raw save may bypass undo for now\",\n", call.Key)
			}
		}
		if len(stale) > 0 {
			b.WriteString("stale raw project-save allowlist entries:\n")
			for _, key := range stale {
				fmt.Fprintf(&b, "\t%s\n", key)
			}
		}
		t.Fatal(b.String())
	}
}

func TestRawProjectSaveAllowlistReasonsAreExplicit(t *testing.T) {
	for key, reason := range rawProjectSaveAllowlist {
		trimmed := strings.TrimSpace(reason)
		if trimmed == "" {
			t.Fatalf("%s has an empty raw-save allowlist reason", key)
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "todo") || strings.Contains(lower, "tbd") || strings.Contains(lower, "not yet") {
			t.Fatalf("%s has a placeholder raw-save allowlist reason: %q", key, reason)
		}
	}
}

type rawProjectSaveCall struct {
	Key string
}

func collectRawProjectSaveCalls(t *testing.T, root string, dirs []string) []rawProjectSaveCall {
	t.Helper()

	var calls []rawProjectSaveCall
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("stat audit dir %s: %v", dir, err)
		}
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			calls = append(calls, collectRawProjectSaveCallsFromFile(t, root, path)...)
			return nil
		})
		if err != nil {
			t.Fatalf("walk audit dir %s: %v", dir, err)
		}
	}
	sort.Slice(calls, func(i, j int) bool { return calls[i].Key < calls[j].Key })
	return calls
}

func collectRawProjectSaveCallsFromFile(t *testing.T, root, path string) []rawProjectSaveCall {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	relPath := slashPath(t, root, path)
	var calls []rawProjectSaveCall
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		fnName := auditFuncName(fset, fn)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if rawProjectSaveKind(fset, relPath, call) == "" {
				return true
			}
			calls = append(calls, rawProjectSaveCall{
				Key: fmt.Sprintf("%s:%s:%s", relPath, fnName, compactNode(fset, call)),
			})
			return true
		})
	}
	return calls
}

func rawProjectSaveKind(fset *token.FileSet, relPath string, call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return rawProjectSaveIdentKind(relPath, fun.Name, call)
	case *ast.SelectorExpr:
		receiver := compactNode(fset, fun.X)
		return rawProjectSaveSelectorKind(receiver, fun.Sel.Name)
	default:
		return ""
	}
}

func rawProjectSaveIdentKind(relPath, name string, call *ast.CallExpr) string {
	if strings.HasPrefix(relPath, "cmd/") && name == "SaveProject" && !saveProjectCallHasUndoAction(call) {
		return "cmd-save-without-undo-action"
	}
	// The TUI user-mutation save helpers (SaveProjectAsync, SaveMultipleProjectsAsync,
	// saveProjectSync, saveProjectNoReload, saveProjectCore) were deleted once every
	// mutation moved behind the application-layer transaction boundary, so there is
	// no longer a bare-ident save helper to detect in internal/tui.
	return ""
}

func rawProjectSaveSelectorKind(receiver, name string) string {
	if receiver == "storage" {
		switch name {
		case "SaveProject", "SaveProjectLocked":
			return "direct-storage-save"
		}
	}
	// The Persister SaveWithBackup*/SaveMultiple* tiers were deleted; only the raw
	// Persister.Save (restore reapply path) remains detectable on a persister-typed
	// receiver.
	if name == "Save" {
		switch receiver {
		case "m.projectService", "m.persister", "opts.Persister", "p", "persister", "ps", "svc":
			return "persister-save-without-undo-options"
		}
	}
	return ""
}

func saveProjectCallHasUndoAction(call *ast.CallExpr) bool {
	if len(call.Args) < 2 {
		return false
	}

	opts := call.Args[1]
	if unary, ok := opts.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		opts = unary.X
	}
	lit, ok := opts.(*ast.CompositeLit)
	if !ok {
		return false
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if ok && key.Name == "UndoAction" {
			return true
		}
	}
	return false
}

func auditFuncName(fset *token.FileSet, fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return fmt.Sprintf("(%s).%s", compactNode(fset, fn.Recv.List[0].Type), fn.Name.Name)
}

func compactNode(fset *token.FileSet, node any) string {
	var b bytes.Buffer
	if err := printer.Fprint(&b, fset, node); err != nil {
		return "<unprintable>"
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func slashPath(t *testing.T, root, path string) string {
	t.Helper()

	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("rel path for %s: %v", path, err)
	}
	return filepath.ToSlash(rel)
}

func findRepoRootForUndoAudit(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root containing go.mod")
		}
		dir = parent
	}
}
