// Command tuiqualify performs the mechanical half of one package extraction:
// given the reference graph `scripts/tuirefs` emits and the file-to-package
// assignment table, it prefixes every use of a moving declaration with the new
// package qualifier and adds the import.
//
// It rewrites bytes at the offsets the type checker reported, so a name inside
// a comment, a string literal, a struct field or a method selector is untouched
// by construction — the distinction a word-boundary sed cannot make. Every file
// it edits is checked against the SHA-256 the graph recorded, so a stale graph
// is a refusal rather than a corrupt splice.
//
// It is deliberately incomplete. It does not export anything, does not move
// files, and cannot see a package qualifier shadowed by a local variable of the
// same name; the compiler reports that as a missing field or method on the
// shadowing value. Run it, then read what the build says.
//
//	go run ./scripts/tuirefs -o /tmp/refs.tsv
//	go run ./scripts/tuiqualify -edges /tmp/refs.tsv -package theme
//	go run ./scripts/tuiqualify -edges /tmp/refs.tsv -package theme -apply
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

const (
	// defaultAssignments is the Session 1.5 table: one production file per row,
	// one target package each.
	defaultAssignments = "scripts/tui_package_assignments.tsv"

	// assignmentRoot prefixes the table's paths, which are relative to the
	// package being split while the reference graph is repository-relative.
	assignmentRoot = "internal/tui"

	// importRoot is where an extracted sub-package lands.
	importRoot = "github.com/sglavoie/docket/internal/tui"

	// graphVersion must match what tuirefs writes.
	graphVersion = "# tuirefs v1"
)

// reference is one edge read back out of the graph.
type reference struct {
	from   string
	to     string
	ident  string
	offset int
}

// referenceGraph is the parsed graph document.
type referenceGraph struct {
	references []reference
	hashes     map[string]string
}

// rewrite is the work planned for one consumer file.
type rewrite struct {
	path    string
	offsets []int
	idents  map[string]bool
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("tuiqualify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	edges := flags.String("edges", "", "reference graph produced by scripts/tuirefs (required)")
	table := flags.String("assignments", defaultAssignments, "file-to-package assignment table")
	target := flags.String("package", "", "sub-package being extracted (required)")
	importPath := flags.String("import", "", "import path of the sub-package (default "+importRoot+"/<package>)")
	dir := flags.String("dir", ".", "module root the rewrite runs in")
	apply := flags.Bool("apply", false, "write the rewrite instead of reporting it")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *edges == "" || *target == "" {
		flags.Usage()
		return fmt.Errorf("both -edges and -package are required")
	}

	root, err := filepath.Abs(*dir)
	if err != nil {
		return fmt.Errorf("resolve module root: %w", err)
	}
	qualified := *importPath
	if qualified == "" {
		qualified = importRoot + "/" + *target
	}

	moving, err := movingFiles(resolve(root, *table), *target)
	if err != nil {
		return err
	}
	graph, err := readGraph(resolve(root, *edges))
	if err != nil {
		return err
	}

	plan, promotions := planRewrites(graph, moving)
	if err := verifyHashes(root, graph, plan); err != nil {
		return err
	}

	buffered := bufio.NewWriter(stdout)
	if err := applyRewrites(buffered, root, qualified, *target, plan, *apply); err != nil {
		_ = buffered.Flush()
		return err
	}
	reportPromotions(buffered, *target, promotions)
	return buffered.Flush()
}

// resolve interprets a command-line path relative to the module root unless it
// is already absolute, so a graph written to a scratch directory is reachable.
func resolve(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, filepath.FromSlash(path))
}

// movingFiles reads the assignment table and returns the repository-relative
// paths of the files landing in the target package.
func movingFiles(path, target string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read assignments: %w", err)
	}
	moving := map[string]bool{}
	known := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || fields[0] == "path" {
			continue
		}
		known[fields[1]] = true
		if fields[1] == target {
			moving[assignmentRoot+"/"+fields[0]] = true
		}
	}
	if len(moving) == 0 {
		names := make([]string, 0, len(known))
		for name := range known {
			names = append(names, name)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("no file is assigned to package %q; the table names %s",
			target, strings.Join(names, ", "))
	}
	return moving, nil
}

func readGraph(path string) (*referenceGraph, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read reference graph: %w", err)
	}
	defer func() { _ = file.Close() }()

	graph := &referenceGraph{hashes: map[string]string{}}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	version := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			version = version || strings.HasPrefix(line, graphVersion)
			continue
		}
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		switch fields[0] {
		case "file":
			if len(fields) != 3 {
				return nil, fmt.Errorf("malformed file record: %q", line)
			}
			graph.hashes[fields[1]] = fields[2]
		case "edge":
			if len(fields) != 6 {
				return nil, fmt.Errorf("malformed edge record: %q", line)
			}
			offset, convErr := strconv.Atoi(fields[4])
			if convErr != nil {
				return nil, fmt.Errorf("malformed edge offset: %q", line)
			}
			graph.references = append(graph.references, reference{
				from: fields[1], to: fields[2], ident: fields[3], offset: offset,
			})
		default:
			return nil, fmt.Errorf("unknown record kind %q", fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read reference graph: %w", err)
	}
	if !version {
		return nil, fmt.Errorf("reference graph does not start with %q; regenerate it", graphVersion)
	}
	if len(graph.references) == 0 {
		return nil, fmt.Errorf("reference graph holds no edge; regenerate it")
	}
	return graph, nil
}

// planRewrites selects the edges that cross the new boundary — a use in a file
// staying behind of a declaration that is leaving — and groups them per file.
// It also collects the unexported names the move forces public, which is the
// export-minimization decision the recipe asks each session to make by hand.
func planRewrites(graph *referenceGraph, moving map[string]bool) ([]rewrite, []string) {
	grouped := map[string]*rewrite{}
	promoted := map[string]bool{}
	for _, ref := range graph.references {
		if !moving[ref.to] || moving[ref.from] {
			continue
		}
		entry, ok := grouped[ref.from]
		if !ok {
			entry = &rewrite{path: ref.from, idents: map[string]bool{}}
			grouped[ref.from] = entry
		}
		entry.offsets = append(entry.offsets, ref.offset)
		entry.idents[ref.ident] = true
		if !ast.IsExported(ref.ident) {
			promoted[ref.ident] = true
		}
	}

	plan := make([]rewrite, 0, len(grouped))
	for _, entry := range grouped {
		sort.Ints(entry.offsets)
		plan = append(plan, *entry)
	}
	sort.Slice(plan, func(i, j int) bool { return plan[i].path < plan[j].path })

	promotions := make([]string, 0, len(promoted))
	for name := range promoted {
		promotions = append(promotions, name)
	}
	sort.Strings(promotions)
	return plan, promotions
}

// emit writes one formatted line. Both commands stream their report through a
// bufio.Writer whose Flush is checked by the caller, and bufio.Writer latches
// the first write error, so a per-line check here would report the same failure
// a second time without adding anything.
func emit(out io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(out, format, args...)
}

func applyRewrites(out io.Writer, root, importPath, qualifier string, plan []rewrite, apply bool) error {
	total := 0
	for _, entry := range plan {
		total += len(entry.offsets)
	}
	mode := "would rewrite"
	if apply {
		mode = "rewrote"
	}
	emit(out, "%s %d references across %d files into %s\n", mode, total, len(plan), importPath)
	emit(out, "\tconsumer\trefs\tidents\n")

	for _, entry := range plan {
		emit(out, "\t%s\t%d\t%d\n", entry.path, len(entry.offsets), len(entry.idents))
		if !apply {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(entry.path))
		original, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.path, err)
		}
		rewritten, err := qualifySource(entry.path, original, entry.offsets, importPath, qualifier)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, rewritten, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", entry.path, err)
		}
	}
	return nil
}

// qualifySource splices the qualifier in ascending offset order and then adds
// the import to the reparsed file, so the printed result is gofmt-clean.
func qualifySource(path string, original []byte, offsets []int, importPath, qualifier string) ([]byte, error) {
	var spliced bytes.Buffer
	previous := 0
	for _, offset := range offsets {
		if offset < previous || offset > len(original) {
			return nil, fmt.Errorf("%s: reference offset %d is out of range", path, offset)
		}
		spliced.Write(original[previous:offset])
		spliced.WriteString(qualifier + ".")
		previous = offset
	}
	spliced.Write(original[previous:])

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, spliced.Bytes(), parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("reparse %s after qualification: %w", path, err)
	}
	astutil.AddImport(fset, file, importPath)

	var printed bytes.Buffer
	if err := format.Node(&printed, fset, file); err != nil {
		return nil, fmt.Errorf("format %s: %w", path, err)
	}
	return printed.Bytes(), nil
}

// verifyHashes refuses to act on a graph whose offsets no longer describe the
// files on disk. Offsets are the whole mechanism, so a stale graph is the one
// way this tool could silently corrupt source.
func verifyHashes(root string, graph *referenceGraph, plan []rewrite) error {
	var drifted []string
	for _, entry := range plan {
		recorded, ok := graph.hashes[entry.path]
		if !ok {
			drifted = append(drifted, entry.path+" (no hash recorded)")
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(entry.path)))
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.path, err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != recorded {
			drifted = append(drifted, entry.path)
		}
	}
	if len(drifted) == 0 {
		return nil
	}
	return fmt.Errorf("the reference graph is stale for %d file(s); re-run scripts/tuirefs:\n\t%s",
		len(drifted), strings.Join(drifted, "\n\t"))
}

func reportPromotions(out io.Writer, target string, promotions []string) {
	if len(promotions) == 0 {
		emit(out, "no unexported declaration crosses the %s boundary\n", target)
		return
	}
	emit(out, "%d unexported declarations cross the %s boundary and must be exported or pulled back:\n",
		len(promotions), target)
	for _, name := range promotions {
		emit(out, "\t%s\n", name)
	}
}
