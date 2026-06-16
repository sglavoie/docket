package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestSectionRemove(t *testing.T) {
	t.Run("rejects removing Next section", func(t *testing.T) {
		setupSectionCommandTest(t)

		err := runSectionRemove(nil, []string{models.SectionNext})
		if err == nil {
			t.Error("runSectionRemove() should error when trying to remove Next section")
		}
	})

	t.Run("rejects removing nonexistent section", func(t *testing.T) {
		setupSectionCommandTest(t)

		err := runSectionRemove(nil, []string{"Nonexistent"})
		if err == nil {
			t.Error("runSectionRemove() should error for nonexistent section")
		}
	})

}

func TestSectionRemoveBuiltInSections(t *testing.T) {
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
		t.Run("rejects removing built-in section "+section, func(t *testing.T) {
			setupSectionCommandTest(t)

			sectionRemoveForce = true
			defer func() { sectionRemoveForce = false }()

			err := runSectionRemove(nil, []string{section})
			if err == nil {
				t.Errorf("runSectionRemove() should error when trying to remove built-in section %q", section)
			}
		})
	}
}
