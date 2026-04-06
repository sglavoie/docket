package cmd

import (
	"testing"

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