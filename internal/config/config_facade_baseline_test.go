package config

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

// configFacadeBaseline is a committed, machine-generated inventory of how much
// production code (cmd/ and internal/tui/) still couples to the config.Config
// facade. It records two per-file metrics the remaining migration children must
// drive toward zero:
//
//   - TypeSignatures: references to the config.Config type (function params,
//     returns, struct fields, conversions) — every `config.Config` selector in
//     source, which is always a type position.
//   - FieldDerefs: field/method-value dereferences on a config-typed value named
//     `cfg` (or `<recv>.cfg`), i.e. `cfg.Field` / `m.cfg.Field` accesses that are
//     NOT method calls. Method calls on cfg are counted separately as
//     MethodCalls; both together measure facade coupling.
//
// Like the storage mutation-callsite baseline, this file is regenerated from the
// AST so it can never silently drift, and each change surfaces in review as a
// diff. Regenerate it with:
//
//	UPDATE_CONFIG_FACADE_BASELINE=1 go test ./internal/config/ -run TestConfigFacadeBaseline
const configFacadeBaseline = "testdata/config_facade_baseline.txt"

// configFacadeScanDirs are the production trees whose config.Config coupling is
// tracked. Test files (_test.go) are excluded.
var configFacadeScanDirs = []string{
	filepath.Join("cmd"),
	filepath.Join("internal", "tui"),
}

type facadeFileStat struct {
	relpath        string
	typeSignatures int
	fieldDerefs    int
	methodCalls    int
}

func TestConfigFacadeBaseline(t *testing.T) {
	root := findRepoRootForConfigBaseline(t)

	stats := map[string]*facadeFileStat{}
	stat := func(rel string) *facadeFileStat {
		s, ok := stats[rel]
		if !ok {
			s = &facadeFileStat{relpath: rel}
			stats[rel] = s
		}
		return s
	}

	for _, dir := range configFacadeScanDirs {
		abs := filepath.Join(root, dir)
		err := filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			ts, fd, mc := scanConfigFacadeFile(t, path)
			if ts == 0 && fd == 0 && mc == 0 {
				return nil
			}
			s := stat(rel)
			s.typeSignatures += ts
			s.fieldDerefs += fd
			s.methodCalls += mc
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", abs, err)
		}
	}

	rows := make([]*facadeFileStat, 0, len(stats))
	for _, s := range stats {
		rows = append(rows, s)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].relpath < rows[j].relpath })

	got := renderConfigFacadeBaseline(rows)

	if os.Getenv("UPDATE_CONFIG_FACADE_BASELINE") == "1" {
		if err := os.WriteFile(configFacadeBaseline, []byte(got), 0o644); err != nil {
			t.Fatalf("write config facade baseline: %v", err)
		}
		t.Logf("regenerated %s with %d files", configFacadeBaseline, len(rows))
		return
	}

	wantBytes, err := os.ReadFile(configFacadeBaseline)
	if err != nil {
		t.Fatalf("read config facade baseline (regenerate with UPDATE_CONFIG_FACADE_BASELINE=1): %v", err)
	}
	if string(wantBytes) != got {
		t.Fatalf("config.Config facade-usage inventory drifted from %s.\n"+
			"Move callers behind the domain-backed runtime seam (config.RuntimeDependencies), or regenerate with:\n"+
			"  UPDATE_CONFIG_FACADE_BASELINE=1 go test ./internal/config/ -run TestConfigFacadeBaseline",
			configFacadeBaseline)
	}
}

// scanConfigFacadeFile parses one production file and returns its (config.Config
// type signatures, cfg field dereferences, cfg method calls) counts.
func scanConfigFacadeFile(t *testing.T, path string) (typeSigs, fieldDerefs, methodCalls int) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	// Selectors that are the Fun of a call are method calls, not field derefs.
	callFuns := map[*ast.SelectorExpr]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				callFuns[sel] = true
			}
		}
		return true
	})

	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		// config.Config type reference: X == ident "config", Sel == "Config".
		if x, ok := sel.X.(*ast.Ident); ok && x.Name == "config" && sel.Sel.Name == "Config" {
			typeSigs++
			return true
		}
		// Facade access on a config-typed value named cfg or <recv>.cfg, with an
		// exported member (fields/exported methods on config.Config).
		if !isConfigValueExpr(sel.X) || !sel.Sel.IsExported() {
			return true
		}
		if callFuns[sel] {
			methodCalls++
		} else {
			fieldDerefs++
		}
		return true
	})

	return typeSigs, fieldDerefs, methodCalls
}

// isConfigValueExpr reports whether an expression is, by naming convention, the
// loaded config value: the identifier `cfg` or a selector ending in `.cfg`
// (e.g. `m.cfg`). This is the syntactic proxy the baseline uses for a
// config.Config-typed receiver in cmd/ and internal/tui/.
func isConfigValueExpr(x ast.Expr) bool {
	switch v := x.(type) {
	case *ast.Ident:
		return v.Name == "cfg"
	case *ast.SelectorExpr:
		return v.Sel.Name == "cfg"
	default:
		return false
	}
}

func renderConfigFacadeBaseline(rows []*facadeFileStat) string {
	var totalTypes, totalFields, totalMethods int
	for _, s := range rows {
		totalTypes += s.typeSignatures
		totalFields += s.fieldDerefs
		totalMethods += s.methodCalls
	}

	var b strings.Builder
	b.WriteString("# Generated config.Config facade-usage baseline — production coupling in cmd/ and internal/tui/.\n")
	b.WriteString("# Do not edit by hand. Regenerate with:\n")
	b.WriteString("#   UPDATE_CONFIG_FACADE_BASELINE=1 go test ./internal/config/ -run TestConfigFacadeBaseline\n")
	b.WriteString("# Columns: <relpath>\\t<config.Config type signatures>\\t<cfg field derefs>\\t<cfg method calls>\n")
	fmt.Fprintf(&b, "# Totals: type_signatures=%d field_derefs=%d method_calls=%d files=%d\n",
		totalTypes, totalFields, totalMethods, len(rows))
	for _, s := range rows {
		fmt.Fprintf(&b, "%s\t%d\t%d\t%d\n", s.relpath, s.typeSignatures, s.fieldDerefs, s.methodCalls)
	}
	return b.String()
}

func findRepoRootForConfigBaseline(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root containing go.mod")
		}
		dir = parent
	}
}
