package compat

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// writePathInventory is the committed map from every production function that
// performs a durable filesystem mutation to the sync integration point that
// owns it. It is checked against the source in both directions: a new write
// site with no row fails, and so does a row whose write site is gone.
//
//	UPDATE_WRITE_PATH_INVENTORY=1 go test ./internal/compat/ -run TestCompatWritePathInventoryCoversEveryDurableWrite
//
// regenerates the symbol column, keeping existing classifications and marking
// new rows UNASSIGNED, which the test then refuses until a person classifies
// them.
const (
	writePathInventory       = "testdata/write_paths.tsv"
	writePathInventoryEnv    = "UPDATE_WRITE_PATH_INVENTORY"
	writePathUnassigned      = "UNASSIGNED"
	writePathIntegrationDoc  = "../../docs/mobile/write-path-inventory.md"
	writePathInventoryHeader = "# symbol\tintegration\tnote"
)

// writePathIntegrations are the integration classes the inventory may assign.
// Each one is defined in docs/mobile/write-path-inventory.md under a heading of
// the same name.
var writePathIntegrations = []string{
	"commit-journal",
	"commit-staging",
	"domain-store",
	"ledger-restore",
	"workspace-relocation",
	"workspace-backup",
	"sync-enrollment",
	"sync-transport",
	"conflict-store",
	"external-editor",
	"export",
	"machine-local",
}

// fileMutators are the filesystem primitives that create, replace, or remove
// durable bytes or directories. util.AtomicWriteFile is listed because its
// callers, not its body, decide what is written.
var fileMutators = map[string]map[string]bool{
	"os": {
		"WriteFile": true, "Rename": true, "Remove": true, "RemoveAll": true,
		"Create": true, "CreateTemp": true, "OpenFile": true, "Mkdir": true,
		"MkdirAll": true, "MkdirTemp": true, "Symlink": true, "Link": true,
		"Chmod": true, "Chtimes": true, "Truncate": true,
	},
	"util": {"AtomicWriteFile": true},
}

// inventoryRoots are the production trees scanned. scripts/ holds developer
// tooling that never opens a workspace, and internal/testutil is test support.
var (
	inventoryRoots         = []string{"main.go", "cmd", "internal"}
	inventoryExcludedTrees = []string{"internal/testutil", "internal/deadcodeaudit"}
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repository root not found from %s", root)
	}
	return root
}

// inventorySource is one parsed production file and the local names it gives
// the packages whose functions mutate the filesystem.
type inventorySource struct {
	relative string
	file     *ast.File
	imports  map[string]string
}

// collectWriteSites returns symbol -> primitive references, keyed by
// "path:Func", "path:Recv.Method", or "path:var name"; a function literal
// belongs to the named declaration that encloses it. A package-level variable
// bound to a mutator is an alias, and every function in that package naming it
// is a write site too, recorded as "alias:name".
func collectWriteSites(t *testing.T, root string) map[string][]string {
	t.Helper()
	sources := parseInventorySources(t, root)
	sites := map[string][]string{}
	packageAliases := recordDirectWriteSites(sources, sites)
	recordAliasWriteSites(sources, packageAliases, sites)
	for symbol := range sites {
		sort.Strings(sites[symbol])
		sites[symbol] = compactStrings(sites[symbol])
	}
	return sites
}

func skipInventoryDir(relative, name string) bool {
	if name == "testdata" {
		return true
	}
	return slices.Contains(inventoryExcludedTrees, relative)
}

func parseInventorySources(t *testing.T, root string) []inventorySource {
	t.Helper()
	var sources []inventorySource
	fileSet := token.NewFileSet()
	for _, entry := range inventoryRoots {
		walkErr := filepath.WalkDir(filepath.Join(root, entry), func(path string, item fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if item.IsDir() {
				if skipInventoryDir(relative, item.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fileSet, path, nil, 0)
			if err != nil {
				return err
			}
			sources = append(sources, inventorySource{relative: relative, file: file, imports: mutatorImports(file)})
			return nil
		})
		if walkErr != nil {
			t.Fatal(walkErr)
		}
	}
	return sources
}

func mutatorImports(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, spec := range file.Imports {
		importPath := strings.Trim(spec.Path.Value, `"`)
		local := filepath.Base(importPath)
		if spec.Name != nil {
			local = spec.Name.Name
		}
		switch importPath {
		case "os":
			imports[local] = "os"
		case "github.com/sglavoie/tt/internal/util":
			imports[local] = "util"
		}
	}
	return imports
}

// directMutatorReferences lists the mutators node names. A reference counts
// whether it is called or bound: the journal stores os.Rename and
// util.AtomicWriteFile as function values, and a seam such as
// `var openFile = os.OpenFile` writes through whoever calls it.
func directMutatorReferences(source inventorySource, node ast.Node) []string {
	var found []string
	ast.Inspect(node, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		pkg := source.imports[ident.Name]
		if pkg == "" || !fileMutators[pkg][selector.Sel.Name] {
			return true
		}
		if pkg == "util" && source.relative == "internal/util/files.go" {
			return true
		}
		found = append(found, pkg+"."+selector.Sel.Name)
		return true
	})
	return found
}

// recordDirectWriteSites records every declaration naming a mutator and
// returns, per package directory, the package variables bound to one.
func recordDirectWriteSites(sources []inventorySource, sites map[string][]string) map[string]map[string]bool {
	packageAliases := map[string]map[string]bool{}
	for _, source := range sources {
		dir := filepath.Dir(source.relative)
		for _, declaration := range source.file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Body == nil {
					continue
				}
				if found := directMutatorReferences(source, declaration.Body); len(found) > 0 {
					symbol := source.relative + ":" + functionName(declaration)
					sites[symbol] = append(sites[symbol], found...)
				}
			case *ast.GenDecl:
				for _, name := range recordVariableWriteSites(source, declaration, sites) {
					if packageAliases[dir] == nil {
						packageAliases[dir] = map[string]bool{}
					}
					packageAliases[dir][name] = true
				}
			}
		}
	}
	return packageAliases
}

func recordVariableWriteSites(source inventorySource, declaration *ast.GenDecl, sites map[string][]string) []string {
	var aliases []string
	for _, spec := range declaration.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for index, name := range value.Names {
			if index >= len(value.Values) {
				continue
			}
			found := directMutatorReferences(source, value.Values[index])
			if len(found) == 0 {
				continue
			}
			symbol := source.relative + ":var " + name.Name
			sites[symbol] = append(sites[symbol], found...)
			aliases = append(aliases, name.Name)
		}
	}
	return aliases
}

// recordAliasWriteSites records every function naming a package variable that
// is bound to a mutator in its own package.
func recordAliasWriteSites(sources []inventorySource, packageAliases map[string]map[string]bool, sites map[string][]string) {
	for _, source := range sources {
		aliases := packageAliases[filepath.Dir(source.relative)]
		if len(aliases) == 0 {
			continue
		}
		for _, declaration := range source.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			symbol := source.relative + ":" + functionName(function)
			ast.Inspect(function.Body, func(node ast.Node) bool {
				if selector, ok := node.(*ast.SelectorExpr); ok {
					// Only the receiver half of a selector can name a package
					// variable; skip the field or method half.
					ast.Inspect(selector.X, func(inner ast.Node) bool {
						if ident, ok := inner.(*ast.Ident); ok && aliases[ident.Name] {
							sites[symbol] = append(sites[symbol], "alias:"+ident.Name)
						}
						return true
					})
					return false
				}
				if ident, ok := node.(*ast.Ident); ok && aliases[ident.Name] {
					sites[symbol] = append(sites[symbol], "alias:"+ident.Name)
				}
				return true
			})
		}
	}
}

func compactStrings(values []string) []string {
	out := values[:0]
	for i, value := range values {
		if i == 0 || value != values[i-1] {
			out = append(out, value)
		}
	}
	return out
}

func functionName(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return function.Name.Name
	}
	receiver := function.Recv.List[0].Type
	if star, ok := receiver.(*ast.StarExpr); ok {
		receiver = star.X
	}
	if index, ok := receiver.(*ast.IndexExpr); ok {
		receiver = index.X
	}
	if ident, ok := receiver.(*ast.Ident); ok {
		return ident.Name + "." + function.Name.Name
	}
	return function.Name.Name
}

type writePathRow struct {
	symbol      string
	integration string
	note        string
}

func readWritePathInventory(t *testing.T) []writePathRow {
	t.Helper()
	data, err := os.ReadFile(writePathInventory)
	if err != nil {
		t.Fatalf("read %s (regenerate with %s=1): %v", writePathInventory, writePathInventoryEnv, err)
	}
	var rows []writePathRow
	for index, text := range strings.Split(string(data), "\n") {
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Split(text, "\t")
		if len(fields) != 3 {
			t.Fatalf("%s:%d: want 3 tab-separated fields, got %d", writePathInventory, index+1, len(fields))
		}
		rows = append(rows, writePathRow{symbol: fields[0], integration: fields[1], note: fields[2]})
	}
	return rows
}

func writeWritePathInventory(t *testing.T, sites map[string][]string, existing []writePathRow) {
	t.Helper()
	known := map[string]writePathRow{}
	for _, row := range existing {
		known[row.symbol] = row
	}
	symbols := make([]string, 0, len(sites))
	for symbol := range sites {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	var b strings.Builder
	b.WriteString("# Durable write-path inventory: every production function that mutates the filesystem,\n")
	b.WriteString("# and the sync integration point that owns it. Classes are defined in\n")
	b.WriteString("# docs/mobile/write-path-inventory.md. Regenerate symbols with\n")
	fmt.Fprintf(&b, "#   %s=1 go test ./internal/compat/ -run TestCompatWritePathInventoryCoversEveryDurableWrite\n", writePathInventoryEnv)
	b.WriteString(writePathInventoryHeader + "\n")
	for _, symbol := range symbols {
		row, ok := known[symbol]
		if !ok {
			row = writePathRow{symbol: symbol, integration: writePathUnassigned, note: strings.Join(sites[symbol], " ")}
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\n", row.symbol, row.integration, row.note)
	}
	if err := os.WriteFile(writePathInventory, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCompatWritePathInventoryCoversEveryDurableWrite proves every durable
// write the production code can perform has an identified sync integration
// point, and that every class the inventory uses is defined in the contract.
func TestCompatWritePathInventoryCoversEveryDurableWrite(t *testing.T) {
	root := repositoryRoot(t)
	sites := collectWriteSites(t, root)
	if len(sites) == 0 {
		t.Fatal("scanner found no write sites; the scan itself is broken")
	}
	if os.Getenv(writePathInventoryEnv) == "1" {
		var existing []writePathRow
		if _, err := os.Stat(writePathInventory); err == nil {
			existing = readWritePathInventory(t)
		}
		writeWritePathInventory(t, sites, existing)
		t.Logf("regenerated %s with %d symbols", writePathInventory, len(sites))
		return
	}

	rows := readWritePathInventory(t)
	allowed := map[string]bool{}
	for _, class := range writePathIntegrations {
		allowed[class] = true
	}
	seen := map[string]bool{}
	used := map[string]int{}
	for _, row := range rows {
		if seen[row.symbol] {
			t.Errorf("%s is listed twice", row.symbol)
		}
		seen[row.symbol] = true
		if _, ok := sites[row.symbol]; !ok {
			t.Errorf("%s no longer writes durable bytes; remove its row", row.symbol)
		}
		if !allowed[row.integration] {
			t.Errorf("%s has integration %q, want one of %v", row.symbol, row.integration, writePathIntegrations)
		}
		if strings.TrimSpace(row.note) == "" {
			t.Errorf("%s has no note", row.symbol)
		}
		used[row.integration]++
	}
	for symbol, calls := range sites {
		if !seen[symbol] {
			t.Errorf("%s writes durable bytes (%s) with no sync integration point in %s", symbol, strings.Join(calls, ", "), writePathInventory)
		}
	}

	doc, err := os.ReadFile(writePathIntegrationDoc)
	if err != nil {
		t.Fatal(err)
	}
	for _, class := range writePathIntegrations {
		heading := regexp.MustCompile(`(?m)^### ` + "`" + regexp.QuoteMeta(class) + "`" + `\s*$`)
		if !heading.Match(doc) {
			t.Errorf("integration class %q has no \"### `%s`\" section in %s", class, class, writePathIntegrationDoc)
		}
		if used[class] == 0 {
			t.Errorf("integration class %q is defined but no write path uses it", class)
		}
	}
	t.Logf("%d write paths across %d integration classes", len(rows), len(used))
}

// commitEntryPoints are the exported mutation entry points whose callers reach
// the commit journal. Each must name its integration in the contract document,
// so a new public commit surface cannot appear without being placed.
var commitEntryPointPattern = regexp.MustCompile(`^func \((?:s|l|c|m) \*(CommitService|WorkspaceLedger|DomainCoordinator|Manager)\) ((?:Commit|Undo|Redo|Migrate|Adopt|Recover|Restore)[A-Za-z0-9]*)\(`)

// TestCompatCommitEntryPointsAreDocumented scans the storage and config
// packages for exported commit, restore, and migration entry points and
// requires the write-path document to name each one.
func TestCompatCommitEntryPointsAreDocumented(t *testing.T) {
	root := repositoryRoot(t)
	doc, err := os.ReadFile(writePathIntegrationDoc)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, dir := range []string{"internal/storage", "internal/config"} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			source, err := os.ReadFile(filepath.Join(root, dir, name))
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range strings.Split(string(source), "\n") {
				match := commitEntryPointPattern.FindStringSubmatch(line)
				if match == nil {
					continue
				}
				found++
				symbol := match[1] + "." + match[2]
				if !strings.Contains(string(doc), "`"+symbol+"`") {
					t.Errorf("%s/%s: entry point %s is not placed in %s", dir, name, symbol, writePathIntegrationDoc)
				}
			}
		}
	}
	if found == 0 {
		t.Fatal("no commit entry points matched; the scan pattern is stale")
	}
	t.Logf("%d commit entry points documented", found)
}
