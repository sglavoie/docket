package tui

import (
	"fmt"
	"strconv"

	"github.com/sglavoie/tt/internal/models"
)

// ListSource is the validated input shape shared by List Surface adapters and
// the retained list surfaces built on top of them.
type ListSource struct {
	Items      []ListItem
	Categories []ListCategory
}

// NewModel creates a ListSurfaceModel from the adapted source.
func (source ListSource) NewModel(config ListSurfaceConfig) (*ListSurfaceModel, error) {
	return NewListSurfaceModel(source.Items, source.Categories, config)
}

// ListSurfaceDisabledPickerItem optionally exposes why a legacy picker item is
// visible but unavailable.
type ListSurfaceDisabledPickerItem interface {
	ListSurfaceDisabledReason() string
}

// ListSurfacePreviewPickerItem optionally supplies rich preview content.
type ListSurfacePreviewPickerItem interface {
	ListSurfacePreview() ListPreview
}

// ListSurfaceOperationPickerItem optionally supplies contextual operation IDs.
type ListSurfaceOperationPickerItem interface {
	ListSurfaceOperationIDs() []OperationID
}

// ListSourceFromPickerItems adapts legacy PickerItem values without changing
// their ordering, display text, payload identity, or filtering behavior.
func ListSourceFromPickerItems[T PickerItem](items []T, showCounts bool) ListSource {
	adapted := make([]ListItem, 0, len(items))
	for _, item := range items {
		primary := item.DisplayText()
		if pinnable, ok := any(item).(PinnablePickerItem); ok && pinnable.IsPinned() {
			primary = PinnedProjectStyle.Render(primary)
		}
		secondary := ""
		if showCounts {
			if count, ok := any(item).(CountDisplayer); ok {
				secondary = count.CountDisplayText()
			} else {
				secondary = fmt.Sprintf("(%d)", item.CountValue())
			}
		}
		adaptedItem := ListItem{
			ID:            pickerItemListID(item),
			PrimaryText:   primary,
			SecondaryText: secondary,
			Aliases:       []string{item.FilterText()},
			Data:          item,
		}
		if disabled, ok := any(item).(ListSurfaceDisabledPickerItem); ok {
			adaptedItem.DisabledReason = disabled.ListSurfaceDisabledReason()
		}
		if preview, ok := any(item).(ListSurfacePreviewPickerItem); ok {
			adaptedItem.Preview = preview.ListSurfacePreview()
		}
		if operations, ok := any(item).(ListSurfaceOperationPickerItem); ok {
			adaptedItem.OperationIDs = append(
				[]OperationID(nil),
				operations.ListSurfaceOperationIDs()...,
			)
		}
		adapted = append(adapted, adaptedItem)
	}
	return ListSource{Items: adapted}
}

func pickerItemListID(item PickerItem) ListItemID {
	if id := item.ItemID(); id != "" {
		return ListItemID(id)
	}
	return "picker:none"
}

// ListSourceFromWorkflowStates adapts canonical lifecycle states.
func ListSourceFromWorkflowStates(states []models.WorkflowState) ListSource {
	items := make([]ListItem, 0, len(states))
	for _, state := range states {
		label := workflowStateListLabel(state)
		items = append(items, ListItem{
			ID:            ListItemID("state:" + state.String()),
			PrimaryText:   state.Icon() + " " + label,
			SecondaryText: state.String(),
			Aliases:       []string{state.String(), label},
			Preview: ListPreview{
				Title: label,
				Fields: []ListPreviewField{
					{Label: "State", Value: state.String()},
					{Label: "Terminal", Value: strconv.FormatBool(state.IsTerminal())},
				},
			},
			Data: state,
		})
	}
	return ListSource{Items: items}
}

// ListSourceFromPriorities adapts the established priority picker options.
func ListSourceFromPriorities(priorities []PriorityInfo) ListSource {
	items := make([]ListItem, 0, len(priorities))
	for _, priority := range priorities {
		items = append(items, ListItem{
			ID:            ListItemID("priority:" + strconv.Itoa(priority.Priority)),
			PrimaryText:   fmt.Sprintf("Priority %d", priority.Priority),
			SecondaryText: priority.Description,
			Aliases: []string{
				strconv.Itoa(priority.Priority),
				priority.Description,
				priority.FilterText(),
			},
			Preview: ListPreview{
				Title: fmt.Sprintf("Priority %d", priority.Priority),
				Body:  priority.Description,
			},
			Data: priority,
		})
	}
	return ListSource{Items: items}
}

// ListSourceFromProjects adapts project catalogs while retaining archived and
// pinned presentation groups.
func ListSourceFromProjects(projects []*models.Project) ListSource {
	categories := []ListCategory{
		{ID: "project:pinned", Label: "Pinned"},
		{ID: "project:active", Label: "Projects"},
		{ID: "project:archived", Label: "Archived"},
	}
	items := make([]ListItem, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		category := ListCategoryID("project:active")
		if project.Archived {
			category = "project:archived"
		} else if project.Pinned {
			category = "project:pinned"
		}
		items = append(items, ListItem{
			ID:            ListItemID("project:" + project.ID),
			CategoryID:    category,
			PrimaryText:   projectPickerDisplayText(project),
			SecondaryText: fmt.Sprintf("%s · %d tasks", project.Context, project.CountTasks()),
			Aliases:       []string{project.Name, project.Context, project.Slug},
			Preview: ListPreview{
				Title:   project.Name,
				Summary: project.Description,
				Fields: []ListPreviewField{
					{Label: "Context", Value: project.Context},
					{Label: "Tasks", Value: strconv.Itoa(project.CountTasks())},
				},
			},
			Data: project,
		})
	}
	return ListSource{Items: items, Categories: categories}
}

// ListSourceFromTags adapts task-tag picker sources.
func ListSourceFromTags(tags []TagInfo) ListSource {
	categories := []ListCategory{
		{ID: "tag:other", Label: "Tags"},
	}
	items := make([]ListItem, 0, len(tags))
	for _, tag := range tags {
		category := ListCategoryID("tag:other")
		items = append(items, ListItem{
			ID:            ListItemID("tag:" + tag.Name),
			CategoryID:    category,
			PrimaryText:   tag.Name,
			SecondaryText: fmt.Sprintf("%d tasks", tag.TaskCount),
			Aliases:       []string{tag.Name, "#" + tag.Name},
			Preview: ListPreview{
				Title: tag.Name,
				Fields: []ListPreviewField{
					{Label: "Tasks", Value: strconv.Itoa(tag.TaskCount)},
				},
			},
			Data: tag,
		})
	}
	return ListSource{Items: items, Categories: categories}
}

func contextScopeListItemID(scope ContextScope) ListItemID {
	switch scope.Kind {
	case ContextScopeRegular:
		return ListItemID("scope:regular:" + scope.Name)
	default:
		return "scope:all"
	}
}

func workflowStateListLabel(state models.WorkflowState) string {
	switch state {
	case models.StateInbox:
		return "Inbox"
	case models.StateBacklog:
		return "Backlog"
	case models.StateDoing:
		return "Doing"
	case models.StateWaiting:
		return "Waiting"
	case models.StatePaused:
		return "Paused"
	case models.StateSomeday:
		return "Someday"
	case models.StateDone:
		return "Done"
	case models.StateCanceled:
		return "Canceled"
	default:
		return state.String()
	}
}
