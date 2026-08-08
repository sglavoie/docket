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

func TestRunFindsProductionConsumedMessagesWithoutProductionEmitters(t *testing.T) {
	root := writeFixtureModule(t, map[string]string{
		"internal/tui/messages.go": `package tui

type OrphanMsg struct{}
type LiveMsg struct{}
// deadcodeaudit:external-message emitted by an external adapter.
type ExternalMsg struct{}

func handle(msg any) {
	switch msg.(type) {
	case OrphanMsg, LiveMsg, ExternalMsg:
	}
}

func emit() any { return LiveMsg{} }
`,
		"internal/tui/messages_test.go": `package tui
func emitOnlyInTests() any { return OrphanMsg{} }
`,
	})

	findings, err := Run(root, "./...")
	if err != nil {
		t.Fatal(err)
	}
	got := findingSymbols(findings)
	if !slices.Contains(got, "OrphanMsg") {
		t.Fatalf("findings %v missing OrphanMsg", got)
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
