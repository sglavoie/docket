package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// ListSource is the validated input shape shared by List Surface adapters and
// the unified finder providers built on top of them.
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

// ListSourceFromSelectorItems adapts the shared CLI/TUI selector rows. The first
// column remains the primary label; remaining columns are preserved as
// searchable secondary text and labeled preview fields.
func ListSourceFromSelectorItems(items []SelectorItem) ListSource {
	adapted := make([]ListItem, 0, len(items))
	for _, item := range items {
		primary := ""
		if len(item.Columns) > 0 {
			primary = item.Columns[0]
		}
		secondary := ""
		if len(item.Columns) > 1 {
			secondary = strings.Join(item.Columns[1:], " · ")
		}
		fields := make([]ListPreviewField, 0, len(item.Columns))
		for index, value := range item.Columns {
			label := fmt.Sprintf("Column %d", index+1)
			if index < len(item.Headers) && item.Headers[index] != "" {
				label = item.Headers[index]
			}
			fields = append(fields, ListPreviewField{Label: label, Value: value})
		}
		adapted = append(adapted, ListItem{
			ID:            ListItemID(item.ID),
			PrimaryText:   primary,
			SecondaryText: secondary,
			Aliases:       append([]string(nil), item.Columns...),
			Preview:       ListPreview{Title: primary, Fields: fields},
			Data:          item,
		})
	}
	return ListSource{Items: adapted}
}

// ListSourceFromOutlinerModes adapts the current unified mode catalog.
func ListSourceFromOutlinerModes(modes []OutlinerMode) ListSource {
	var categories []ListCategory
	for _, category := range []OutlinerModeCategory{ModeCategoryWork, ModeCategoryComputed, ModeCategoryReview} {
		for _, mode := range modes {
			if mode.Category == category {
				categories = append(categories, ListCategory{
					ID:    modeListCategoryID(category),
					Label: category.String(),
				})
				break
			}
		}
	}
	items := make([]ListItem, 0, len(modes))
	for _, mode := range modes {
		secondary := "Manual order"
		if mode.FixedOrder() {
			secondary = "Fixed order"
		}
		reordering := "Unavailable"
		if mode.SupportsManualReorder() {
			reordering = "Single-project sequence"
		}
		item := ListItem{
			ID:            ListItemID("mode:" + string(mode.ID)),
			CategoryID:    modeListCategoryID(mode.Category),
			PrimaryText:   mode.Label,
			SecondaryText: secondary,
			Aliases:       append([]string(nil), mode.Aliases...),
			Preview: ListPreview{
				Title: mode.Label,
				Fields: []ListPreviewField{
					{Label: "Category", Value: mode.Category.String()},
					{Label: "Ordering", Value: secondary},
					{Label: "Reordering", Value: reordering},
				},
			},
			Data: mode,
		}
		if modePickerModeUnavailable(mode) {
			item.DisabledReason = "not available in this scope"
		}
		items = append(items, item)
	}
	return ListSource{Items: items, Categories: categories}
}

// ListSourceFromContextScopes adapts the canonical scope summaries.
func ListSourceFromContextScopes(summaries []ContextScopeSummary) ListSource {
	categories := []ListCategory{
		{ID: "scope:all", Label: "Workspace"},
		{ID: "scope:custom", Label: "Custom"},
		{ID: "scope:regular", Label: "Contexts"},
	}
	items := make([]ListItem, 0, len(summaries))
	for _, summary := range summaries {
		scope := summary.Scope
		label := scope.Name
		if scope.IsAll() {
			label = "All Projects"
		}
		items = append(items, ListItem{
			ID:            contextScopeListItemID(scope),
			CategoryID:    contextScopeListCategoryID(scope.Kind),
			PrimaryText:   label,
			SecondaryText: fmt.Sprintf("%d projects", summary.ActiveProjectCount),
			Aliases:       []string{scope.Name, "scope", "projects"},
			Preview: ListPreview{
				Title: label,
				Fields: []ListPreviewField{
					{Label: "Active", Value: strconv.Itoa(summary.ActiveProjectCount)},
					{Label: "Archived", Value: strconv.Itoa(summary.ArchivedProjectCount)},
					{Label: "Hidden", Value: strconv.Itoa(summary.HiddenProjectCount)},
					{Label: "Missing", Value: strconv.Itoa(summary.StaleReferenceCount)},
				},
			},
			Data: summary,
		})
	}
	return ListSource{Items: items, Categories: categories}
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

// ListSourceFromTags adapts tag manager and picker sources.
func ListSourceFromTags(tags []TagInfo) ListSource {
	categories := []ListCategory{
		{ID: "tag:pinned", Label: "Pinned"},
		{ID: "tag:other", Label: "Tags"},
	}
	items := make([]ListItem, 0, len(tags))
	for _, tag := range tags {
		category := ListCategoryID("tag:other")
		if tag.Pinned {
			category = "tag:pinned"
		}
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
					{Label: "Pinned", Value: strconv.FormatBool(tag.Pinned)},
				},
			},
			Data: tag,
		})
	}
	return ListSource{Items: items, Categories: categories}
}

// TaskListSourceData retains the project-qualified task target required by the
// global finder and operation executor.
type TaskListSourceData struct {
	Task    *models.Task
	Project *models.Project
}

// ListSourceFromTasks adapts project task catalogs with project-qualified stable
// IDs so equal task IDs in different projects cannot collide.
func ListSourceFromTasks(projects []*models.Project) ListSource {
	categories := make([]ListCategory, 0, len(projects))
	items := make([]ListItem, 0)
	for _, project := range projects {
		if project == nil {
			continue
		}
		categoryID := ListCategoryID("tasks:" + project.ID)
		categories = append(categories, ListCategory{ID: categoryID, Label: project.Name})
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			state := task.State()
			aliases := []string{
				task.Title,
				fmt.Sprintf("#%d", task.ShortID),
				state.String(),
				project.Name,
			}
			aliases = append(aliases, task.Tags...)
			items = append(items, ListItem{
				ID:            QualifiedTaskListItemID(project.ID, task.ID),
				CategoryID:    categoryID,
				PrimaryText:   fmt.Sprintf("#%d %s", task.ShortID, task.Title),
				SecondaryText: workflowStateListLabel(state),
				Aliases:       aliases,
				Preview: ListPreview{
					Title:   task.Title,
					Summary: task.Description,
					Fields: []ListPreviewField{
						{Label: "Project", Value: project.Name},
						{Label: "State", Value: state.String()},
						{Label: "Priority", Value: strconv.Itoa(task.Priority)},
						{Label: "Tags", Value: strings.Join(task.Tags, ", ")},
					},
				},
				Data: TaskListSourceData{Task: task, Project: project},
			})
		}
	}
	return ListSource{Items: items, Categories: categories}
}

// QualifiedTaskListItemID returns the stable project-qualified identity shared
// by task adapters and future finder destinations.
func QualifiedTaskListItemID(projectID, taskID string) ListItemID {
	return ListItemID("task:" + projectID + ":" + taskID)
}

func modeListCategoryID(category OutlinerModeCategory) ListCategoryID {
	switch category {
	case ModeCategoryComputed:
		return "mode:computed"
	case ModeCategoryReview:
		return "mode:review"
	default:
		return "mode:work"
	}
}

func contextScopeListItemID(scope ContextScope) ListItemID {
	switch scope.Kind {
	case ContextScopeCustom:
		return ListItemID("scope:custom:" + scope.Name)
	case ContextScopeRegular:
		return ListItemID("scope:regular:" + scope.Name)
	default:
		return "scope:all"
	}
}

func contextScopeListCategoryID(kind ContextScopeKind) ListCategoryID {
	switch kind {
	case ContextScopeCustom:
		return "scope:custom"
	case ContextScopeRegular:
		return "scope:regular"
	default:
		return "scope:all"
	}
}

func workflowStateListLabel(state models.WorkflowState) string {
	switch state {
	case models.StateInbox:
		return "Inbox"
	case models.StateReady:
		return "Ready"
	case models.StateNext:
		return "Next"
	case models.StateInProgress:
		return "In Progress"
	case models.StateWaiting:
		return "Waiting"
	case models.StatePaused:
		return "Paused"
	case models.StateReview:
		return "Review"
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
