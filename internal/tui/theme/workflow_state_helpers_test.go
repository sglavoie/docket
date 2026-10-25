package theme

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/models"
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

// TestStateGroupInlineBadgeStyleIsTheSectionBadgeUnpadded holds capture's
// inline badge to the section badge it borrows: the same fill and ink, and none
// of the padding, since padding would widen a value read inside a sentence.
func TestStateGroupInlineBadgeStyleIsTheSectionBadgeUnpadded(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{Name: "dark"}) })
	for _, themeName := range []string{"dark", "light"} {
		ApplyTheme(ThemeConfig{Name: themeName})
		for _, group := range append(models.AllModeWorkflowStateGroups(), models.StateGroupDone, models.StateGroupCanceled) {
			t.Run(themeName+"/"+string(group), func(t *testing.T) {
				section := GetWorkflowStateGroupStyle(group)
				badge := StateGroupInlineBadgeStyle(group)
				if badge.GetBackground() != section.GetBackground() || badge.GetForeground() != section.GetForeground() {
					t.Fatal("inline badge does not wear the section badge's fill and ink")
				}
				top, right, bottom, left := badge.GetPadding()
				if top+right+bottom+left != 0 {
					t.Fatal("inline badge carries padding")
				}
				if lipgloss.Width(badge.Render("Inbox")) != len("Inbox") {
					t.Fatal("inline badge widened the text it styles")
				}
			})
		}
	}
}
