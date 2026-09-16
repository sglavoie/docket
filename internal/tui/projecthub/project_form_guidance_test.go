package projecthub

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
)

// The project creator and editor are the two forms this package owns, and their
// guidance is the half of the shell's form coverage that moved with them. The
// text editor's own budget stays beside the shell, which still owns it.

// The exit entry names what Esc actually reaches, which is why each form is
// driven through its own inputs here rather than through a flag: an untouched
// form leaves outright and an edited one raises the discard question, and both
// forms answer that question from the same predicate their Esc handler reads.
func TestProjectFormGuidanceRequiresCommitAndCancelByState(t *testing.T) {
	creator := &ProjectCreatorModel{}
	assertRequiredGuidance(t, "project creator", creator.guidanceSpec(), "Enter create", "Esc cancel")
	creator.nameInput.SetValue("Roadmap")
	assertRequiredGuidance(t, "drafted project creator", creator.guidanceSpec(), "Enter create", "Esc discard")
	creator.ConfirmDiscard = true
	assertRequiredGuidance(t, "project creator discard", creator.guidanceSpec(), "Y discard", "N/Esc keep editing")

	editor := &ProjectEditorModel{originalName: "Roadmap"}
	editor.nameInput.SetValue("Roadmap")
	assertRequiredGuidance(t, "untouched project editor", editor.guidanceSpec(), "Ctrl+S save", "Esc cancel")
	editor.nameInput.SetValue("Roadmap 2")
	editor.hasChanges = true
	assertRequiredGuidance(t, "dirty project editor", editor.guidanceSpec(), "Ctrl+S save", "Esc discard")
	editor.err = errors.New("invalid project")
	assertRequiredGuidance(t, "invalid project editor", editor.guidanceSpec(), "Ctrl+S retry", "Esc discard")
	editor.ConfirmDiscard = true
	assertRequiredGuidance(t, "project editor discard", editor.guidanceSpec(), "Y discard", "N/Esc keep editing")
}

func TestProjectFormGuidanceFitsNarrowBudgets(t *testing.T) {
	dirtyEditor := &ProjectEditorModel{hasChanges: true}
	dirtyEditor.nameInput.SetValue("Roadmap 2")
	specs := []chrome.GuidanceSpec{
		(&ProjectCreatorModel{}).guidanceSpec(),
		dirtyEditor.guidanceSpec(),
		chrome.DiscardGuidanceSpec(),
	}
	for index, spec := range specs {
		if spec.Rows != 2 {
			t.Errorf("spec %d row budget = %d, want 2", index, spec.Rows)
		}
		assertGuidanceFits(t, spec, 24)
	}
}
