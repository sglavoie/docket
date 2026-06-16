package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestSectionRename(t *testing.T) {
	t.Run("rejects renaming Next section", func(t *testing.T) {
		setupSectionCommandTest(t)

		err := runSectionRename(nil, []string{models.SectionNext, "Something"})
		if err == nil {
			t.Error("runSectionRename() should error when trying to rename Next section")
		}
	})

	t.Run("rejects renaming nonexistent section", func(t *testing.T) {
		setupSectionCommandTest(t)

		err := runSectionRename(nil, []string{"Nonexistent", "NewName"})
		if err == nil {
			t.Error("runSectionRename() should error for nonexistent section")
		}
	})

}

// TestSectionRenameRejectsGTDTarget guards against creating a duplicate that
// conflicts with the built-in section in filters and views.
func TestSectionRenameRejectsGTDTarget(t *testing.T) {
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
		t.Run("rejects target "+name, func(t *testing.T) {
			setupSectionCommandTest(t)

			err := runSectionRename(nil, []string{"Custom", name})
			if err == nil {
				t.Fatalf("runSectionRename to %q should error: built-in GTD name must be rejected as target", name)
			}
		})
	}
}

// TestSectionRenameRejectsGTDTargetCaseInsensitive guards against
// `tt section rename Foo inbox` (or any case variant of a GTD name) succeeding
// and silently creating a sibling of the built-in section.
func TestSectionRenameRejectsGTDTargetCaseInsensitive(t *testing.T) {
	variants := []string{"inbox", "INBOX", "next", "NEXT", "completed", "COMPLETED"}
	for _, name := range variants {
		t.Run("rejects target "+name, func(t *testing.T) {
			setupSectionCommandTest(t)

			err := runSectionRename(nil, []string{"Custom", name})
			if err == nil {
				t.Fatalf("runSectionRename to %q should error: case variant of GTD name must be rejected", name)
			}
		})
	}
}

func TestSectionRenameBuiltInSections(t *testing.T) {
	builtInSections := []string{
		models.SectionNext,
		models.SectionInbox,
		models.SectionWaiting,
		models.SectionSomeday,
		models.SectionReview,
		models.SectionStandby,
		models.SectionCompleted,
	}

	for _, section := range builtInSections {
		t.Run("rejects renaming built-in section "+section, func(t *testing.T) {
			setupSectionCommandTest(t)

			err := runSectionRename(nil, []string{section, "CustomName"})
			if err == nil {
				t.Errorf("runSectionRename() should error when trying to rename built-in section %q", section)
			}
		})
	}
}
