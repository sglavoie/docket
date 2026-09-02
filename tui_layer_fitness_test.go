package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	// tuiPackageGraphPath is the intended graph: one row per target package,
	// naming every sibling it may import.
	tuiPackageGraphPath = "scripts/tui_package_graph.tsv"

	// tuiPackageAssignmentsPath is the Session 1.5 table this graph must agree
	// with about which layer each package sits in.
	tuiPackageAssignmentsPath = "scripts/tui_package_assignments.tsv"

	// tuiImportPrefix is the import path every package under the split shares.
	tuiImportPrefix = "github.com/sglavoie/tt/internal/tui"

	// tuiRootPackage is the row describing internal/tui itself.
	tuiRootPackage = "tui"
)

// tuiPackageFloor is the number of real packages under internal/tui today: the
// shell, keymap, and presentation. The split only adds packages, so finding
// fewer means the walk stopped reaching them and the fitness check has gone
// vacuous rather than clean.
const tuiPackageFloor = 3

// tuiPackageNode is one row of the intended graph.
type tuiPackageNode struct {
	name  string
	layer int
	deps  map[string]bool
}

// tuiImportEdge is one real import of one TUI package by another, kept with the
// file that spells it so a failure names something editable.
type tuiImportEdge struct {
	file string
	from string
	to   string
}

// TestTUIPackageGraphTableIsWellFormed audits the table itself. A fitness test
// is only as good as the graph it reads, and a graph naming a package that does
// not exist, or contradicting the assignment table about a layer, would let a
// violation through while still going through the motions.
func TestTUIPackageGraphTableIsWellFormed(t *testing.T) {
	graph := readTUIPackageGraph(t)

	if _, ok := graph[tuiRootPackage]; !ok {
		t.Fatalf("%s does not describe the root shell package %q", tuiPackageGraphPath, tuiRootPackage)
	}

	for _, node := range sortedNodes(graph) {
		for _, dep := range sortedNames(node.deps) {
			if dep == node.name {
				t.Errorf("%s: %s depends on itself", tuiPackageGraphPath, node.name)
				continue
			}
			target, ok := graph[dep]
			if !ok {
				t.Errorf("%s: %s depends on unknown package %q", tuiPackageGraphPath, node.name, dep)
				continue
			}
			if target.layer > node.layer {
				t.Errorf("%s: %s (L%d) depends on %s (L%d), which is upward",
					tuiPackageGraphPath, node.name, node.layer, dep, target.layer)
			}
		}
	}

	if cycle := findGraphCycle(graph); cycle != nil {
		t.Errorf("%s declares a cycle: %s", tuiPackageGraphPath, strings.Join(cycle, " -> "))
	}

	for name, layer := range assignedPackageLayers(t) {
		node, ok := graph[name]
		if !ok {
			t.Errorf("%s assigns files to package %q, which %s does not describe",
				tuiPackageAssignmentsPath, name, tuiPackageGraphPath)
			continue
		}
		if node.layer != layer {
			t.Errorf("%s puts %s at L%d but %s puts it at L%d",
				tuiPackageAssignmentsPath, name, layer, tuiPackageGraphPath, node.layer)
		}
	}
}

// TestTUIPackageImportsFollowTheIntendedGraph is the layer-fitness test. Go's
// own package-boundary check only catches a cycle; a sideways or upward import
// that happens to be acyclic compiles cleanly and quietly erodes the layering
// this refactor exists to establish. This reads the intended graph from the
// committed table and fails on any import it does not allow.
//
// Test files are held to the same rule as production files. A sub-package test
// that reaches up into the shell is the same coupling as a production one; the
// approved constraint is that a genuinely cross-cutting test becomes a root
// integration test instead.
func TestTUIPackageImportsFollowTheIntendedGraph(t *testing.T) {
	graph := readTUIPackageGraph(t)
	root := repositoryRootDir(t)
	edges, packagesWalked := readTUIImportEdges(t, root)

	if packagesWalked < tuiPackageFloor {
		t.Fatalf("walked %d packages under %s, want at least %d; the fitness check is not reaching them",
			packagesWalked, tuiSubtree, tuiPackageFloor)
	}
	if len(edges) == 0 {
		t.Fatalf("found no import between packages under %s; the fitness check would pass vacuously", tuiSubtree)
	}

	for _, edge := range edges {
		node, ok := graph[edge.from]
		if !ok {
			t.Errorf("%s: package %q is not described in %s", edge.file, edge.from, tuiPackageGraphPath)
			continue
		}
		if _, ok := graph[edge.to]; !ok {
			t.Errorf("%s: imports package %q, which is not described in %s", edge.file, edge.to, tuiPackageGraphPath)
			continue
		}
		if !node.deps[edge.to] {
			t.Errorf("%s: %s (L%d) imports %s (L%d), which %s does not allow; move the code down or add the edge deliberately",
				edge.file, edge.from, node.layer, edge.to, graph[edge.to].layer, tuiPackageGraphPath)
		}
	}
}

func readTUIPackageGraph(t *testing.T) map[string]tuiPackageNode {
	t.Helper()

	graph := map[string]tuiPackageNode{}
	for number, line := range tableRows(t, tuiPackageGraphPath) {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			t.Fatalf("%s:%d: want package, layer and depends_on columns, got %d", tuiPackageGraphPath, number, len(fields))
		}
		if fields[0] == "package" {
			continue
		}
		name := fields[0]
		if _, duplicate := graph[name]; duplicate {
			t.Fatalf("%s:%d: package %q is described twice", tuiPackageGraphPath, number, name)
		}
		deps := map[string]bool{}
		if fields[2] != "-" {
			for _, dep := range strings.Split(fields[2], ",") {
				deps[strings.TrimSpace(dep)] = true
			}
		}
		graph[name] = tuiPackageNode{name: name, layer: parseLayer(t, tuiPackageGraphPath, number, fields[1]), deps: deps}
	}
	if len(graph) == 0 {
		t.Fatalf("%s describes no package", tuiPackageGraphPath)
	}
	return graph
}

// assignedPackageLayers reduces the assignment table to the layer it records
// for each package, failing when one package's files disagree with each other.
func assignedPackageLayers(t *testing.T) map[string]int {
	t.Helper()

	layers := map[string]int{}
	for number, line := range tableRows(t, tuiPackageAssignmentsPath) {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 || fields[0] == "path" {
			continue
		}
		name := fields[1]
		layer := parseLayer(t, tuiPackageAssignmentsPath, number, fields[2])
		if recorded, ok := layers[name]; ok && recorded != layer {
			t.Fatalf("%s:%d: package %q is recorded at both L%d and L%d",
				tuiPackageAssignmentsPath, number, name, recorded, layer)
		}
		layers[name] = layer
	}
	if len(layers) == 0 {
		t.Fatalf("%s assigns no file to a package", tuiPackageAssignmentsPath)
	}
	return layers
}

// tableRows returns the data lines of one committed table, keyed by their line
// number so a diagnostic can point at the row.
func tableRows(t *testing.T, relative string) map[int]string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(repositoryRootDir(t), filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	rows := map[int]string{}
	for index, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rows[index+1] = line
	}
	return rows
}

func parseLayer(t *testing.T, relative string, number int, field string) int {
	t.Helper()

	value, err := strconv.Atoi(strings.TrimPrefix(field, "L"))
	if err != nil {
		t.Fatalf("%s:%d: %q is not an L<n> layer", relative, number, field)
	}
	return value
}

// readTUIImportEdges parses every Go file under internal/tui, test files
// included, and returns the imports that stay inside the subtree.
func readTUIImportEdges(t *testing.T, root string) ([]tuiImportEdge, int) {
	t.Helper()

	subtree := filepath.Join(root, filepath.FromSlash(tuiSubtree))
	seen := map[string]bool{}
	var edges []tuiImportEdge
	fileSet := token.NewFileSet()

	err := filepath.WalkDir(subtree, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if skipsPackageDir(entry.Name()) && path != subtree {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		owner, err := tuiPackageForDir(subtree, filepath.Dir(path))
		if err != nil {
			return err
		}
		seen[owner] = true

		parsed, parseErr := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, spec := range parsed.Imports {
			imported, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				return unquoteErr
			}
			target, inside := tuiPackageForImport(imported)
			if !inside || target == owner {
				continue
			}
			edges = append(edges, tuiImportEdge{file: subtreePath(t, root, path), from: owner, to: target})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", tuiSubtree, err)
	}

	slices.SortFunc(edges, func(left, right tuiImportEdge) int {
		if left.file != right.file {
			return strings.Compare(left.file, right.file)
		}
		return strings.Compare(left.to, right.to)
	})
	return edges, len(seen)
}

// tuiPackageForDir names the package owning one directory. The subtree root is
// the shell; anything else is a single directory below it, which is the shape
// the target graph describes.
func tuiPackageForDir(subtree, dir string) (string, error) {
	if dir == subtree {
		return tuiRootPackage, nil
	}
	relative, err := filepath.Rel(subtree, dir)
	if err != nil {
		return "", err
	}
	slashed := filepath.ToSlash(relative)
	if strings.Contains(slashed, "/") {
		return "", fmt.Errorf("%s is nested deeper than one package below %s", slashed, tuiSubtree)
	}
	return slashed, nil
}

// tuiPackageForImport maps an import path to the TUI package it names, and
// reports false for every path outside the subtree.
func tuiPackageForImport(path string) (string, bool) {
	if path == tuiImportPrefix {
		return tuiRootPackage, true
	}
	suffix, inside := strings.CutPrefix(path, tuiImportPrefix+"/")
	if !inside {
		return "", false
	}
	return suffix, true
}

// findGraphCycle returns one cycle in the declared graph, or nil. The layer
// check already rejects an upward edge, but same-layer edges are legal and are
// exactly where L7's surface ordering could double back.
func findGraphCycle(graph map[string]tuiPackageNode) []string {
	const (
		unvisited = 0
		active    = 1
		done      = 2
	)
	state := map[string]int{}
	var path []string
	var walk func(name string) []string
	walk = func(name string) []string {
		switch state[name] {
		case active:
			return append(path[slices.Index(path, name):], name)
		case done:
			return nil
		}
		state[name] = active
		path = append(path, name)
		for _, dep := range sortedNames(graph[name].deps) {
			if _, ok := graph[dep]; !ok {
				continue
			}
			if cycle := walk(dep); cycle != nil {
				return cycle
			}
		}
		path = path[:len(path)-1]
		state[name] = done
		return nil
	}
	for _, node := range sortedNodes(graph) {
		if cycle := walk(node.name); cycle != nil {
			return cycle
		}
	}
	return nil
}

func sortedNodes(graph map[string]tuiPackageNode) []tuiPackageNode {
	nodes := make([]tuiPackageNode, 0, len(graph))
	for _, node := range graph {
		nodes = append(nodes, node)
	}
	slices.SortFunc(nodes, func(left, right tuiPackageNode) int {
		if left.layer != right.layer {
			return left.layer - right.layer
		}
		return strings.Compare(left.name, right.name)
	})
	return nodes
}

func sortedNames(set map[string]bool) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
