package outliner

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

func fullscreenChecklistLines(task *models.Task, indent string, width int) []string {
	lines := inlinePeekChecklistLines(task, indent, width)
	for _, item := range task.Checklist {
		mark := "[ ] "
		if item.Done {
			mark = "[x] "
		}
		lines = append(lines, inlinePeekStyledLines(theme.MutedStyle.Render(mark), item.Text, indent+"  ", max(width-2, 1))...)
	}
	return lines
}

func fullscreenCommentSection(task *models.Task, indent string, width int) inlinePeekSection {
	section := inlinePeekSection{key: "comments:"}
	if len(task.Comments) == 0 {
		return section
	}
	section.lines = inlinePeekCommentsLines(task, indent, width)
	for i := len(task.Comments) - 1; i >= 0; i-- {
		comment := task.Comments[i]
		if comment == nil {
			continue
		}
		at := comment.CreatedAt.Local().Format(time.DateTime)
		section.key += at + "\x1e" + comment.Text + "\x1e"
		section.lines = append(section.lines, indent+theme.MutedStyle.Render(at))
		// Preserve paragraph breaks in the full reader; only the dock needs a
		// compact summary. Every line wraps rather than losing a long tail.
		for _, line := range textfit.WrapText(comment.Text, max(width-2, 1)) {
			section.lines = append(section.lines, indent+"  "+line)
		}
	}
	return section
}

func fullscreenMetadataSection(task *models.Task, indent string, width int) inlinePeekSection {
	section := inlinePeekSection{key: "metadata:"}
	add := func(label, value string) {
		if value == "" {
			return
		}
		section.key += label + "=" + value + "\x1e"
		section.lines = append(section.lines, inlinePeekStyledLines(theme.MutedStyle.Render(label+": "), value, indent, width)...)
	}
	instant := func(label string, value time.Time) {
		if !value.IsZero() {
			add(label, value.Local().Format(time.RFC3339))
		}
	}
	add("ID", fmt.Sprintf("#%d · %s", task.ShortID, task.ID))
	if task.Hidden {
		add("Hidden", "Yes")
	}
	instant("Created", task.CreatedAt)
	instant("Updated", task.UpdatedAt)
	instant("State changed", task.StateChangedAt)
	for _, entry := range []struct {
		label string
		at    *time.Time
	}{
		{"Completed", task.CompletedAt},
		{"Pinned at", task.PinnedAt},
		{"Last reviewed", task.LastReviewedAt},
		{"Deferred until", task.DeferredUntil},
	} {
		if entry.at != nil {
			instant(entry.label, *entry.at)
		}
	}
	if task.DeferCount > 0 {
		add("Deferrals", fmt.Sprint(task.DeferCount))
	}
	if task.ChecklistOrigin != nil {
		origin := task.ChecklistOrigin
		add("Checklist origin", fmt.Sprintf("%s/%s · item %d", origin.ProjectID, origin.TaskID, origin.Index+1))
	}
	var children []string
	for _, child := range task.Children {
		if child != nil {
			children = append(children, fmt.Sprintf("#%d %s · %s", child.ShortID, child.Title, child.WorkflowState))
		}
	}
	if len(children) > 0 {
		add("Subtasks", strings.Join(children, "\n"))
	}
	return section
}

func fullscreenAttachmentLines(task *models.Task, indent string, width int) []string {
	lines := inlinePeekAttachmentsLines(task, indent, width)
	for _, attachment := range task.Attachments {
		target := attachment.OriginalPath
		if target == "" {
			target = attachment.Filename
		}
		if attachment.IsCodeRef && attachment.LineNumber > 0 {
			target += fmt.Sprintf(":%d", attachment.LineNumber)
		}
		lines = append(lines, inlinePeekStyledLines(theme.MutedStyle.Render("Location: "), target, indent, width)...)
		if !attachment.AddedAt.IsZero() {
			lines = append(lines, inlinePeekStyledLines(theme.MutedStyle.Render("Added: "), attachment.AddedAt.Local().Format(time.RFC3339), indent, width)...)
		}
	}
	return lines
}
