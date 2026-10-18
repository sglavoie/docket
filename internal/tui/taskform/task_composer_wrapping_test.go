package taskform

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskComposerPastedAttachmentKeepsEndVisible(t *testing.T) {
	const attachment = `&/var/folders/p3/qvz3zdtn5ggfs6pk_6jqbwhh0000gn/T/images/Firefox\ 2026-10-18\ 10.11.38.png`
	for _, width := range []int{60, 100, 180} {
		for _, repeats := range []int{1, 2, 16, 60} {
			for _, height := range []int{12, 35} {
				t.Run(fmt.Sprintf("%dx%d/title-%d", width, height, repeats), func(t *testing.T) {
					title := strings.Repeat("Check screenshot ", repeats)
					composer := NewTaskComposerModel(TaskComposerPreset{
						Kind: ComposerProject, Project: &models.Project{ID: "project", Name: "Project"},
					}, nil)
					composer.SetSize(width, height)
					composer.Update(tea.PasteMsg{Content: title})
					_ = composer.View()
					composer.Update(tea.PasteMsg{Content: attachment})
					for _, resizedWidth := range []int{width, width, 60, 180} {
						composer.SetSize(resizedWidth, height)
						_ = composer.View()
						input := ansi.Strip(composer.Input.View())
						if !strings.Contains(input, "10.11.38.png") {
							t.Fatalf("width %d: attachment end is hidden:\n%s", resizedWidth, input)
						}
						if repeats <= 2 && height == 35 && strings.Join(strings.Fields(input), "") != strings.Join(strings.Fields(title+attachment), "") {
							t.Fatalf("a draft that fits is not fully visible:\n%s", input)
						}
						if composer.Input.Value() != title+attachment || composer.Input.Column() != len(title+attachment) {
							t.Fatal("layout changed the draft or caret")
						}
					}
				})
			}
		}
	}
}

func TestTaskComposerPastedBatchWithAttachmentShowsWholeDraft(t *testing.T) {
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind: ComposerProject, Project: &models.Project{ID: "project", Name: "Project"},
	}, nil)
	composer.SetSize(100, 35)
	draft := "First task\nCheck screenshot Check screenshot &/" + strings.Repeat("long-directory/", 5) + "image.png"
	composer.Update(tea.PasteMsg{Content: draft})
	_ = composer.View()
	input := ansi.Strip(composer.Input.View())
	if strings.Join(strings.Fields(input), "") != strings.Join(strings.Fields(draft), "") {
		t.Fatalf("pasted batch is not fully visible:\n%s", input)
	}
}
