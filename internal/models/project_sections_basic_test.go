package models

import (
	"strings"
	"testing"
)

func TestProjectGetSections(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Default should be just Next
	sections := project.GetSections()
	if len(sections) != 1 || sections[0] != SectionNext {
		t.Errorf("expected [Next], got %v", sections)
	}

	// Add more sections
	project.Sections = []string{"Active", "Done", SectionNext}
	sections = project.GetSections()

	// Next should always be first
	if sections[0] != SectionNext {
		t.Errorf("expected Next first, got %s", sections[0])
	}
	if len(sections) != 3 {
		t.Errorf("expected 3 sections, got %d", len(sections))
	}

	// Test with empty sections - should return Next
	project.Sections = []string{}
	sections = project.GetSections()
	if len(sections) != 1 || sections[0] != SectionNext {
		t.Errorf("expected [Next] for empty sections, got %v", sections)
	}

	// Test with sections missing Next - should prepend it
	project.Sections = []string{"Active", "Done"}
	sections = project.GetSections()
	if sections[0] != SectionNext {
		t.Errorf("expected Next first even when missing, got %s", sections[0])
	}
	if len(sections) != 3 {
		t.Errorf("expected 3 sections after prepending Next, got %d", len(sections))
	}
}

func TestProjectHasSection(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.Sections = []string{SectionNext, "Active", "Done"}

	if !project.HasSection(SectionNext) {
		t.Error("expected HasSection to return true for Next")
	}
	if !project.HasSection("Active") {
		t.Error("expected HasSection to return true for Active")
	}
	if project.HasSection("Nonexistent") {
		t.Error("expected HasSection to return false for Nonexistent")
	}
}

func TestProjectEnsureSection(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	initialLen := len(project.Sections)

	// Adding existing section should not add duplicate
	project.EnsureSection(SectionNext)
	if len(project.Sections) != initialLen {
		t.Error("EnsureSection should not add duplicate Next")
	}

	// Adding new section should add it
	project.EnsureSection("Active")
	if !project.HasSection("Active") {
		t.Error("EnsureSection should add new section")
	}

	// Adding empty string should do nothing
	prevLen := len(project.Sections)
	project.EnsureSection("")
	if len(project.Sections) != prevLen {
		t.Error("EnsureSection should not add empty string")
	}

	// Adding whitespace-only string should do nothing
	project.EnsureSection("   ")
	if len(project.Sections) != prevLen {
		t.Error("EnsureSection should not add whitespace-only string")
	}

	// Adding a very long name should do nothing
	longName := strings.Repeat("a", MaxSectionNameLength+1)
	project.EnsureSection(longName)
	if len(project.Sections) != prevLen {
		t.Error("EnsureSection should not add excessively long name")
	}
}
