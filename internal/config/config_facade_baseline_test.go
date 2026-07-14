package config

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestConfigFacadeCannotBeReintroduced is the permanent architecture boundary
// after the split-domain cutover. It rejects the aggregate type, its constructor
// and adapters, and the monolithic Manager persistence methods across production
// and test code. Unlike the transitional census, there is no allowlist or
// regeneratable baseline that can bless a regression.
func TestConfigFacadeCannotBeReintroduced(t *testing.T) {
	root := findRepositoryRoot(t)
	forbiddenFunctions := map[string]bool{
		"NewConfig":                 true,
		"Decompose":                 true,
		"NewRuntimeStateFromConfig": true,
	}
	forbiddenManagerMethods := map[string]bool{
		"Load":                  true,
		"Save":                  true,
		"LoadOrCreate":          true,
		"SyncedDomainArtifacts": true,
	}
	var violations []string

	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			inConfigPackage := file.Name.Name == "config" && strings.HasPrefix(rel, "internal/config/")

			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.GenDecl:
					if !inConfigPackage || d.Tok != token.TYPE {
						continue
					}
					for _, spec := range d.Specs {
						if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == "Config" {
							violations = append(violations, rel+": declares forbidden type Config")
						}
					}
				case *ast.FuncDecl:
					if !inConfigPackage {
						continue
					}
					if d.Recv == nil && forbiddenFunctions[d.Name.Name] {
						violations = append(violations, rel+": declares forbidden adapter "+d.Name.Name)
					}
					if receiverName(d.Recv) == "Domains" && d.Name.Name == "ApplyTo" {
						violations = append(violations, rel+": declares forbidden Domains.ApplyTo adapter")
					}
					if receiverName(d.Recv) == "Manager" && forbiddenManagerMethods[d.Name.Name] {
						violations = append(violations, rel+": declares forbidden monolithic Manager."+d.Name.Name)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", dir, err)
		}
	}

	if _, err := os.Stat(filepath.Join(root, "internal", "config", "config.go")); !os.IsNotExist(err) {
		violations = append(violations, "internal/config/config.go: retired facade definition file exists")
	}
	if len(violations) != 0 {
		sort.Strings(violations)
		t.Fatalf("split-domain architecture regressed:\n%s", strings.Join(violations, "\n"))
	}
}

func receiverName(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	typ := fields.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	if ident, ok := typ.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

func findRepositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}
