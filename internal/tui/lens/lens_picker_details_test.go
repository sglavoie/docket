package lens

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
)

func TestLensDetailsWrapPageAndFollowSelectionWithoutApplying(t *testing.T) {
	model, err := NewNamedLensPickerModel(operation.NewOperationExecutor(operation.NewOperationRegistry()),
		presentation.ViewCrossProjectOutliner, NamedLensPickerSnapshot{
			Lenses: config.NamedLensList{
				{ID: "long", Name: "Release", Mode: config.LensModeAll, Scope: "web",
					Query: strings.Repeat("界 release planning ", 35) + "TAILMARKER",
					Sort:  config.LensSort{Field: config.LensSortPriority}},
				{ID: "short", Name: "Waiting", Mode: config.LensModeAll, Query: "$waiting"},
			},
			ProjectSlugs: map[string]string{"web": "website"},
		})
	if err != nil {
		t.Fatal(err)
	}
	model.SelectLens("long")
	for _, size := range [][2]int{{60, 20}, {100, 35}, {60, 20}} {
		model.detailOffset = 0
		got := ansi.Strip(model.RenderWithin(size[0], size[1]))
		if !strings.Contains(got, "Project: website") || !strings.Contains(got, "Ctrl+F/B page") {
			t.Fatalf("missing details or paging guidance:\n%s", got)
		}
		for range 100 {
			cmd, closed, msg := model.HandleKey(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
			if cmd != nil || closed || msg != nil {
				t.Fatal("paging applied or dismissed the lens")
			}
			got = ansi.Strip(model.RenderWithin(size[0], size[1]))
			if lipgloss.Width(got) > size[0] || lipgloss.Height(got) > size[1] {
				t.Fatalf("details exceed %v:\n%s", size, got)
			}
			if strings.Contains(got, "TAILMARKER") && strings.Contains(got, "sort:") {
				break
			}
		}
		if !strings.Contains(got, "TAILMARKER") || !strings.Contains(got, "sort:") {
			t.Fatalf("definition tail is unreachable:\n%s", got)
		}
	}
	model.HandleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	got := ansi.Strip(model.RenderWithin(60, 20))
	if model.detailOffset != 0 || !strings.Contains(got, "All · $waiting") || strings.Contains(got, "above") {
		t.Fatalf("new selection retained old details:\n%s", got)
	}
}
