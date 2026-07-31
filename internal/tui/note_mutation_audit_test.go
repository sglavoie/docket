package tui

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

// These storage helpers are intentionally read-only or pure codecs. Listing
// them explicitly makes a new note-related storage call fail closed until its
// mutation semantics are reviewed.
var allowedNoteStorageReads = map[string]string{
	"GlobalNotesDir":   "derives the canonical global collection path without changing it",
	"NoteFilePathIn":   "validates and derives a path for read-only editor/reference input",
	"NotesDir":         "derives a project note collection path without changing it",
	"NotesExist":       "checks whether a project has notes",
	"ParseNoteContent": "parses temporary editor bytes without writing canonical storage",
	"RenderNoteContent": "renders a copied note for temporary editor input without writing " +
		"canonical storage",
}

func TestNoteMutationAuditRejectsDirectProductionWrites(t *testing.T) {
	root := findNoteAuditRepoRoot(t)
	census := collectProductionNoteStorageCalls(t, root)

	var unexpected []string
	seenReads := make(map[string]bool)
	for _, call := range census {
		switch call.Kind {
		case "mutation":
			unexpected = append(unexpected, call.Key)
		case "read":
			if _, ok := allowedNoteStorageReads[call.Name]; !ok {
				unexpected = append(unexpected, call.Key+" (unclassified note storage call)")
			} else {
				seenReads[call.Name] = true
			}
		case "directory-removal":
			unexpected = append(unexpected, call.Key)
		}
	}

	var stale []string
	for name := range allowedNoteStorageReads {
		if !seenReads[name] {
			stale = append(stale, "read classification: "+name)
		}
	}
	if len(unexpected) != 0 || len(stale) != 0 {
		sort.Strings(unexpected)
		sort.Strings(stale)
		t.Fatalf("note mutation boundary drifted; unexpected=%v stale=%v", unexpected, stale)
	}
}

func TestNoteMutationAuditReasonsAreExplicit(t *testing.T) {
	for key, reason := range allowedNoteStorageReads {
		assertNoteAuditReason(t, key, reason)
	}
}

func assertNoteAuditReason(t *testing.T, key, reason string) {
	t.Helper()
	trimmed := strings.TrimSpace(reason)
	lower := strings.ToLower(trimmed)
	if trimmed == "" || strings.Contains(lower, "todo") || strings.Contains(lower, "tbd") || strings.Contains(lower, "not yet") {
		t.Fatalf("%s has a placeholder note-audit reason: %q", key, reason)
	}
}

type productionNoteStorageCall struct {
	Key  string
	Kind string
	Name string
}

func collectProductionNoteStorageCalls(t *testing.T, root string) []productionNoteStorageCall {
	t.Helper()
	var calls []productionNoteStorageCall
	for _, dir := range []string{filepath.Join(root, "cmd"), filepath.Join(root, "internal", "tui")} {
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			calls = append(calls, collectProductionNoteStorageCallsFromFile(t, root, path)...)
			return nil
		})
		if err != nil {
			t.Fatalf("walk note audit directory %s: %v", dir, err)
		}
	}
	sort.Slice(calls, func(i, j int) bool { return calls[i].Key < calls[j].Key })
	return calls
}

func collectProductionNoteStorageCallsFromFile(t *testing.T, root, path string) []productionNoteStorageCall {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("relative audit path %s: %v", path, err)
	}
	rel = filepath.ToSlash(rel)
	storageNames, osNames := noteAuditImportNames(file)
	var calls []productionNoteStorageCall
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		fnName := noteAuditFuncName(fset, fn)
		noteDirs := noteDirectoryVariables(fn.Body, storageNames)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			receiver, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			key := fmt.Sprintf("%s:%s:%s", rel, fnName, noteAuditNode(fset, call))
			if storageNames[receiver.Name] && isDirectNoteMutation(selector.Sel.Name) {
				calls = append(calls, productionNoteStorageCall{Key: key, Kind: "mutation", Name: selector.Sel.Name})
				return true
			}
			if storageNames[receiver.Name] && strings.Contains(selector.Sel.Name, "Note") {
				calls = append(calls, productionNoteStorageCall{Key: key, Kind: "read", Name: selector.Sel.Name})
				return true
			}
			if osNames[receiver.Name] && (selector.Sel.Name == "Remove" || selector.Sel.Name == "RemoveAll") && removesKnownNoteDirectory(call, noteDirs, storageNames) {
				calls = append(calls, productionNoteStorageCall{Key: key, Kind: "directory-removal", Name: selector.Sel.Name})
			}
			return true
		})
	}
	return calls
}

func noteAuditImportNames(file *ast.File) (map[string]bool, map[string]bool) {
	storageNames := make(map[string]bool)
	osNames := make(map[string]bool)
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, "\"")
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if strings.HasSuffix(path, "/internal/storage") {
			storageNames[name] = true
		}
		if path == "os" {
			osNames[name] = true
		}
	}
	return storageNames, osNames
}

func isDirectNoteMutation(name string) bool {
	return strings.HasPrefix(name, "SaveNote") || strings.HasPrefix(name, "DeleteNote") || strings.HasPrefix(name, "RenameNote")
}

func noteDirectoryVariables(body *ast.BlockStmt, storageNames map[string]bool) map[string]bool {
	vars := make(map[string]bool)
	changed := true
	for changed {
		changed = false
		ast.Inspect(body, func(node ast.Node) bool {
			assign, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, lhs := range assign.Lhs {
				if i >= len(assign.Rhs) {
					continue
				}
				ident, ok := lhs.(*ast.Ident)
				if !ok || vars[ident.Name] {
					continue
				}
				if noteDirectoryExpr(assign.Rhs[i], vars, storageNames) {
					vars[ident.Name] = true
					changed = true
				}
			}
			return true
		})
	}
	return vars
}

func noteDirectoryExpr(expr ast.Expr, vars, storageNames map[string]bool) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		return vars[ident.Name]
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	return ok && storageNames[receiver.Name] && (selector.Sel.Name == "NotesDir" || selector.Sel.Name == "GlobalNotesDir")
}

func removesKnownNoteDirectory(call *ast.CallExpr, vars, storageNames map[string]bool) bool {
	return len(call.Args) > 0 && noteDirectoryExpr(call.Args[0], vars, storageNames)
}

func noteAuditFuncName(fset *token.FileSet, fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return fmt.Sprintf("(%s).%s", noteAuditNode(fset, fn.Recv.List[0].Type), fn.Name.Name)
}

func noteAuditNode(fset *token.FileSet, node any) string {
	var b bytes.Buffer
	if err := printer.Fprint(&b, fset, node); err != nil {
		return "<unprintable>"
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func findNoteAuditRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repository root for note mutation audit")
		}
		dir = parent
	}
}
