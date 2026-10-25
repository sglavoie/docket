package listsurface

import (
	"fmt"
	"strconv"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/util"
)

// The concrete ListSource constructors. Each adapts one established picker
// vocabulary into the validated shape list_surface_adapters.go declares, without
// changing that vocabulary's ordering, display text, payload identity, or
// filtering behavior.

// ListSourceFromPickerItems adapts legacy PickerItem values without changing
// their ordering, display text, payload identity, or filtering behavior.
func ListSourceFromPickerItems[T PickerItem](items []T, showCounts bool) ListSource {
	adapted := make([]ListItem, 0, len(items))
	for _, item := range items {
		primary := item.DisplayText()
		marker := ""
		if pinnable, ok := any(item).(PinnablePickerItem); ok && pinnable.IsPinned() {
			marker = PinnedMarker()
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
			PrimaryMarker: marker,
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
				[]operation.OperationID(nil),
				operations.ListSurfaceOperationIDs()...,
			)
		}
		if styled, ok := any(item).(ListSurfaceStyledPickerItem); ok {
			style := styled.ListSurfacePrimaryStyle()
			adaptedItem.PrimaryStyle = &style
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
func ListSourceFromPriorities(priorities []chrome.PriorityInfo) ListSource {
	items := make([]ListItem, 0, len(priorities))
	for _, priority := range priorities {
		items = append(items, ListItem{
			ID:            ListItemID("priority:" + strconv.Itoa(priority.Priority)),
			PrimaryText:   priority.Description,
			SecondaryText: fmt.Sprintf("!%d", priority.Priority),
			Aliases: []string{
				strconv.Itoa(priority.Priority),
				priority.Description,
				priority.FilterText(),
			},
			Preview: ListPreview{
				Title: priority.Description,
				Body:  fmt.Sprintf("Priority !%d", priority.Priority),
			},
			Data: priority,
		})
	}
	return ListSource{Items: items}
}

// ListSourceFromTags adapts task-tag picker sources.
func ListSourceFromTags(tags []query.TagInfo) ListSource {
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
			SecondaryText: util.Count(tag.TaskCount, "task"),
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

// CountDisplayer is implemented by PickerItems that want custom styled count
// text instead of the default "(N)" format. The returned string is pre-rendered
// and should not be wrapped in additional styles.
type CountDisplayer interface {
	CountDisplayText() string
}
