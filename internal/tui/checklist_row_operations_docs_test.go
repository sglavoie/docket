package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/operation"
)

// checklistDocumentationSections names, per document, the heading whose own
// prose owns the checklist row grammar. A section ends at the next heading of
// any level, so a neighboring or nested section can never lend this one a key
// or a label it did not print itself.
var checklistDocumentationSections = map[string]string{
	"docs/guide/outliner-rows.md":           "## Inline checklist rows",
	"docs/architecture/tui-architecture.md": "## TUI architecture",
}

func readRepositoryDocument(t *testing.T, relative string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve caller path")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", filepath.FromSlash(relative))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func checklistDocumentationSection(t *testing.T, relative string) string {
	t.Helper()
	heading, ok := checklistDocumentationSections[relative]
	if !ok {
		t.Fatalf("no checklist section is declared for %s", relative)
	}
	lines := strings.Split(readRepositoryDocument(t, relative), "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = i + 1
			break
		}
	}
	if start < 0 {
		t.Fatalf("%s no longer carries the heading %q", relative, heading)
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "#") {
			end = i
			break
		}
	}
	return strings.Join(strings.Fields(strings.Join(lines[start:end], " ")), " ")
}

func spelledOperationCount(t *testing.T, count int) string {
	t.Helper()
	words := []string{
		"zero", "one", "two", "three", "four",
		"five", "six", "seven", "eight", "nine",
	}
	if count < 0 || count >= len(words) {
		t.Fatalf("count %d is outside the range the documentation spells out", count)
	}
	return words[count]
}

type checklistDocumentedOperation struct {
	ID    operation.OperationID
	Label string
	// Key is the descriptor's own display key, empty when it answers none.
	Key string
}

type checklistDocumentationCatalog struct {
	operations []checklistDocumentedOperation
	byID       map[operation.OperationID]checklistDocumentedOperation
	totalWord  string
	keyedWord  string
}

// checklistDocumentationCatalogFromRegistry pairs every catalog descriptor with
// the key its own binding declares, which is the association the documents must
// reproduce. Reading the pair here rather than two independent populations is
// what lets a swapped key fail.
func checklistDocumentationCatalogFromRegistry(t *testing.T) checklistDocumentationCatalog {
	t.Helper()
	keys := make(map[operation.OperationID]string)
	for _, binding := range SimplifiedInputStrategy().Bindings {
		if operation.ChecklistRowGrammar[binding.OperationID] {
			keys[binding.OperationID] = binding.DisplayKey
		}
	}
	catalog := checklistDocumentationCatalog{byID: make(map[operation.OperationID]checklistDocumentedOperation)}
	for _, descriptor := range checklistRowOperationDescriptors() {
		operation := checklistDocumentedOperation{
			ID:    descriptor.ID,
			Label: descriptor.Label,
			Key:   keys[descriptor.ID],
		}
		catalog.operations = append(catalog.operations, operation)
		catalog.byID[descriptor.ID] = operation
	}
	catalog.totalWord = spelledOperationCount(t, len(catalog.operations))
	catalog.keyedWord = spelledOperationCount(t, len(keys))
	return catalog
}

func (c checklistDocumentationCatalog) unbound() []checklistDocumentedOperation {
	var unbound []checklistDocumentedOperation
	for _, operation := range c.operations {
		if operation.Key == "" {
			unbound = append(unbound, operation)
		}
	}
	return unbound
}

// checklistDocumentationFindings reports every way one section disagrees with
// the catalog. It is a pure function of the section text so the negative
// fixture below can feed it a deliberately corrupted copy.
func checklistDocumentationFindings(
	section string,
	catalog checklistDocumentationCatalog,
) []string {
	var findings []string
	lowered := strings.ToLower(section)
	if !strings.Contains(lowered, catalog.totalWord+" operations") {
		findings = append(findings, fmt.Sprintf(
			"the section does not count the checklist row grammar as %s operations",
			catalog.totalWord,
		))
	}
	if !strings.Contains(lowered, catalog.keyedWord+" of them") {
		findings = append(findings, fmt.Sprintf(
			"the section does not say %s of those operations answer a shared key",
			catalog.keyedWord,
		))
	}

	var labels, keys []string
	for _, operation := range catalog.operations {
		if !strings.Contains(section, operation.Label) {
			findings = append(findings, fmt.Sprintf(
				"the section does not name checklist operation %q", operation.Label,
			))
			continue
		}
		if operation.Key == "" {
			continue
		}
		labels = append(labels, operation.Label)
		printed := "`" + operation.Key + "`"
		if !strings.Contains(section, printed) {
			findings = append(findings, fmt.Sprintf(
				"the section does not print key %s for checklist operation %q",
				printed, operation.Label,
			))
			continue
		}
		keys = append(keys, printed)
	}
	findings = append(findings, checklistDocumentationOrderFindings(section, "label", labels)...)
	findings = append(findings, checklistDocumentationOrderFindings(section, "key", keys)...)

	for _, operation := range catalog.unbound() {
		unbound := regexp.MustCompile(regexp.QuoteMeta(operation.Label) + `[^.]*answers no key`)
		if !unbound.MatchString(section) {
			findings = append(findings, fmt.Sprintf(
				"the section does not say checklist operation %q answers no key", operation.Label,
			))
		}
	}
	return findings
}

// checklistDocumentationOrderFindings holds both documents to one convention:
// keys and labels are printed in catalog order. README pairs them per table
// row and docs/architecture.md prints two parallel lists, and under this rule
// either shape still binds a key to its own operation, because swapping one
// pair reverses the sequence it belongs to.
func checklistDocumentationOrderFindings(section, kind string, needles []string) []string {
	var findings []string
	previous, previousNeedle := -1, ""
	for _, needle := range needles {
		index := strings.Index(section, needle)
		if index < previous {
			findings = append(findings, fmt.Sprintf(
				"%s %s is printed before %s, which the catalog orders the other way",
				kind, needle, previousNeedle,
			))
		}
		previous, previousNeedle = index, needle
	}
	return findings
}

// TestChecklistRowOperationDocumentationMatchesCatalog binds both prose
// descriptions of the checklist row grammar to the catalog itself. README and
// docs/architecture.md count the same way — every descriptor, then the subset
// answering a key — and print label and key in catalog order, so neither a new
// operation nor a rebound key can land while either document keeps describing
// the old set.
func TestChecklistRowOperationDocumentationMatchesCatalog(t *testing.T) {
	catalog := checklistDocumentationCatalogFromRegistry(t)
	for relative := range checklistDocumentationSections {
		section := checklistDocumentationSection(t, relative)
		for _, finding := range checklistDocumentationFindings(section, catalog) {
			t.Errorf("%s: %s", relative, finding)
		}
	}
}

// TestChecklistRowOperationDocumentationRejectsSwappedAssociations is the
// negative fixture for the guard above. Swapping the two far-end move
// operations leaves every label and every key present, so only an assertion
// that reads them as pairs can reject it.
func TestChecklistRowOperationDocumentationRejectsSwappedAssociations(t *testing.T) {
	catalog := checklistDocumentationCatalogFromRegistry(t)
	bottom := catalog.byID[operation.OperationChecklistMoveBottom]
	top := catalog.byID[operation.OperationChecklistMoveTop]
	if bottom.Key == "" || top.Key == "" {
		t.Fatal("the far-end checklist move operations no longer answer keys")
	}

	corruptions := []struct {
		name   string
		first  string
		second string
	}{
		{"swapped far-end keys", "`" + bottom.Key + "`", "`" + top.Key + "`"},
		{"swapped far-end labels", bottom.Label, top.Label},
	}
	for relative := range checklistDocumentationSections {
		section := checklistDocumentationSection(t, relative)
		for _, corruption := range corruptions {
			corrupted := swapDocumentationTokens(section, corruption.first, corruption.second)
			if corrupted == section {
				t.Fatalf("%s: %s changed nothing", relative, corruption.name)
			}
			if len(checklistDocumentationFindings(corrupted, catalog)) == 0 {
				t.Errorf("%s: %s is accepted by the checklist documentation guard", relative, corruption.name)
			}
		}
	}
}

func swapDocumentationTokens(section, first, second string) string {
	const placeholder = "\x00checklist-swap\x00"
	section = strings.ReplaceAll(section, first, placeholder)
	section = strings.ReplaceAll(section, second, first)
	return strings.ReplaceAll(section, placeholder, second)
}
