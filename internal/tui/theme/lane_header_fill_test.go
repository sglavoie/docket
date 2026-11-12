package theme

import (
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

// Every lane header stands on a solid fill of its own lane's color in both
// themes, and the fill is neither the paper nor shared between lanes.
func TestLaneHeaderStandsOnItsOwnFillInBothThemes(t *testing.T) {
	defer ApplyTheme(ThemeConfig{})
	lanes := []models.WorkflowStateGroup{
		models.StateGroupInbox, models.StateGroupDoing, models.StateGroupWaiting,
		models.StateGroupBacklog, models.StateGroupToday, models.StateGroupNext,
		models.StateGroupPaused,
	}
	for _, name := range []string{"dark", "light"} {
		ApplyTheme(ThemeConfig{Name: name})
		seen := map[string]models.WorkflowStateGroup{}
		for _, lane := range lanes {
			fill := colorHex(LaneHeaderStyle(GetWorkflowStateGroupStyle(lane)).GetBackground())
			if fill == "" || fill == ThemeBgColor {
				t.Fatalf("%s %s: header has no fill of its own (%q on paper %s)", name, lane.Label(), fill, ThemeBgColor)
			}
			if other, dup := seen[fill]; dup {
				t.Fatalf("%s: %s and %s share fill %s", name, other.Label(), lane.Label(), fill)
			}
			seen[fill] = lane
		}
	}
}
