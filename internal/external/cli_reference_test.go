package external

import (
	"strings"
	"testing"
)

const testReference = `# tt CLI Reference

## tt

A terminal-based task and project manager

Usage: tt [flags]

Flags:
  --config string    config file path
  --verbose, -v bool    verbose output

## tt add

Quick capture a task

Usage: tt add <title> [flags]

Flags:
  --bug bool    create a bug
  --epic bool    create an epic
  --tags stringSlice    tags for the task
  --priority int    priority (1=highest, 5=lowest)

## tt task list

List tasks in the current project

Usage: tt task list [flags]

Flags:
  --type, -t string    filter by task type
  --status, -s string    filter by status
  --tag string    filter by tag

## tt project new

Create a new project

Usage: tt project new <name> [flags]

## tt project delete

Delete a project

Usage: tt project delete <name> [flags]
`

func TestParseCLIReference(t *testing.T) {
	sections := ParseCLIReference(testReference)
	if len(sections) != 5 {
		t.Fatalf("expected 5 sections, got %d", len(sections))
	}

	if sections[0].Heading != "tt" {
		t.Errorf("expected heading 'tt', got %q", sections[0].Heading)
	}
	if sections[1].Heading != "tt add" {
		t.Errorf("expected heading 'tt add', got %q", sections[1].Heading)
	}

	// Check keywords include flag names
	addKw := sections[1].Keywords
	found := false
	for _, kw := range addKw {
		if kw == "tags" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'tags' in keywords for 'tt add', got %v", addKw)
	}
}

func TestSelectRelevantSections_TagQuery(t *testing.T) {
	sections := ParseCLIReference(testReference)
	result := SelectRelevantSections(sections, "how do I add tags?")

	if !strings.Contains(result, "## tt add") {
		t.Error("expected result to contain 'tt add' section")
	}
	if !strings.Contains(result, "## tt task list") {
		t.Error("expected result to contain 'tt task list' section (has --tag flag)")
	}
	// Should NOT contain unrelated sections as full content
	if strings.Contains(result, "## tt project delete\n\nDelete a project") {
		t.Error("result should not contain full 'tt project delete' section")
	}
}

func TestSelectRelevantSections_ProjectQuery(t *testing.T) {
	sections := ParseCLIReference(testReference)
	result := SelectRelevantSections(sections, "how to create a project")

	if !strings.Contains(result, "## tt project new") {
		t.Error("expected result to contain 'tt project new' section")
	}
}

func TestSelectRelevantSections_IgnoresSingleLetterQueryTokens(t *testing.T) {
	sections := ParseCLIReference(testReference)
	result := SelectRelevantSections(sections, "a")

	if !strings.Contains(result, "# Command Index") {
		t.Error("expected result to contain command index")
	}
	if strings.Contains(result, "## tt add\n\nQuick capture a task") {
		t.Error("single-letter query token should not match full command sections")
	}
	if strings.Contains(result, "## tt task list\n\nList tasks in the current project") {
		t.Error("single-letter query token should not match unrelated command sections")
	}
}

func TestSelectRelevantSections_AlwaysHasIndex(t *testing.T) {
	sections := ParseCLIReference(testReference)
	result := SelectRelevantSections(sections, "add tags")

	if !strings.Contains(result, "# Command Index") {
		t.Error("expected result to contain command index")
	}
	if !strings.Contains(result, "tt project delete") {
		t.Error("expected index to mention all commands including project delete")
	}
}

func TestBuildCLIContext_EmptyReference(t *testing.T) {
	result := BuildCLIContext("", "test query")
	if result != "" {
		t.Errorf("expected empty string for empty reference, got %q", result)
	}
}

func TestBuildCLIContext_HasHeader(t *testing.T) {
	result := BuildCLIContext(testReference, "add a task")
	if !strings.HasPrefix(result, "# tt CLI Reference") {
		t.Error("expected result to start with '# tt CLI Reference'")
	}
}
