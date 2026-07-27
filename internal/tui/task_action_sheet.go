package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	taskActionCategoryPlan          ListCategoryID = "task-actions:plan"
	taskActionCategoryOrganize      ListCategoryID = "task-actions:organize"
	taskActionCategoryContent       ListCategoryID = "task-actions:content"
	taskActionCategoryRelationships ListCategoryID = "task-actions:relationships"
	taskActionCategoryMore          ListCategoryID = "task-actions:more"
	taskActionCategoryDanger        ListCategoryID = "task-actions:danger"
)

var taskActionCategories = []ListCategory{
	{ID: taskActionCategoryPlan, Label: "Plan"},
	{ID: taskActionCategoryOrganize, Label: "Organize"},
	{ID: taskActionCategoryContent, Label: "Content"},
	{ID: taskActionCategoryRelationships, Label: "Relationships"},
	{ID: taskActionCategoryMore, Label: "More"},
	{ID: taskActionCategoryDanger, Label: "Danger"},
}

// TaskActionSheetExecuteMsg asks the shell to execute the frozen operation
// request selected from the task action sheet.
type TaskActionSheetExecuteMsg struct {
	Request OperationExecutionRequest
}

// TaskActionSheetClosedMsg dismisses the sheet without executing an action.
type TaskActionSheetClosedMsg struct{}

// TaskActionSheetModel is a searchable, grouped ListSurface projection of the
// operations applicable to the frozen cursor task or visual selection.
type TaskActionSheetModel struct {
	surface   *ListSurfaceModel
	context   CommandContext
	taskTitle string
	targets   int
	bulk      bool
	searching bool
}

// NewTaskActionSheetModel builds a frozen action catalog for the selected
// target set. The CommandContext carries stable task and scope identities, so
// cursor changes beneath the modal cannot retarget the eventual operation.
func NewTaskActionSheetModel(
	registry *OperationRegistry,
	context CommandContext,
	taskTitle string,
) (*TaskActionSheetModel, error) {
	descriptors := TaskActionSheetDescriptors(registry, context)
	items := make([]ListItem, 0, len(descriptors))
	for _, descriptor := range descriptors {
		descriptorCopy := descriptor
		secondary := taskActionSecondaryText(descriptor)
		items = append(items, ListItem{
			ID:             ListItemID("task-action:" + string(descriptor.ID)),
			CategoryID:     taskActionCategory(descriptor),
			PrimaryText:    descriptor.Label,
			SecondaryText:  secondary,
			Aliases:        append([]string(nil), descriptor.Aliases...),
			DisabledReason: taskActionDisabledReason(descriptor, context),
			OperationIDs:   []OperationID{descriptor.ID},
			Preview: ListPreview{
				Title: descriptor.Label,
				Fields: []ListPreviewField{
					{Label: "Risk", Value: string(descriptor.Risk)},
					{Label: "Value", Value: string(descriptor.ValueProviderID)},
				},
			},
			Data: descriptorCopy,
		})
	}
	surface, err := NewListSurfaceModel(items, taskActionCategories, ListSurfaceConfig{
		SelectionMode: ListSelectionSingle,
		EmptyState: ListEmptyState{
			Title:  "No task actions available.",
			Detail: "Select a task that can be acted on.",
		},
		NoMatchState: ListEmptyState{
			Title:  "No matching actions.",
			Detail: "Clear or change the search.",
		},
	})
	if err != nil {
		return nil, err
	}
	return &TaskActionSheetModel{
		surface:   surface,
		context:   cloneCommandContext(context),
		taskTitle: strings.TrimSpace(taskTitle),
		targets:   taskActionTargetCount(context),
		bulk:      context.Selection.Visual,
	}, nil
}

// TaskActionSheetDescriptors returns the deterministic selected-task operation
// projection used by the sheet and its tests.
func TaskActionSheetDescriptors(
	registry *OperationRegistry,
	context CommandContext,
) []OperationDescriptor {
	if registry == nil || taskActionTargetCount(context) == 0 {
		return []OperationDescriptor{}
	}
	descriptors := make([]OperationDescriptor, 0)
	for _, descriptor := range registry.All() {
		if !taskActionSheetEligible(descriptor) ||
			!taskActionDescriptorApplicableToEveryTarget(descriptor, context) {
			continue
		}
		if context.Selection.Visual && !bulkTaskActionSheetEligible(descriptor) {
			continue
		}
		if _, err := (OperationTargetResolver{}).Resolve(descriptor, context); err != nil {
			continue
		}
		descriptors = append(descriptors, descriptor)
	}
	sortTaskActionDescriptors(descriptors)
	return descriptors
}

func taskActionTargetCount(context CommandContext) int {
	if context.Selection.Visual {
		return len(normalizedTaskIdentities(context.Selection.Tasks))
	}
	if currentTaskIdentity(context).Valid() {
		return 1
	}
	return 0
}

func taskActionDescriptorApplicableToEveryTarget(
	descriptor OperationDescriptor,
	context CommandContext,
) bool {
	if !descriptor.Applicable(context) {
		return false
	}
	if !context.Selection.Visual {
		return true
	}
	selected := normalizedTaskIdentities(context.Selection.Tasks)
	if len(selected) == 0 || len(context.Selection.Entities) != len(selected) {
		return false
	}
	for _, entity := range context.Selection.Entities {
		targetContext := cloneCommandContext(context)
		targetContext.Entity = entity
		if !descriptor.Applicable(targetContext) {
			return false
		}
	}
	return true
}

func bulkTaskActionSheetEligible(descriptor OperationDescriptor) bool {
	switch descriptor.ID {
	case operationTaskSetState,
		operationTaskSetPriority,
		operationTaskSetDueDate,
		operationTaskSetSnooze,
		operationTaskSetTags:
		return true
	default:
	}
	return descriptor.Risk == OperationRiskDestructive &&
		descriptor.Category == OperationCategoryTask &&
		strings.EqualFold(descriptor.Label, "delete task")
}

func taskActionSheetEligible(descriptor OperationDescriptor) bool {
	id := string(descriptor.ID)
	label := strings.ToLower(descriptor.Label)
	if strings.HasPrefix(id, "tui.core.") || descriptor.ID == operationCoreActions {
		return false
	}
	if strings.HasPrefix(label, "global ") ||
		label == "edit last task" ||
		strings.HasPrefix(label, "archive completed") ||
		strings.HasPrefix(label, "archive all completed") ||
		label == "multi-select mode" {
		return false
	}
	// The canonical state picker replaces obsolete one-state action labels.
	switch label {
	case "start task", "complete task", "block task", "standby task",
		"cancel task", "reopen task", "review task", "maybe task",
		"move to project", "manage dependencies", "move/reparent task":
		return false
	}
	if descriptor.ID == operationTaskSetState ||
		descriptor.ID == operationTaskSetPriority ||
		descriptor.ID == operationTaskSetDueDate ||
		descriptor.ID == operationTaskSetSnooze ||
		descriptor.ID == operationTaskSetTags ||
		descriptor.ID == operationTaskMoveProject ||
		descriptor.ID == operationTaskSetDependencies ||
		descriptor.ID == operationTaskSetParent {
		return true
	}
	if label == "task activity" || label == "peek task detail" {
		return true
	}
	if descriptor.Category != OperationCategoryTask {
		return false
	}
	return descriptor.TargetSource == OperationTargetCurrentTask ||
		descriptor.TargetSource == OperationTargetSelectedTasks
}

func taskActionCategory(descriptor OperationDescriptor) ListCategoryID {
	if descriptor.Risk == OperationRiskDestructive {
		return taskActionCategoryDanger
	}
	switch descriptor.ValueProviderID {
	case ValueProviderState, ValueProviderPriority, ValueProviderDueDate, ValueProviderSnooze:
		return taskActionCategoryPlan
	case ValueProviderTags, ValueProviderProject, ValueProviderParent:
		return taskActionCategoryOrganize
	case ValueProviderDependencies:
		return taskActionCategoryRelationships
	}
	label := strings.ToLower(descriptor.Label)
	switch {
	case strings.Contains(label, "dependenc"),
		strings.Contains(label, "linked"),
		strings.Contains(label, "subtask"),
		strings.Contains(label, "parent"):
		return taskActionCategoryRelationships
	case strings.Contains(label, "edit"),
		strings.Contains(label, "rename"),
		strings.Contains(label, "copy"),
		strings.Contains(label, "comment"),
		strings.Contains(label, "attachment"),
		strings.Contains(label, "decision"),
		strings.Contains(label, "activity"),
		strings.Contains(label, "detail"),
		strings.Contains(label, "note"):
		return taskActionCategoryContent
	case strings.Contains(label, "move"),
		strings.Contains(label, "tag"),
		strings.Contains(label, "dim"),
		strings.Contains(label, "pin"):
		return taskActionCategoryOrganize
	default:
		return taskActionCategoryMore
	}
}

func taskActionSecondaryText(descriptor OperationDescriptor) string {
	switch {
	case descriptor.ValueProviderID != "":
		return "choose " + strings.ReplaceAll(string(descriptor.ValueProviderID), "-", " ")
	case descriptor.Risk == OperationRiskDestructive:
		return "confirmation required"
	case descriptor.DirectKey != "":
		return descriptor.DirectKey
	default:
		return ""
	}
}

func taskActionDisabledReason(
	descriptor OperationDescriptor,
	context CommandContext,
) string {
	if descriptor.ID == operationTaskSetSnooze {
		switch context.Entity.State {
		case "in_progress":
			return "in-progress tasks stay visible"
		case "done", "canceled":
			return "terminal tasks cannot be snoozed"
		}
	}
	if descriptor.Risk == OperationRiskMutation &&
		context.Entity.Archived &&
		descriptor.ID != operationTaskMoveProject {
		return "project is archived"
	}
	return ""
}

func sortTaskActionDescriptors(descriptors []OperationDescriptor) {
	categoryRank := func(id ListCategoryID) int {
		for index, category := range taskActionCategories {
			if category.ID == id {
				return index
			}
		}
		return len(taskActionCategories)
	}
	for i := 1; i < len(descriptors); i++ {
		for j := i; j > 0; j-- {
			left, right := descriptors[j-1], descriptors[j]
			leftRank, rightRank := categoryRank(taskActionCategory(left)), categoryRank(taskActionCategory(right))
			leftLabel, rightLabel := strings.ToLower(left.Label), strings.ToLower(right.Label)
			if leftRank < rightRank ||
				(leftRank == rightRank && leftLabel < rightLabel) ||
				(leftRank == rightRank && leftLabel == rightLabel && left.ID < right.ID) {
				break
			}
			descriptors[j-1], descriptors[j] = descriptors[j], descriptors[j-1]
		}
	}
}

func cloneCommandContext(context CommandContext) CommandContext {
	context.Capabilities = cloneCapabilitySet(context.Capabilities)
	context.Selection.Tasks = append([]TaskIdentity(nil), context.Selection.Tasks...)
	context.Selection.Entities = append([]EntityContext(nil), context.Selection.Entities...)
	context.ScopeProjectIDs = append([]string(nil), context.ScopeProjectIDs...)
	return context
}

// handleKey implements the shell's compact key-interceptor contract.
func (model *TaskActionSheetModel) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil || model.surface == nil {
		return nil, true, TaskActionSheetClosedMsg{}
	}
	key := msg.String()
	if model.searching {
		switch key {
		case "esc":
			if model.surface.Query().Raw != "" {
				model.surface.SetQuery("")
				return nil, false, nil
			}
			model.searching = false
			return nil, false, nil
		case "enter":
			model.searching = false
			return nil, false, nil
		case "backspace":
			query := model.surface.Query().Raw
			if query != "" {
				_, size := utf8.DecodeLastRuneInString(query)
				model.surface.SetQuery(query[:len(query)-size])
			}
			return nil, false, nil
		default:
			if msg.Text != "" {
				model.surface.SetQuery(model.surface.Query().Raw + msg.Text)
			}
			return nil, false, nil
		}
	}

	switch key {
	case "esc":
		return nil, true, TaskActionSheetClosedMsg{}
	case "/":
		model.searching = true
	case "up", "k", "ctrl+p":
		model.surface.MoveCursor(-1)
	case "down", "j", "ctrl+n":
		model.surface.MoveCursor(1)
	case "home", "g":
		model.surface.MoveCursorToStart()
	case "end", "G":
		model.surface.MoveCursorToEnd()
	case "pgup":
		model.surface.PageCursor(-1)
	case "pgdown":
		model.surface.PageCursor(1)
	case "enter":
		item, ok := model.surface.CurrentItem()
		if !ok {
			return nil, false, nil
		}
		if !item.Enabled() {
			return nil, false, StatusMsg(item.DisabledReason)
		}
		descriptor, ok := item.Data.(OperationDescriptor)
		if !ok {
			return nil, false, StatusMsg("Action is unavailable")
		}
		return nil, false, TaskActionSheetExecuteMsg{Request: OperationExecutionRequest{
			ID:      descriptor.ID,
			Context: cloneCommandContext(model.context),
			Source:  model.invocationSource(),
		}}
	}
	return nil, false, nil
}

func (model *TaskActionSheetModel) invocationSource() OperationInvocationSource {
	if model != nil && model.bulk {
		return OperationSourceBulk
	}
	return OperationSourceActionSheet
}

func (model *TaskActionSheetModel) render(width int) string {
	if model == nil || model.surface == nil {
		return ""
	}
	innerWidth := min(max(width*3/5, 48), max(width-6, 24))
	listHeight := min(max(len(model.surface.Rows()), 7), 16)
	query := model.surface.Query().Raw
	search := "Press / to search"
	if model.searching || query != "" {
		cursor := ""
		if model.searching {
			cursor = "▏"
		}
		search = "Search: " + query + cursor
	}
	content := MutedStyle.Render(search) + "\n\n" +
		RenderCompactListSurface(model.surface, innerWidth-2, listHeight)
	titleText := "Task Actions"
	if model.bulk {
		titleText = fmt.Sprintf("Bulk Actions · %d selected", model.targets)
	}
	if model.taskTitle != "" {
		titleText += ": " + truncateToVisualWidth(model.taskTitle, max(innerWidth-20, 12))
	}
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(titleText)
	footer := HelpStyle.Render("Enter choose · / search · Esc close")
	return renderTitledBox(content, innerWidth, title, footer, lipgloss.NewStyle().Foreground(ColorPrimary))
}

func (m *AppModel) openTaskActionSheet(context CommandContext) error {
	if context.Selection.Visual {
		refs, err := m.operationTaskRefs(context)
		if err != nil {
			return err
		}
		if len(refs) == 0 {
			return fmt.Errorf("select at least one task to open bulk actions")
		}
		context.Selection.Tasks = make([]TaskIdentity, 0, len(refs))
		context.Selection.Entities = make([]EntityContext, 0, len(refs))
		for _, ref := range refs {
			context.Selection.Tasks = append(context.Selection.Tasks, ref.Identity())
			context.Selection.Entities = append(
				context.Selection.Entities,
				taskOperationEntity(ref.Task, ref.Project),
			)
		}
		context.Entity = context.Selection.Entities[0]
		sheet, err := NewTaskActionSheetModel(
			m.GetOperationRegistry(),
			context,
			fmt.Sprintf("%d task(s)", len(refs)),
		)
		if err != nil {
			return err
		}
		m.taskActionSheet = sheet
		return nil
	}
	task, _ := m.getCurrentCursorTaskAndProject()
	if task == nil {
		return fmt.Errorf("select a task to open actions")
	}
	sheet, err := NewTaskActionSheetModel(m.GetOperationRegistry(), context, task.Title)
	if err != nil {
		return err
	}
	m.taskActionSheet = sheet
	return nil
}
