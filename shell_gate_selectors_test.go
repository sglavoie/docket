package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"regexp/syntax"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// gateScripts are the shell gates that select tests by `-run` regex rather than
// by running a whole package.
var gateScripts = []string{"scripts/check_docs.sh", "scripts/run_tui_scenarios.sh"}

// selectorAlternativeBudget caps the alternation cross product, so a future
// selector built from nested groups degrades to a whole-pattern check instead
// of enumerating combinatorially many branches.
const selectorAlternativeBudget = 128

// testNameLine matches the bare test names `go test -list` prints. Every other
// line it emits is a package summary (`ok`, `?`, `FAIL`) or a build diagnostic
// (`#`), none of which start a line with a test-function prefix.
var testNameLine = regexp.MustCompile(`^(Test|Benchmark|Example|Fuzz)\S*$`)

// goTestInvocation is one `go test` command parsed out of a gate script.
type goTestInvocation struct {
	line     int
	selector string
	packages []string
}

// scriptLine is one logical shell line: backslash continuations already joined,
// numbered by where the continuation started.
type scriptLine struct {
	number int
	text   string
}

// TestShellGatesSelectAtLeastOneTest closes the hole that `go test` exits 0
// when a `-run` pattern matches nothing. Both gate scripts name tests by regex,
// so the moment a selected test file is renamed or moved to another package the
// gate keeps reporting success while testing nothing. Every selector in both
// scripts is replayed through `go test -list` and must name at least one test.
//
// Each selector is also required to be live alternative by alternative. A bare
// per-selector check is too weak to notice a real break: every selector here is
// an alternation, so one dead alternative hides behind its live siblings and
// the gate stays green while one whole test family has stopped being selected.
func TestShellGatesSelectAtLeastOneTest(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	parsed := make(map[string][]goTestInvocation, len(gateScripts))
	var patterns []string
	for _, script := range gateScripts {
		parsed[script] = parseGoTestInvocations(t, root, script)
		for _, invocation := range parsed[script] {
			patterns = append(patterns, invocation.packages...)
		}
	}
	listed := listPackageTests(t, root, append(patterns, "."))

	// An impossible selector proves the zero-match detector itself works, so a
	// regression in parsing or counting cannot quietly make this audit vacuous.
	if names := selectTests(t, listed, "^TestNoSuchTestExistsUnderThisName$", []string{"."}); len(names) != 0 {
		t.Fatalf("impossible selector listed %v; the zero-match detector is broken", names)
	}
	if names := selectTests(t, listed, "^TestShellGatesSelectAtLeastOneTest$", []string{"."}); len(names) != 1 {
		t.Fatalf("this audit's own name listed %v; the listing is broken", names)
	}

	for _, script := range gateScripts {
		invocations := parsed[script]
		if len(invocations) == 0 {
			t.Errorf("%s: parsed no `go test -run` invocation", script)
			continue
		}
		for _, invocation := range invocations {
			scope := strings.Join(invocation.packages, " ")
			names := selectTests(t, listed, invocation.selector, invocation.packages)
			if len(names) == 0 {
				t.Errorf(
					"%s:%d: selector %q matches no test in %s; the gate would exit 0 without running anything",
					script, invocation.line, invocation.selector, scope,
				)
				continue
			}
			// Every alternative is a subset of the selector, so the names the
			// selector already listed are the whole candidate set.
			for _, alternative := range selectorAlternatives(t, invocation.selector) {
				if !matchesAny(t, alternative, names) {
					t.Errorf(
						"%s:%d: selector %q alternative %q matches no test in %s; that branch of the gate is dead",
						script, invocation.line, invocation.selector, alternative, scope,
					)
				}
			}
		}
	}
}

// selectorAlternatives enumerates the selector's alternation branches, so a
// pattern naming several test families is audited one family at a time. A
// pattern whose branches multiply out beyond the budget is audited whole rather
// than expanded, which is weaker but never wrong.
func selectorAlternatives(t *testing.T, selector string) []string {
	t.Helper()
	parsed, err := syntax.Parse(selector, syntax.Perl)
	if err != nil {
		t.Fatalf("parse selector %q: %v", selector, err)
	}
	expanded := expandAlternations(parsed)
	if len(expanded) == 0 || len(expanded) > selectorAlternativeBudget {
		return []string{selector}
	}
	alternatives := make([]string, 0, len(expanded))
	for _, variant := range expanded {
		alternatives = append(alternatives, variant.String())
	}
	return alternatives
}

// expandAlternations rewrites a parsed pattern into the set of alternation-free
// patterns it is the union of. It returns nil once the cross product would
// exceed the budget.
func expandAlternations(re *syntax.Regexp) []*syntax.Regexp {
	if re.Op == syntax.OpAlternate {
		var variants []*syntax.Regexp
		for _, sub := range re.Sub {
			expanded := expandAlternations(sub)
			if expanded == nil || len(variants)+len(expanded) > selectorAlternativeBudget {
				return nil
			}
			variants = append(variants, expanded...)
		}
		return variants
	}
	if len(re.Sub) == 0 {
		return []*syntax.Regexp{re}
	}

	combinations := [][]*syntax.Regexp{nil}
	for _, sub := range re.Sub {
		options := expandAlternations(sub)
		if options == nil || len(combinations)*len(options) > selectorAlternativeBudget {
			return nil
		}
		next := make([][]*syntax.Regexp, 0, len(combinations)*len(options))
		for _, prefix := range combinations {
			for _, option := range options {
				combination := make([]*syntax.Regexp, len(prefix), len(prefix)+1)
				copy(combination, prefix)
				next = append(next, append(combination, option))
			}
		}
		combinations = next
	}

	variants := make([]*syntax.Regexp, 0, len(combinations))
	for _, combination := range combinations {
		variant := *re
		variant.Sub0 = [1]*syntax.Regexp{}
		variant.Sub = combination
		variants = append(variants, &variant)
	}
	return variants
}

// matchesAny applies one alternative with the same unanchored semantics
// `go test -run` and `-list` use on a top-level test name.
func matchesAny(t *testing.T, alternative string, names []string) bool {
	t.Helper()
	pattern, err := regexp.Compile(alternative)
	if err != nil {
		t.Fatalf("compile selector alternative %q: %v", alternative, err)
	}
	for _, name := range names {
		if pattern.MatchString(name) {
			return true
		}
	}
	return false
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve shell gate audit path")
	}
	return filepath.Dir(filename)
}

// parseGoTestInvocations reads one gate script and returns every `go test`
// command in it that carries a `-run` selector. The parsed count is held
// against the selectors the script actually spells, so a parser that silently
// stops recognizing an invocation fails here instead of shrinking the audit.
func parseGoTestInvocations(t *testing.T, root, script string) []goTestInvocation {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, script))
	if err != nil {
		t.Fatalf("read %s: %v", script, err)
	}

	lines := logicalLines(string(data))
	var invocations []goTestInvocation
	spelled := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line.text), "#") {
			continue
		}
		spelled += strings.Count(line.text, "-run")
		invocation, ok := parseGoTestLine(line.text)
		if !ok {
			continue
		}
		invocation.line = line.number
		invocations = append(invocations, invocation)
	}
	if len(invocations) != spelled {
		t.Fatalf(
			"%s: parsed %d selectors but the script spells %d; teach this parser the new invocation form",
			script, len(invocations), spelled,
		)
	}
	return invocations
}

// logicalLines joins backslash continuations so a command split across source
// lines is tokenized as one.
func logicalLines(content string) []scriptLine {
	raw := strings.Split(content, "\n")
	var lines []scriptLine
	for index := 0; index < len(raw); index++ {
		start := index
		text := raw[index]
		for strings.HasSuffix(text, "\\") && index+1 < len(raw) {
			index++
			text = strings.TrimSuffix(text, "\\") + " " + raw[index]
		}
		lines = append(lines, scriptLine{number: start + 1, text: text})
	}
	return lines
}

// parseGoTestLine extracts the selector and package arguments of a `go test`
// command. Leading environment assignments are skipped, and a command without a
// `-run` selector is not an invocation this audit covers.
func parseGoTestLine(line string) (goTestInvocation, bool) {
	tokens := shellTokens(line)
	start := -1
	for index := 0; index+1 < len(tokens); index++ {
		if tokens[index] == "go" && tokens[index+1] == "test" {
			start = index + 2
			break
		}
	}
	if start < 0 {
		return goTestInvocation{}, false
	}

	var invocation goTestInvocation
	for index := start; index < len(tokens); index++ {
		token := tokens[index]
		switch {
		case token == "-run" && index+1 < len(tokens):
			index++
			invocation.selector = tokens[index]
		case strings.HasPrefix(token, "-run="):
			invocation.selector = strings.TrimPrefix(token, "-run=")
		case strings.HasPrefix(token, "-"):
		case token == "." || strings.HasPrefix(token, "./"):
			invocation.packages = append(invocation.packages, token)
		}
	}
	if invocation.selector == "" || len(invocation.packages) == 0 {
		return goTestInvocation{}, false
	}
	return invocation, true
}

// shellTokens splits a shell line on unquoted whitespace, keeping quoted regex
// selectors whole.
func shellTokens(line string) []string {
	var tokens []string
	var current strings.Builder
	var quote rune
	started := false
	for _, symbol := range line {
		switch {
		case quote != 0:
			if symbol == quote {
				quote = 0
				break
			}
			current.WriteRune(symbol)
		case symbol == '\'' || symbol == '"':
			quote = symbol
			started = true
		case symbol == ' ' || symbol == '\t':
			if started {
				tokens = append(tokens, current.String())
				current.Reset()
				started = false
			}
		default:
			current.WriteRune(symbol)
			started = true
		}
	}
	if started {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// listPackageTests lists every test name under each distinct package pattern
// once, all patterns at the same time. Each gate selector is then applied
// in-process: `go test -list` matches its pattern against top-level names with
// the same unanchored semantics, so filtering one complete listing answers
// every selector without building each test binary again per selector.
func listPackageTests(t *testing.T, root string, patterns []string) map[string][]string {
	t.Helper()
	type listing struct {
		names []string
		err   error
	}
	results := make(map[string]*listing)
	var group sync.WaitGroup
	for _, pattern := range patterns {
		if results[pattern] != nil {
			continue
		}
		result := &listing{}
		results[pattern] = result
		group.Go(func() {
			result.names, result.err = listTests(root, pattern)
		})
	}
	group.Wait()

	listed := make(map[string][]string, len(results))
	for pattern, result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		listed[pattern] = result.names
	}
	return listed
}

// selectTests reports the listed test names under packages that selector
// matches, as `go test -list selector packages...` would print them.
func selectTests(t *testing.T, listed map[string][]string, selector string, packages []string) []string {
	t.Helper()
	pattern, err := regexp.Compile(selector)
	if err != nil {
		t.Fatalf("compile selector %q: %v", selector, err)
	}
	var names []string
	for _, pkg := range packages {
		all, ok := listed[pkg]
		if !ok {
			t.Fatalf("package pattern %q was never listed", pkg)
		}
		for _, name := range all {
			if pattern.MatchString(name) {
				names = append(names, name)
			}
		}
	}
	return names
}

// listTests reports every test name `go test -list` prints for one package
// pattern.
func listTests(root, pattern string) ([]string, error) {
	arguments := []string{"test", "-list", ".", pattern}
	command := exec.Command("go", arguments...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go %s: %w\n%s", strings.Join(arguments, " "), err, output)
	}

	var names []string
	for _, line := range strings.Split(string(output), "\n") {
		if testNameLine.MatchString(line) {
			names = append(names, line)
		}
	}
	return names, nil
}
