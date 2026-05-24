package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSectionAdd(t *testing.T) {
	t.Run("adds new section successfully", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		initialSections := len(project.Sections)

		err := runSectionAdd(nil, []string{"InProgress"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Sections) != initialSections+1 {
			t.Errorf("expected %d sections, got %d", initialSections+1, len(savedProject.Sections))
		}

		if !savedProject.HasSection("InProgress") {
			t.Error("section 'InProgress' was not added")
		}
	})

	t.Run("rejects empty section name", func(t *testing.T) {
		setupSectionCommandTest(t)

		err := runSectionAdd(nil, []string{""})
		if err == nil {
			t.Error("runSectionAdd() should error for empty section name")
		}
	})

	t.Run("rejects duplicate section", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		project.EnsureSection("ExistingSection")
		deps.Repository.Projects[project.ID] = project

		err := runSectionAdd(nil, []string{"ExistingSection"})
		if err == nil {
			t.Error("runSectionAdd() should error for duplicate section")
		}
	})

	t.Run("rejects section name with spaces", func(t *testing.T) {
		setupSectionCommandTest(t)

		err := runSectionAdd(nil, []string{"Has Space"})
		if err == nil {
			t.Error("runSectionAdd() should error for section name with spaces")
		}
	})

	t.Run("rejects section name with colon", func(t *testing.T) {
		setupSectionCommandTest(t)

		err := runSectionAdd(nil, []string{"Has:Colon"})
		if err == nil {
			t.Error("runSectionAdd() should error for section name with colons (used as color delimiter)")
		}
	})
}

// TestSectionAddRejectsGTDNamesCaseInsensitive is a regression test for a bug
// where `tt section add inbox` (lowercase) or any other case-variant of a
// built-in GTD section name succeeded because IsGTDSection used exact-case
// matching. The resulting custom "inbox" section then coexisted with the
// built-in "Inbox", producing two sibling sections that confuse filters,
// the section sidebar, and the /sigil quick-capture syntax.
func TestSectionAddRejectsGTDNamesCaseInsensitive(t *testing.T) {
	variants := []string{
		"inbox", "INBOX", "iNbOx",
		"next", "NEXT",
		"waiting", "WAITING",
		"someday", "SOMEDAY",
		"review", "REVIEW",
		"standby", "STANDBY",
		"completed", "COMPLETED",
	}
	for _, name := range variants {
		t.Run("rejects case variant "+name, func(t *testing.T) {
			deps, project := setupSectionCommandTest(t)
			before := len(project.Sections)
			err := runSectionAdd(nil, []string{name})
			if err == nil {
				t.Fatalf("runSectionAdd(%q) should error: case variant of built-in GTD name must be rejected", name)
			}
			saved := deps.Repository.Projects[project.ID]
			if saved != nil && len(saved.Sections) != before {
				t.Fatalf("project.Sections count changed from %d to %d after failed add of %q", before, len(saved.Sections), name)
			}
		})
	}
}

// TestSectionAddRejectsGTDNames is a regression test for a bug where
// `tt section add Inbox` (or any other GTD built-in name) succeeded and
// appended the name to project.Sections, creating a duplicate of the
// built-in section that conflicts with section-filter logic.
func TestSectionAddRejectsGTDNames(t *testing.T) {
	builtIns := []string{
		models.SectionNext,
		models.SectionInbox,
		models.SectionWaiting,
		models.SectionSomeday,
		models.SectionReview,
		models.SectionStandby,
		models.SectionCompleted,
	}
	for _, name := range builtIns {
		t.Run("rejects built-in "+name, func(t *testing.T) {
			deps, project := setupSectionCommandTest(t)
			before := 0
			for _, s := range project.Sections {
				if s == name {
					before++
				}
			}
			err := runSectionAdd(nil, []string{name})
			if err == nil {
				t.Fatalf("runSectionAdd(%q) should error: built-in GTD name must be rejected", name)
			}
			saved := deps.Repository.Projects[project.ID]
			if saved != nil {
				after := 0
				for _, s := range saved.Sections {
					if s == name {
						after++
					}
				}
				if after != before {
					t.Fatalf("project.Sections count for %q changed from %d to %d after failed add", name, before, after)
				}
			}
		})
	}
}
