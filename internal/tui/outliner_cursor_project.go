package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// cursorProject resolves the project the Outliner cursor sits inside, whatever
// row kind it rests on. Task rows and project headers both name their project
// directly; a scoped Outliner has only one project to name. `O` reads it to
// decide which project the outline editor dumps.
func (m *OutlinerModel) cursorProject() *models.Project {
	if m == nil {
		return nil
	}
	if m.Cursor >= 0 && m.Cursor < len(m.rows) {
		if projectID := m.rows[m.Cursor].ProjectID; projectID != "" {
			if m.project != nil && m.project.ID == projectID {
				return m.project
			}
			if project := m.crossResolveProject(projectID); project != nil {
				return project
			}
		}
	}
	return m.project
}
