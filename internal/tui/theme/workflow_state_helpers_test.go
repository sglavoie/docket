package theme

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestWorkflowStateGroupThemeSemanticsCoverEveryGroup(t *testing.T) {
	groups := append(models.AllModeWorkflowStateGroups(), models.StateGroupDone, models.StateGroupCanceled)
	for _, group := range groups {
		label := group.Label()
		if got := GetWorkflowStateGroupStyle(group).Render(label); got == "" {
			t.Errorf("group %q rendered an empty header", group)
		}
	}
}
