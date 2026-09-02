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
// must not route user-visible task/project mutations through raw storage
// mutators. The legacy cmd.SaveProject helper is gone; this audit still treats any future bare
// SaveProject call as a forbidden reintroduction and flags every direct storage
// save inside cmd/.
//
// Every surviving entry is an explicit privileged repair/restore path or a
// production implementation branch reachable only with a test repository.
const cmdSaveInfra = "infra: privileged repair/restore or test-repository adapter path below the app layer"

var cmdLegacySaveAllowlist = map[string]string{
	"cmd/doctor_tasks.go:fixIssues:storage.SaveProjectLocked(project)": cmdSaveInfra,
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
			b.WriteString("Cobra commands must not use a reintroduced cmd.SaveProject helper or raw storage saves. " +
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
// invokes a forbidden cmd.SaveProject helper (bare Ident) or a raw storage save
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
		// A bare cmd.SaveProject helper would reintroduce the retired save tier.
		return fun.Name == "SaveProject" && len(call.Args) >= 1
	case *ast.SelectorExpr:
		if compactNode(fset, fun.X) != "storage" {
			return false
		}
		return fun.Sel.Name == "SaveProject" || fun.Sel.Name == "SaveProjectLocked"
	}
	return false
}
