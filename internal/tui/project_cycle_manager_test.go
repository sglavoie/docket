package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestRenderProjectCyclePopupKeepsCountsOnSameLine(t *testing.T) {
	app := newTestAppModel()
	app.width = 80
	app.height = 24
	app.showProjectCycle = true
	app.config.FilterCustomContext = "Work"
	app.config.SetCycleProjectsForContext("+Work", []string{"p1", "p2", "p3", "p4"})

	app.projectCycleItems = []ProjectInfo{
		{Project: &models.Project{ID: "p1", Name: "Excel Add-in"}, TaskCount: 2},
		{Project: &models.Project{ID: "p2", Name: "SIS Shield"}, TaskCount: 21},
		{Project: &models.Project{ID: "p3", Name: "muse"}, TaskCount: 2},
		{Project: &models.Project{ID: "p4", Name: "Next Unicorn"}, TaskCount: 0},
		{Project: &models.Project{ID: "p5", Name: "LogScanner"}, TaskCount: 3},
		{Project: &models.Project{ID: "p6", Name: "ocm"}, TaskCount: 0},
		{Project: &models.Project{ID: "p7", Name: "spec"}, TaskCount: 6},
		{Project: &models.Project{ID: "p8", Name: "tt"}, TaskCount: 1},
	}

	rendered := ansi.Strip(app.renderProjectCyclePopup())
	for _, want := range []string{
		`1 \[x\] Excel Add-in\s+2`,
		`2 \[x\] SIS Shield\s+21`,
		`3 \[x\] muse\s+2`,
		`4 \[x\] Next Unicorn\s+0`,
		`5 \[ \] LogScanner\s+3`,
		`6 \[ \] ocm\s+0`,
		`7 \[ \] spec\s+6`,
		`8 \[ \] tt\s+1`,
	} {
		if !regexp.MustCompile(want).MatchString(rendered) {
			t.Fatalf("rendered popup is missing expected row %q\n\n%s", want, rendered)
		}
	}

	for _, bad := range []string{"│ 2 │", "│ 21 │", "│ 0 │", "│ 3 │", "│ 6 │", "│ 1 │"} {
		if strings.Contains(rendered, bad) {
			t.Fatalf("rendered popup still wraps counts onto standalone lines: %q\n\n%s", bad, rendered)
		}
	}
}
