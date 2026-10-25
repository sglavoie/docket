// Command tuirefs extracts a type-resolved, file-granular reference graph for
// one Go package, so a package split can be planned and executed against what
// the type checker sees rather than against grep.
//
// It loads the package with Tests: true, maps every top-level object to the
// file that declares it, walks TypesInfo.Uses, and emits one edge per use that
// crosses a file boundary, carrying the identifier name and the byte offset of
// the use. Comments, string literals, struct fields, methods and locals are
// excluded by construction, because only package-scope objects need a package
// qualifier once their declaring file moves.
//
// The emitted graph is the input to `scripts/tuiqualify`, which splices the
// qualifier in at those offsets. Each source file is emitted with its SHA-256
// so the rewriter can refuse to act on offsets that have since drifted.
//
//	go run ./scripts/tuirefs -o /tmp/refs.tsv
//	go run ./scripts/tuirefs -summary | head -40
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// defaultPackage is the package this refactor exists to split; every other
// package in the repository is small enough to read whole.
const defaultPackage = "github.com/sglavoie/docket/internal/tui"

// formatVersion prefixes the emitted graph. tuiqualify checks it, so a format
// change is a refusal rather than a misparse.
const formatVersion = "tuirefs v1"

// reference is one use of a top-level object from a file other than the one
// declaring it. Offsets are byte offsets into `from`, which is what makes the
// rewrite a splice rather than a re-print.
type reference struct {
	from   string
	to     string
	ident  string
	offset int
}

// graph is the whole emitted document: the references plus the SHA-256 of every
// file an offset points into.
type graph struct {
	pattern    string
	references []reference
	hashes     map[string]string
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("tuirefs", flag.ContinueOnError)
	flags.SetOutput(stderr)
	pattern := flags.String("pkg", defaultPackage, "package to analyze")
	dir := flags.String("dir", ".", "module root the analysis runs in")
	output := flags.String("o", "", "write the graph to this path instead of stdout")
	summary := flags.Bool("summary", false, "print per-file reference totals instead of the graph")
	if err := flags.Parse(args); err != nil {
		return err
	}

	root, err := filepath.Abs(*dir)
	if err != nil {
		return fmt.Errorf("resolve module root: %w", err)
	}

	extracted, err := analyze(root, *pattern)
	if err != nil {
		return err
	}

	sink := stdout
	if *output != "" {
		file, createErr := os.Create(*output)
		if createErr != nil {
			return fmt.Errorf("create %s: %w", *output, createErr)
		}
		defer func() { _ = file.Close() }()
		sink = file
	}

	buffered := bufio.NewWriter(sink)
	if *summary {
		writeSummary(buffered, extracted)
	} else {
		writeGraph(buffered, extracted)
	}
	return buffered.Flush()
}

// analyze loads the package and reduces it to cross-file references between
// top-level objects.
func analyze(root, pattern string) (*graph, error) {
	config := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Tests: true,
		Dir:   root,
	}
	loaded, err := packages.Load(config, pattern)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", pattern, err)
	}
	if err := loadErrors(loaded); err != nil {
		return nil, err
	}

	variants := packageVariants(loaded, pattern)
	if len(variants) == 0 {
		return nil, fmt.Errorf("load %s: no type-checked variant of that package", pattern)
	}

	declaring := declaringFiles(root, variants)
	if len(declaring) == 0 {
		return nil, fmt.Errorf("load %s: no top-level declaration found", pattern)
	}

	extracted := &graph{pattern: pattern, hashes: map[string]string{}}
	seen := map[string]bool{}
	for _, pkg := range variants {
		for ident, obj := range pkg.TypesInfo.Uses {
			if !topLevel(obj) || !samePackage(obj.Pkg().Path(), pattern) {
				continue
			}
			declared, ok := declaring[obj.Name()]
			if !ok {
				continue
			}
			position := pkg.Fset.Position(ident.Pos())
			from, relative := repositoryPath(root, position.Filename)
			if !relative || from == declared {
				continue
			}
			key := from + ":" + strconv.Itoa(position.Offset)
			if seen[key] {
				continue
			}
			seen[key] = true
			extracted.references = append(extracted.references, reference{
				from:   from,
				to:     declared,
				ident:  obj.Name(),
				offset: position.Offset,
			})
		}
	}

	sort.Slice(extracted.references, func(i, j int) bool {
		left, right := extracted.references[i], extracted.references[j]
		if left.from != right.from {
			return left.from < right.from
		}
		return left.offset < right.offset
	})

	for _, ref := range extracted.references {
		if _, ok := extracted.hashes[ref.from]; ok {
			continue
		}
		sum, hashErr := hashFile(filepath.Join(root, filepath.FromSlash(ref.from)))
		if hashErr != nil {
			return nil, hashErr
		}
		extracted.hashes[ref.from] = sum
	}
	return extracted, nil
}

// loadErrors collapses per-package load and type errors into one refusal. A
// partially type-checked package would silently under-report references, which
// is the one failure mode a rewrite must never inherit.
func loadErrors(loaded []*packages.Package) error {
	var reported []string
	packages.Visit(loaded, nil, func(pkg *packages.Package) {
		for _, pkgErr := range pkg.Errors {
			reported = append(reported, pkg.PkgPath+": "+pkgErr.Error())
		}
	})
	if len(reported) == 0 {
		return nil
	}
	return errors.New("package load failed:\n\t" + strings.Join(reported, "\n\t"))
}

// packageVariants keeps the variants that type-check the package's own source:
// the package itself and, under Tests: true, its in-package test variant. The
// generated test main is excluded because its path is `main`.
func packageVariants(loaded []*packages.Package, pattern string) []*packages.Package {
	var variants []*packages.Package
	packages.Visit(loaded, nil, func(pkg *packages.Package) {
		if pkg.TypesInfo == nil || pkg.Types == nil {
			return
		}
		if samePackage(pkg.Types.Path(), pattern) {
			variants = append(variants, pkg)
		}
	})
	return variants
}

// declaringFiles maps each top-level object name to the file that declares it.
// Names are unique across a package and its in-package test files, because
// they share one scope, so a name is a sufficient key across variants — and it
// has to be, since each variant type-checks to its own set of objects.
func declaringFiles(root string, variants []*packages.Package) map[string]string {
	declaring := map[string]string{}
	for _, pkg := range variants {
		for ident, obj := range pkg.TypesInfo.Defs {
			if !topLevel(obj) {
				continue
			}
			path, relative := repositoryPath(root, pkg.Fset.Position(ident.Pos()).Filename)
			if !relative {
				continue
			}
			declaring[obj.Name()] = path
		}
	}
	return declaring
}

// topLevel is the whole filter that keeps this analysis honest. A package-scope
// object is exactly the thing that needs a qualifier once its file moves;
// methods, struct fields, labels and locals have a nil parent scope or a
// function's, and none of them ever take one.
func topLevel(obj types.Object) bool {
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	return obj.Parent() == obj.Pkg().Scope()
}

// samePackage compares a types.Package path to the requested pattern, treating
// an external test package as the package it tests.
func samePackage(path, pattern string) bool {
	return path == pattern || strings.TrimSuffix(path, "_test") == pattern
}

// repositoryPath converts an absolute filename to a slash path relative to the
// module root, reporting false for anything outside it (a generated file, or
// the module cache).
func repositoryPath(root, filename string) (string, bool) {
	if filename == "" {
		return "", false
	}
	relative, err := filepath.Rel(root, filename)
	if err != nil || strings.HasPrefix(relative, "..") {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// emit writes one formatted line. Both commands stream their report through a
// bufio.Writer whose Flush is checked by the caller, and bufio.Writer latches
// the first write error, so a per-line check here would report the same failure
// a second time without adding anything.
func emit(out io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(out, format, args...)
}

func writeGraph(out io.Writer, extracted *graph) {
	emit(out, "# %s\n", formatVersion)
	emit(out, "# package\t%s\n", extracted.pattern)
	emit(out, "# file\tpath\tsha256\n")
	emit(out, "# edge\tfrom\tto\tident\toffset\tvisibility\n")

	paths := make([]string, 0, len(extracted.hashes))
	for path := range extracted.hashes {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		emit(out, "file\t%s\t%s\n", path, extracted.hashes[path])
	}
	for _, ref := range extracted.references {
		emit(out, "edge\t%s\t%s\t%s\t%d\t%s\n",
			ref.from, ref.to, ref.ident, ref.offset, visibility(ref.ident))
	}
}

// writeSummary sizes an extraction the way context.md's churn table does: how
// many references a file's declarations attract, and from how many consumers.
func writeSummary(out io.Writer, extracted *graph) {
	type tally struct {
		refs      int
		consumers map[string]bool
	}
	totals := map[string]*tally{}
	for _, ref := range extracted.references {
		entry, ok := totals[ref.to]
		if !ok {
			entry = &tally{consumers: map[string]bool{}}
			totals[ref.to] = entry
		}
		entry.refs++
		entry.consumers[ref.from] = true
	}

	files := make([]string, 0, len(totals))
	for path := range totals {
		files = append(files, path)
	}
	sort.Slice(files, func(i, j int) bool {
		left, right := totals[files[i]], totals[files[j]]
		if left.refs != right.refs {
			return left.refs > right.refs
		}
		return files[i] < files[j]
	})

	emit(out, "refs\tconsumers\tdeclaring_file\n")
	for _, path := range files {
		emit(out, "%d\t%d\t%s\n", totals[path].refs, len(totals[path].consumers), path)
	}
}

func visibility(ident string) string {
	if ast.IsExported(ident) {
		return "exported"
	}
	return "unexported"
}
