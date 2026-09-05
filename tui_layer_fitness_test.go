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

// The Phase 0 vacuity guard was a floor of three, the number of real packages
// that existed when Session 1.6 wrote it. Twenty-three exist now, so a floor of
// three guards nothing, and it never closed the hole that matters: a package
// spelling no sibling import appears in no edge and so was invisible to the
// whole check. TestTUIPackageGraphDescribesEveryPackageThatExists replaces it
// with an exact population match against the graph table, which is both a
// tighter vacuity guard and the thing that makes a new package declare its
// layer before it can declare an import.

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
	edges := readTUIImportEdges(t, root)

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

// TestTUIPackageGraphDescribesEveryPackageThatExists holds the table to the
// tree in both directions. TestTUIPackageImportsFollowTheIntendedGraph can only
// judge a package that appears in an edge, so a new package importing nothing
// under internal/tui, or imported by nothing there, slipped past the whole
// fitness check while it decided where its files lived. Requiring the table to
// name exactly the packages on disk is what makes declaring a layer the first
// step of adding a package rather than an afterthought, and it is the vacuity
// guard as well: a walk that stops reaching the subtree reports every described
// package missing instead of passing.
func TestTUIPackageGraphDescribesEveryPackageThatExists(t *testing.T) {
	graph := readTUIPackageGraph(t)
	onDisk := tuiPackageDirs(t, repositoryRootDir(t))

	for _, name := range sortedNames(onDisk) {
		if _, ok := graph[name]; !ok {
			t.Errorf("%s/%s exists but %s does not describe it; give it a layer and a depends_on row",
				tuiSubtree, name, tuiPackageGraphPath)
		}
	}
	for _, node := range sortedNodes(graph) {
		if node.name == tuiRootPackage {
			continue
		}
		if !onDisk[node.name] {
			t.Errorf("%s describes package %q, which holds no Go file under %s; delete the row",
				tuiPackageGraphPath, node.name, tuiSubtree)
		}
	}
}

// tuiPackageFileCeiling is the most production .go files any package under
// internal/tui may hold, the root shell included. The ceiling is a shape
// constraint rather than a style one: a package that needs a forty-first file
// is a package holding more than one concern, and the split exists so that
// concern has somewhere else to be.
const tuiPackageFileCeiling = 40

// TestNoTUIPackageExceedsTheFileCeiling holds every package under internal/tui
// to that ceiling, the root shell included and exempted from nothing. The root
// is the residue of the split rather than one of its extractions, which is
// exactly why it is the package that would drift back: an exemption here would
// leave the one package with no upper bound at all.
func TestNoTUIPackageExceedsTheFileCeiling(t *testing.T) {
	counts := tuiPackageFileCounts(t, repositoryRootDir(t))

	if _, measured := counts[tuiRootPackage]; !measured {
		t.Fatalf("the root shell was not measured, so this check would pass vacuously")
	}
	if len(counts) < 2 {
		t.Fatalf("measured %d packages under %s, so this check would pass vacuously", len(counts), tuiSubtree)
	}
	measured := map[string]bool{}
	for name := range counts {
		measured[name] = true
	}
	for _, name := range sortedNames(measured) {
		if counts[name] > tuiPackageFileCeiling {
			t.Errorf("package %s holds %d production .go files, over the ceiling of %d; move a concern out",
				name, counts[name], tuiPackageFileCeiling)
		}
	}
}

// tuiPackageFileCounts counts the production .go files each package under
// internal/tui holds. Test files are excluded: the ceiling is about how much
// production behavior one package carries, and a package's tests are as many
// files as its tests need.
func tuiPackageFileCounts(t *testing.T, root string) map[string]int {
	t.Helper()

	subtree := filepath.Join(root, filepath.FromSlash(tuiSubtree))
	counts := map[string]int{}
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
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		owner, err := tuiPackageForDir(subtree, filepath.Dir(path))
		if err != nil {
			return err
		}
		counts[owner]++
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", tuiSubtree, err)
	}
	return counts
}

// tuiPackageDirs names every package under internal/tui that holds at least one
// Go file, the root shell included. A directory with no Go file is not a
// package and the table must not carry a row for it.
func tuiPackageDirs(t *testing.T, root string) map[string]bool {
	t.Helper()

	subtree := filepath.Join(root, filepath.FromSlash(tuiSubtree))
	found := map[string]bool{}
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
		found[owner] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", tuiSubtree, err)
	}
	if len(found) == 0 {
		t.Fatalf("found no Go file under %s; the population check would pass vacuously", tuiSubtree)
	}
	return found
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
func readTUIImportEdges(t *testing.T, root string) []tuiImportEdge {
	t.Helper()

	subtree := filepath.Join(root, filepath.FromSlash(tuiSubtree))
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
	return edges
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
