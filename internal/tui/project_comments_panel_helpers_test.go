package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/tui/taskdetail"
)

// mountedProjectCommentsPanel is the panel `;` put on the route, or a failure
// naming the view that is there instead.
func mountedProjectCommentsPanel(t *testing.T, app *AppModel) *taskdetail.CommentsModel {
	t.Helper()
	panel := app.entities.ActiveCommentsPanel(app)
	if panel == nil {
		t.Fatalf("no Comments panel is mounted; view = %s", app.currentView())
	}
	return panel
}
