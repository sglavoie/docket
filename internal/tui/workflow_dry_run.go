package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// DryRunTaskResult holds the evaluation result for a single task.
type DryRunTaskResult struct {
	Task         *models.Task    // original (unmodified) task
	Project      *models.Project // owning project; read-only during dry-run display
	MatchedRules []string        // names of rules that matched
	Actions      []string        // human-readable action descriptions
	Changes      []DryRunChange  // specific field changes
}

// DryRunChange describes a single field change from the dry run.
type DryRunChange struct {
	Field    string
	OldValue string
	NewValue string
}

// WorkflowDryRunModel displays dry run results for workflow rules.
type WorkflowDryRunModel struct {
	BaseSearchableView
	// dataDir is the workspace root used to resolve the active-project set through
	// the catalog domain; configManager backs the automation-domain rule read.
	dataDir       string
	configManager *config.Manager
	persister     Persister

	InteractiveViewState

	results     []DryRunTaskResult
	filtered    []DryRunTaskResult
	conflicts   []workflow.RuleConflict
	singleRule  *workflow.WorkflowRule
	ruleCount   int
	taskCount   int
	statusLabel string
	loading     bool
	loadErr     error
}

// ShowWorkflowDryRunMsg triggers the dry run view.
type ShowWorkflowDryRunMsg struct {
	SingleRule *workflow.WorkflowRule // nil = test all enabled rules
}

// NewWorkflowDryRunModel creates a new dry run model. It depends only on the
// workspace dataDir (catalog domain) and the manager (automation domain) rather
// than the config.Config facade: enabled rules load from automation.json and the
// active-project set from the workspace catalog.
func NewWorkflowDryRunModel(dataDir string, manager *config.Manager, persister Persister, singleRule *workflow.WorkflowRule) *WorkflowDryRunModel {
	m := &WorkflowDryRunModel{
		dataDir:       dataDir,
		configManager: manager,
		persister:     persister,
		singleRule:    singleRule,
		loading:       true,
	}
	m.SetSize(80, 24)
	m.InitSearchInput("Search tasks...", 50, 30)
	return m
}

func (m *WorkflowDryRunModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
}

func (m *WorkflowDryRunModel) Init() tea.Cmd {
	return m.runDryRun
}

// runDryRun performs the actual evaluation against deep-copied tasks.
func (m *WorkflowDryRunModel) runDryRun() tea.Msg {
	if m.configManager == nil {
		return dryRunDoneMsg{err: fmt.Errorf("config manager unavailable")}
	}
	deps := m.configManager.RuntimeDependencies()

	var rules []workflow.WorkflowRule
	if m.singleRule != nil {
		rule := *m.singleRule
		rule.Enabled = true
		rules = []workflow.WorkflowRule{rule}
	} else {
		automation, err := deps.Automation.LoadAutomation()
		if err != nil {
			return dryRunDoneMsg{err: err}
		}
		rules = automation.GetEnabledWorkflowRules()
	}

	if len(rules) == 0 {
		return dryRunDoneMsg{err: fmt.Errorf("no enabled workflow rules")}
	}

	projects, err := deps.ActiveProjects(m.dataDir)
	if err != nil {
		return dryRunDoneMsg{err: err}
	}
	var allResults []DryRunTaskResult
	var allConflicts []workflow.RuleConflict
	totalTasks := 0

	for _, project := range projects {
		if err := m.persister.Load(project); err != nil {
			continue
		}
		if len(project.Tasks) == 0 {
			continue
		}

		totalTasks += len(project.Tasks)

		// Deep-copy tasks so the engine doesn't mutate originals
		originals := make(map[string]*models.Task, len(project.Tasks))
		copies := make([]*models.Task, len(project.Tasks))
		for i, task := range project.Tasks {
			originals[task.ID] = task
			copies[i] = deepCopyTaskForDryRun(task)
		}

		// Build task tree on copies for hierarchical triggers
		buildTaskTreeForCopies(copies)

		evalCtx := &workflow.EvaluationContext{
			Changes:     buildChanges(copies),
			AllTasks:    copies,
			TaskMap:     buildTaskMap(copies),
			ProjectName: project.Name,
			ProjectID:   project.ID,
		}

		result := workflow.EvaluateRulesWithContext(evalCtx, rules)

		// Build results from modified tasks
		for _, modifiedCopy := range result.ModifiedTasks {
			original := originals[modifiedCopy.ID]
			if original == nil {
				continue
			}

			ruleNames := result.AppliedRules[modifiedCopy.ID]
			changes := detectChanges(original, modifiedCopy)
			if len(changes) == 0 {
				continue
			}

			var actionDescs []string
			for _, c := range changes {
				actionDescs = append(actionDescs, fmt.Sprintf("%s: %s → %s", c.Field, c.OldValue, c.NewValue))
			}

			allResults = append(allResults, DryRunTaskResult{
				Task:         original,
				Project:      project,
				MatchedRules: ruleNames,
				Actions:      actionDescs,
				Changes:      changes,
			})
		}

		allConflicts = append(allConflicts, result.Conflicts...)
	}

	return dryRunDoneMsg{
		results:   allResults,
		conflicts: allConflicts,
		ruleCount: len(rules),
		taskCount: totalTasks,
	}
}

type dryRunDoneMsg struct {
	results   []DryRunTaskResult
	conflicts []workflow.RuleConflict
	ruleCount int
	taskCount int
	err       error
}

// deepCopyTaskForDryRun creates a copy of a task suitable for dry-run evaluation.
// Copies all fields that the engine might read or modify.
func deepCopyTaskForDryRun(task *models.Task) *models.Task {
	if task == nil {
		return nil
	}

	copy := &models.Task{
		ID:          task.ID,
		ShortID:     task.ShortID,
		Type:        task.Type,
		Title:       task.Title,
		Description: task.Description,
		// The workflow engine now reads task lifecycle through State()/WorkflowState
		// (33.7.6.4), so only the canonical WorkflowState is carried — the retired
		// Status/Section clones are gone. It is seeded from State() (not the raw
		// WorkflowState field, which is empty for schema-v1 tasks) so the copy's
		// State() resolves identically to the original and stays coherent after the
		// engine's lifecycle mutators sync it. The engine also does not read Active
		// (ActionSetActive became a Focus Queue change in 33.7.6.1).
		WorkflowState: task.State(),
		Priority:      task.Priority,
		CreatedAt:     task.CreatedAt,
		UpdatedAt:     task.UpdatedAt,
		HasTime:       task.HasTime,
		Pinned:        task.Pinned,
		Dimmed:        task.Dimmed,
		Level:         task.Level,
		Source:        task.Source,
		URL:           task.URL,
	}

	// Copy pointer fields
	if task.ParentID != nil {
		pid := *task.ParentID
		copy.ParentID = &pid
	}
	if task.DueDate != nil {
		d := *task.DueDate
		copy.DueDate = &d
	}
	if task.CompletedAt != nil {
		c := *task.CompletedAt
		copy.CompletedAt = &c
	}
	if task.PinnedAt != nil {
		p := *task.PinnedAt
		copy.PinnedAt = &p
	}
	if task.SnoozedUntil != nil {
		s := *task.SnoozedUntil
		copy.SnoozedUntil = &s
	}

	// Copy slices
	if len(task.Tags) > 0 {
		copy.Tags = make([]string, len(task.Tags))
		for i, t := range task.Tags {
			copy.Tags[i] = t
		}
	}
	if len(task.DependsOn) > 0 {
		copy.DependsOn = make([]string, len(task.DependsOn))
		for i, d := range task.DependsOn {
			copy.DependsOn[i] = d
		}
	}
	if len(task.RelatedTo) > 0 {
		copy.RelatedTo = make([]string, len(task.RelatedTo))
		for i, r := range task.RelatedTo {
			copy.RelatedTo[i] = r
		}
	}

	// Copy recurrence
	if task.Recurrence != nil {
		rec := *task.Recurrence
		if len(task.Recurrence.Weekdays) > 0 {
			rec.Weekdays = make([]int, len(task.Recurrence.Weekdays))
			for i, w := range task.Recurrence.Weekdays {
				rec.Weekdays[i] = w
			}
		}
		copy.Recurrence = &rec
	}

	// Copy comments (shallow — engine only appends)
	if len(task.Comments) > 0 {
		copy.Comments = make([]*models.Comment, len(task.Comments))
		for i, c := range task.Comments {
			copy.Comments[i] = c
		}
	}

	return copy
}

// buildTaskTreeForCopies rebuilds parent/child relationships for deep-copied tasks.
func buildTaskTreeForCopies(tasks []*models.Task) {
	taskMap := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		task.Children = []*models.Task{}
		task.Parent = nil
		task.Level = 0
		taskMap[task.ID] = task
	}

	for _, task := range tasks {
		if task.ParentID != nil {
			if parent, ok := taskMap[*task.ParentID]; ok {
				task.Parent = parent
				parent.Children = append(parent.Children, task)
			}
		}
	}

	var setLevels func(task *models.Task, level int)
	setLevels = func(task *models.Task, level int) {
		task.Level = level
		for _, child := range task.Children {
			setLevels(child, level+1)
		}
	}

	for _, task := range tasks {
		if task.ParentID == nil {
			setLevels(task, 0)
		}
	}
}

// buildChanges creates TaskChangeContext entries for all tasks (no previous state = no event triggers).
func buildChanges(tasks []*models.Task) []workflow.TaskChangeContext {
	changes := make([]workflow.TaskChangeContext, len(tasks))
	for i, task := range tasks {
		changes[i] = workflow.TaskChangeContext{Task: task, Previous: nil}
	}
	return changes
}

// buildTaskMap creates a task ID to task map.
func buildTaskMap(tasks []*models.Task) map[string]*models.Task {
	m := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		m[task.ID] = task
	}
	return m
}

// detectChanges compares original and modified tasks, returning field-level differences.
func detectChanges(original, modified *models.Task) []DryRunChange {
	var changes []DryRunChange

	// Lifecycle changes are reported as a single canonical WorkflowState
	// transition rather than separate status/section fields (the engine still
	// mutates the legacy shims, but State() collapses them to one truth).
	if original.State() != modified.State() {
		changes = append(changes, DryRunChange{
			Field: "state", OldValue: string(original.State()), NewValue: string(modified.State()),
		})
	}
	if original.Priority != modified.Priority {
		changes = append(changes, DryRunChange{
			Field: "priority", OldValue: fmt.Sprintf("%d", original.Priority), NewValue: fmt.Sprintf("%d", modified.Priority),
		})
	}
	if original.Type != modified.Type {
		changes = append(changes, DryRunChange{
			Field: "type", OldValue: string(original.Type), NewValue: string(modified.Type),
		})
	}
	if original.Dimmed != modified.Dimmed {
		changes = append(changes, DryRunChange{
			Field: "dimmed", OldValue: fmt.Sprintf("%t", original.Dimmed), NewValue: fmt.Sprintf("%t", modified.Dimmed),
		})
	}
	if original.Pinned != modified.Pinned {
		changes = append(changes, DryRunChange{
			Field: "pinned", OldValue: fmt.Sprintf("%t", original.Pinned), NewValue: fmt.Sprintf("%t", modified.Pinned),
		})
	}

	// Tag changes
	origTags := strings.Join(original.Tags, ", ")
	modTags := strings.Join(modified.Tags, ", ")
	if origTags != modTags {
		if origTags == "" {
			origTags = "(none)"
		}
		if modTags == "" {
			modTags = "(none)"
		}
		changes = append(changes, DryRunChange{
			Field: "tags", OldValue: origTags, NewValue: modTags,
		})
	}

	// Due date changes
	origDue := "(none)"
	modDue := "(none)"
	if original.DueDate != nil {
		origDue = original.DueDate.Format("2006-01-02")
		origDue = AppendTimeIfHasTime(original, origDue, "15:04")
	}
	if modified.DueDate != nil {
		modDue = modified.DueDate.Format("2006-01-02")
		modDue = AppendTimeIfHasTime(modified, modDue, "15:04")
	}
	if origDue != modDue {
		changes = append(changes, DryRunChange{
			Field: "due_date", OldValue: origDue, NewValue: modDue,
		})
	}

	// Comment additions
	if len(modified.Comments) > len(original.Comments) {
		for _, c := range modified.Comments[len(original.Comments):] {
			changes = append(changes, DryRunChange{
				Field: "comment", OldValue: "", NewValue: c.Text,
			})
		}
	}

	// Snooze changes
	origSnooze := "(none)"
	modSnooze := "(none)"
	if original.SnoozedUntil != nil {
		origSnooze = original.SnoozedUntil.Format("2006-01-02")
	}
	if modified.SnoozedUntil != nil {
		modSnooze = modified.SnoozedUntil.Format("2006-01-02")
	}
	if origSnooze != modSnooze {
		changes = append(changes, DryRunChange{
			Field: "snoozed_until", OldValue: origSnooze, NewValue: modSnooze,
		})
	}

	// Recurrence changes (ActionSetRecurrence mutates this field).
	origRec := describeRecurrence(original.Recurrence)
	modRec := describeRecurrence(modified.Recurrence)
	if origRec != modRec {
		changes = append(changes, DryRunChange{
			Field: "recurrence", OldValue: origRec, NewValue: modRec,
		})
	}

	return changes
}

// describeRecurrence renders a recurrence pattern for diff display.
func describeRecurrence(r *models.RecurrencePattern) string {
	if r == nil {
		return "(none)"
	}
	return r.String()
}

// filterResults filters results based on search query.
func (m *WorkflowDryRunModel) filterResults() {
	query := strings.ToLower(m.searchQuery)
	if query == "" {
		m.filtered = m.results
	} else {
		m.filtered = nil
		for _, r := range m.results {
			titleLower := strings.ToLower(r.Task.Title)
			projectLower := strings.ToLower(dryRunProjectName(r.Project))
			if strings.Contains(titleLower, query) || strings.Contains(projectLower, query) {
				m.filtered = append(m.filtered, r)
			}
		}
	}

	if m.cursor >= len(m.filtered) {
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		} else {
			m.cursor = 0
		}
	}
}

func dryRunProjectName(project *models.Project) string {
	if project == nil {
		return ""
	}
	return project.Name
}

func (m *WorkflowDryRunModel) HasTextInput() bool     { return m.searchActive }
func (m *WorkflowDryRunModel) StatusBarLabel() string { return m.statusLabel }
func (m *WorkflowDryRunModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() || m.SearchShouldCaptureEsc() || m.searchQuery != ""
}

// isWideScreen returns true if the screen is wide enough for split layout.
func (m *WorkflowDryRunModel) isWideScreen() bool {
	return m.Width() >= 81
}

// GetHelpItems returns help items for this view.
func (m *WorkflowDryRunModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewWorkflowDryRun)
}
