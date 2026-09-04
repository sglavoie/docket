package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/route"
)

func (m *AppModel) activeOutliner() *OutlinerModel {
	if m == nil || m.navigation.Primary.Current().ID() != route.TaskWorkspace {
		return nil
	}
	return m.crossOutliner
}

// cursorProjectHeaderProject returns the project the Outliner cursor names when
// it rests on a project sub-header.
func (m *AppModel) cursorProjectHeaderProject() *models.Project {
	if out := m.activeOutliner(); out != nil {
		return out.cursorProjectHeaderProject()
	}
	return nil
}

// cursorNamesSectionHeader reports whether the Outliner cursor rests on a
// grouping section header.
func (m *AppModel) cursorNamesSectionHeader() bool {
	if out := m.activeOutliner(); out != nil {
		return out.cursorNamesSectionHeader()
	}
	return false
}

// cursorNoteIdentity returns the note the Outliner cursor names, so the shell's
// command context reads the same row kind the Outliner's own does.
func (m *AppModel) cursorNoteIdentity() (notes.NoteRowIdentity, bool) {
	if out := m.activeOutliner(); out != nil {
		return out.cursorNoteIdentity()
	}
	return notes.NoteRowIdentity{}, false
}

// cursorChecklistItem returns the checklist item the Outliner cursor names.
func (m *AppModel) cursorChecklistItem() (checklistCursor, bool) {
	if out := m.activeOutliner(); out != nil {
		return out.cursorChecklistItem()
	}
	return checklistCursor{}, false
}

// anchorOutlinerCursorAfterProjectPark points the cursor at the project header
// that will sit where the parked projects' rows were. It self-guards on the
// active Outliner, so a park committed from the Project Hub — where parked rows
// stay visible — leaves the cursor alone.
func (m *AppModel) anchorOutlinerCursorAfterProjectPark(projectIDs ...string) {
	out := m.activeOutliner()
	if out == nil {
		return
	}
	anchor, onScreen := out.projectHeaderAnchorAfterRemoval(projectIDs...)
	if !onScreen {
		return
	}
	out.crossReloadHeaderProjectID = anchor
}

func (m *AppModel) outlinerTarget() (*models.Task, *models.Project) {
	if out := m.activeOutliner(); out != nil {
		return out.getTaskRef()
	}
	return nil, nil
}
