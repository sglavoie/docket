package storage

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// This audit enforces the session-28 boundary: Cobra command implementations
// must route user-visible task/project mutations through the application layer
// (internal/app via newAppService), not through the legacy cmd.SaveProject
// helper or raw storage mutators. Unlike TestRawProjectSaveCallsitesAreAllowlisted
// (which only flags cmd.SaveProject calls that omit UndoAction), this audit flags
// EVERY invocation of the legacy save helper and every direct storage save inside
// cmd/, so newly-added commands cannot silently adopt the legacy persistence path.
//
// The allowlist has two tiers:
//   - infra: the helper's own definition and privileged repair/restore/adapter
//     paths that legitimately touch storage below the app layer.
//   - legacy: command families catalogued for app-layer migration. These are the
//     known stragglers the migration will retire (legacy persistence is removed
//     in session 33); the entry documents each one so no new caller hides among
//     them and the list shrinks as commands migrate.
const (
	cmdSaveInfra  = "infra: helper definition or privileged repair/restore/adapter path below the app layer"
	cmdSaveLegacy = "legacy command straggler pending app-layer migration; do NOT add new callers here"
)

var cmdLegacySaveAllowlist = map[string]string{
	// Infrastructure: the legacy helper itself and privileged non-command paths.
	"cmd/project_ops.go:SaveProject:storage.SaveProject(project)":                      cmdSaveInfra,
	"cmd/project_ops.go:processDeferredMovesWithUndo:storage.SaveProject(destProject)": cmdSaveInfra,
	"cmd/app_service.go:(*cliRepository).Commit:storage.SaveProject(cs.Project)":       cmdSaveInfra,
	"cmd/app_service.go:(*cliRepository).RawSaveProject:storage.SaveProject(project)":  cmdSaveInfra,
	"cmd/doctor.go:fixAutoRepaired:storage.SaveProjectLocked(project)":                 cmdSaveInfra,
	"cmd/doctor.go:fixIssues:storage.SaveProjectLocked(project)":                       cmdSaveInfra,
	"cmd/doctor.go:fixMissingDescriptionMarkers:storage.SaveProjectLocked(project)":    cmdSaveInfra,
	"cmd/project_delete.go:cleanCrossProjectRefs:storage.SaveProjectLocked(p)":         cmdSaveInfra,
	"cmd/project_edit.go:runProjectEdit:storage.SaveProjectLocked(project)":            cmdSaveInfra,
	"cmd/project_edit.go:updateCrossProjectRefs:storage.SaveProjectLocked(p)":          cmdSaveInfra,
	"cmd/undo.go:reapplySideEffectsAndSave:storage.SaveProject(project)":               cmdSaveInfra,

	// The cmdSaveLegacy tier is empty: session 33.3 migrated the last legacy
	// stragglers (tag pin/unpin, score set, template use, comment edit, the
	// WithTaskUpdate helper family, sandbox reset) onto internal/app. Do not add
	// new cmdSaveLegacy entries — new commands must persist through newAppService.
}

func TestCmdCommandsDoNotUseLegacyProjectSave(t *testing.T) {
	root := findRepoRootForUndoAudit(t)
	cmdDir := filepath.Join(root, "cmd")
	calls := collectCmdLegacySaveCalls(t, root, cmdDir)

	actual := make(map[string]struct{}, len(calls))
	var unexpected []string
	for _, key := range calls {
		actual[key] = struct{}{}
		if _, ok := cmdLegacySaveAllowlist[key]; !ok {
			unexpected = append(unexpected, key)
		}
	}

	var stale []string
	for key := range cmdLegacySaveAllowlist {
		if _, ok := actual[key]; !ok {
			stale = append(stale, key)
		}
	}

	if len(unexpected) > 0 || len(stale) > 0 {
		sort.Strings(unexpected)
		sort.Strings(stale)
		var b strings.Builder
		if len(unexpected) > 0 {
			b.WriteString("Cobra commands must persist user mutations through the application layer " +
				"(newAppService), not the legacy cmd.SaveProject helper or raw storage saves. " +
				"Migrate these callsites, or add a justified allowlist entry if they are infra/repair paths:\n")
			for _, key := range unexpected {
				fmt.Fprintf(&b, "\t%q: cmdSaveLegacy, // or cmdSaveInfra\n", key)
			}
		}
		if len(stale) > 0 {
			b.WriteString("stale cmd legacy-save allowlist entries (callsite migrated or removed — delete these):\n")
			for _, key := range stale {
				fmt.Fprintf(&b, "\t%s\n", key)
			}
		}
		t.Fatal(b.String())
	}
}

func TestCmdLegacySaveAllowlistReasonsAreExplicit(t *testing.T) {
	for key, reason := range cmdLegacySaveAllowlist {
		if strings.TrimSpace(reason) == "" {
			t.Fatalf("%s has an empty cmd legacy-save allowlist reason", key)
		}
	}
}

// collectCmdLegacySaveCalls returns keys for every call in cmd/ (non-test) that
// invokes the legacy cmd.SaveProject helper (bare Ident) or a raw storage save
// (storage.SaveProject / storage.SaveProjectLocked).
func collectCmdLegacySaveCalls(t *testing.T, root, dir string) []string {
	t.Helper()

	var keys []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		keys = append(keys, cmdLegacySaveCallsFromFile(t, root, path)...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk cmd dir %s: %v", dir, err)
	}
	sort.Strings(keys)
	return keys
}

func cmdLegacySaveCallsFromFile(t *testing.T, root, path string) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	relPath := slashPath(t, root, path)
	var keys []string
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
			if !isCmdLegacySaveCall(fset, call) {
				return true
			}
			keys = append(keys, fmt.Sprintf("%s:%s:%s", relPath, fnName, compactNode(fset, call)))
			return true
		})
	}
	return keys
}

func isCmdLegacySaveCall(fset *token.FileSet, call *ast.CallExpr) bool {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		// Legacy cmd.SaveProject helper invoked with (project, opts...).
		return fun.Name == "SaveProject" && len(call.Args) >= 1
	case *ast.SelectorExpr:
		if compactNode(fset, fun.X) != "storage" {
			return false
		}
		return fun.Sel.Name == "SaveProject" || fun.Sel.Name == "SaveProjectLocked"
	}
	return false
}
