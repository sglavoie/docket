package projecthub

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
)

// The project creator and editor are the two forms this package owns, and their
// guidance is the half of the shell's form coverage that moved with them. The
// text editor's own budget stays beside the shell, which still owns it.

func TestProjectFormGuidanceRequiresCommitAndCancelByState(t *testing.T) {
	creator := &ProjectCreatorModel{}
	assertRequiredGuidance(t, "project creator", creator.guidanceSpec(), "Enter create", "Esc cancel")
	creator.ConfirmDiscard = true
	assertRequiredGuidance(t, "project creator discard", creator.guidanceSpec(), "y confirm", "n/Esc cancel")

	editor := &ProjectEditorModel{hasChanges: true}
	assertRequiredGuidance(t, "dirty project editor", editor.guidanceSpec(), "Ctrl+S save", "Esc cancel")
	editor.err = errors.New("invalid project")
	assertRequiredGuidance(t, "invalid project editor", editor.guidanceSpec(), "Ctrl+S retry", "Esc cancel")
}

func TestProjectFormGuidanceFitsNarrowBudgets(t *testing.T) {
	specs := []chrome.GuidanceSpec{
		(&ProjectCreatorModel{}).guidanceSpec(),
		(&ProjectEditorModel{hasChanges: true}).guidanceSpec(),
	}
	for index, spec := range specs {
		if spec.Rows != 2 {
			t.Errorf("spec %d row budget = %d, want 2", index, spec.Rows)
		}
		assertGuidanceFits(t, spec, 24)
	}
}
