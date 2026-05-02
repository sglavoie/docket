package models

import (
	"testing"
)

func TestRemoveSectionRejectsBuiltInSections(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.EnsureDefaultSections()

	builtInSections := []string{
		SectionNext, SectionInbox, SectionWaiting,
		SectionSomeday, SectionReview, SectionStandby, SectionCompleted,
	}

	for _, section := range builtInSections {
		if project.RemoveSection(section) {
			t.Errorf("RemoveSection(%q) should return false for built-in section", section)
		}
		if !project.HasSection(section) {
			t.Errorf("built-in section %q should still exist after failed removal", section)
		}
	}
}

func TestRenameSectionRejectsBuiltInSections(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.EnsureDefaultSections()

	builtInSections := []string{
		SectionNext, SectionInbox, SectionWaiting,
		SectionSomeday, SectionReview, SectionStandby, SectionCompleted,
	}

	for _, section := range builtInSections {
		if err := project.RenameSection(section, "CustomName"); err == nil {
			t.Errorf("RenameSection(%q, ...) should return error for built-in section", section)
		}
		if !project.HasSection(section) {
			t.Errorf("built-in section %q should still exist after failed rename", section)
		}
	}
}
