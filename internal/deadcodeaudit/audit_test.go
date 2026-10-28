package deadcodeaudit

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestRunFindsProductionUnusedDeclarationsAndFields(t *testing.T) {
	root := writeFixtureModule(t, map[string]string{
		"sample/sample.go": `package sample

type DeadType struct{}
const deadConstant = 1

type Result struct {
	Read int
	Written int
	Tagged string ` + "`json:\"tagged\"`" + `
}

func Build() Result {
	result := Result{Read: 1, Written: 2}
	_ = result.Read
	result.Written++
	return result
}
`,
		"sample/sample_test.go": `package sample
func useOnlyInTests() { _ = DeadType{}; _ = deadConstant; _ = Build().Written }
`,
	})

	findings, err := Run(root, "./...")
	if err != nil {
		t.Fatal(err)
	}
	got := findingSymbols(findings)
	for _, want := range []string{"DeadType", "deadConstant", "Result.Written"} {
		if !slices.Contains(got, want) {
			t.Errorf("findings %v missing %q", got, want)
		}
	}
	for _, exempt := range []string{"Result.Read", "Result.Tagged"} {
		if slices.Contains(got, exempt) {
			t.Errorf("findings %v unexpectedly contain %q", got, exempt)
		}
	}
}

// A struct compared with == reads every field, nested ones included, even when
// no field is ever selected by name; a field of a type never compared stays
// unused.
func TestRunCountsWholeValueComparisonAsReadingEveryField(t *testing.T) {
	root := writeFixtureModule(t, map[string]string{
		"sample/sample.go": `package sample

type inner struct{ depth int }

type owner struct {
	layer int
	nested inner
}

type unrelated struct{ never int }

func Same(left, right owner) bool { return left == right }

func Build() (owner, unrelated) {
	return owner{layer: 1, nested: inner{depth: 2}}, unrelated{never: 3}
}
`,
	})

	findings, err := Run(root, "./...")
	if err != nil {
		t.Fatal(err)
	}
	got := findingSymbols(findings)
	for _, read := range []string{"owner.layer", "owner.nested", "inner.depth"} {
		if slices.Contains(got, read) {
			t.Errorf("findings %v contain %q, which == reads", got, read)
		}
	}
	if !slices.Contains(got, "unrelated.never") {
		t.Errorf("findings %v missing unrelated.never, which nothing reads", got)
	}
}

func TestRunHonorsReflectionDirectiveAndIgnoresAnonymousFields(t *testing.T) {
	root := writeFixtureModule(t, map[string]string{
		"sample/sample.go": `package sample

// deadcodeaudit:reflect populated by the reflection adapter.
type Reflected struct { Value string }

func Decode() string {
	value := struct { Name string }{Name: "ok"}
	return value.Name
}
`,
	})
	findings, err := Run(root, "./...")
	if err != nil {
		t.Fatal(err)
	}
	if got := findingSymbols(findings); len(got) != 0 {
		t.Fatalf("unexpected findings: %v", got)
	}
}

func TestRunIgnoresReadsFromTestSupportPackages(t *testing.T) {
	root := writeFixtureModule(t, map[string]string{
		"sample/sample.go": `package sample

type TestSupportType struct{}
var TestSupportValue = 1
`,
		"internal/testutil/support.go": `package testutil
import "example.com/auditfixture/sample"
var _ = sample.TestSupportType{}
var _ = sample.TestSupportValue
`,
	})

	findings, err := Run(root, "./...")
	if err != nil {
		t.Fatal(err)
	}
	got := findingSymbols(findings)
	for _, want := range []string{"TestSupportType", "TestSupportValue"} {
		if !slices.Contains(got, want) {
			t.Errorf("findings %v missing %q", got, want)
		}
	}
}

// The fixture declares messages both in the TUI shell package and in a TUI
// sub-package, because the audit gates on package path: a message moved out of
// the shell must stay audited rather than silently leaving the corpus.
func TestRunFindsProductionConsumedMessagesWithoutProductionEmitters(t *testing.T) {
	root := writeFixtureModule(t, map[string]string{
		"internal/tui/status/messages.go": `package status

type OrphanMsg struct{}
type LiveMsg struct{}
// deadcodeaudit:external-message emitted by an external adapter.
type ExternalMsg struct{}
`,
		"internal/tui/messages.go": `package tui

import "example.com/auditfixture/internal/tui/status"

type ShellOrphanMsg struct{}

func handle(msg any) {
	switch msg.(type) {
	case ShellOrphanMsg, status.OrphanMsg, status.LiveMsg, status.ExternalMsg:
	}
}

func emit() any { return status.LiveMsg{} }
`,
		"internal/tui/messages_test.go": `package tui

import "example.com/auditfixture/internal/tui/status"

func emitOnlyInTests() []any { return []any{ShellOrphanMsg{}, status.OrphanMsg{}} }
`,
	})

	findings, err := Run(root, "./...")
	if err != nil {
		t.Fatal(err)
	}
	got := findingSymbols(findings)
	for _, want := range []string{"ShellOrphanMsg", "OrphanMsg"} {
		if !slices.Contains(got, want) {
			t.Fatalf("findings %v missing %q", got, want)
		}
	}
	for _, exempt := range []string{"LiveMsg", "ExternalMsg"} {
		if slices.Contains(got, exempt) {
			t.Errorf("findings %v unexpectedly contain %q", got, exempt)
		}
	}
}

func findingSymbols(findings []Finding) []string {
	result := make([]string, 0, len(findings))
	for _, finding := range findings {
		result = append(result, finding.Symbol)
	}
	return result
}

func writeFixtureModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module example.com/auditfixture\n\ngo 1.25\n"
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
