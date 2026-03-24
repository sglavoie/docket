package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/workflow"
)

func TestTruncateToVisualWidthUsesDisplayWidth(t *testing.T) {
	truncated := truncateToVisualWidth("修复界面布局问题", 8)

	if lipgloss.Width(truncated) > 8 {
		t.Fatalf("truncateToVisualWidth exceeded width: %q (width=%d)", truncated, lipgloss.Width(truncated))
	}
	if !strings.HasSuffix(truncated, "…") {
		t.Fatalf("expected ellipsis for truncated text, got %q", truncated)
	}
	if lipgloss.Width("修复界面") != 8 {
		t.Fatalf("test assumption failed: expected sample text width 8")
	}
}

func TestProjectSelectorViewRespectsVisualWidth(t *testing.T) {
	sourceProject, err := models.NewProject("source", "Source Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject source: %v", err)
	}

	task, err := models.NewTask("task-1", 42, models.TaskTypeTask, "修复跨项目移动选择器的宽字符截断问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	projectA, err := models.NewProject("proj-a", "ui-fix-sprint", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject A: %v", err)
	}
	projectA.Name = "界面修复冲刺计划项目"
	projectA.Context = "work"

	projectB, err := models.NewProject("proj-b", "Release coordination", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject B: %v", err)
	}
	projectB.Context = "ops"

	m := NewProjectSelectorModel([]*models.Project{sourceProject, projectA, projectB}, task, sourceProject)
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(line) > 32 {
			t.Fatalf("project selector line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(line), ansi.Strip(rendered))
		}
	}

	if !strings.Contains(ansi.Strip(rendered), "界面修复") {
		t.Fatalf("expected rendered selector to include the unicode project name, got:\n%s", ansi.Strip(rendered))
	}
	if !strings.Contains(ansi.Strip(rendered), "Moving: #42") {
		t.Fatalf("expected task header in selector view, got:\n%s", ansi.Strip(rendered))
	}
}

func TestRenderScoreCardContentAlignsValueColumnByVisualWidth(t *testing.T) {
	profile := &scoring.ScoringProfile{
		ID:      "weighted",
		Name:    "Weighted",
		Formula: scoring.FormulaWeightedSum,
		Dimensions: []scoring.ScoringDimension{
			{ID: "impact", Name: "Impact", Min: 1, Max: 5, Default: 3, Weight: 1, Source: scoring.SourceManual},
			{ID: "reach", Name: "修复范围广度", Min: 1, Max: 5, Default: 3, Weight: 1, Source: scoring.SourceManual},
		},
	}

	task, err := models.NewTask("task-1", 7, models.TaskTypeTask, "Align score badge rows")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.ScoreDimensions = map[string]int{"impact": 3, "reach": 4}

	rendered := renderScoreCardContent(task, profile, 28)
	lines := strings.Split(rendered, "\n")

	for _, line := range lines {
		if lipgloss.Width(line) > 28 {
			t.Fatalf("score card line exceeds width 28: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(line), ansi.Strip(rendered))
		}
	}

	strippedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		strippedLines = append(strippedLines, ansi.Strip(line))
	}

	impactPrefixWidth := prefixWidthBeforeToken(t, strippedLines, "Impact", " 3 ")
	reachPrefixWidth := prefixWidthBeforeToken(t, strippedLines, "修复范围广度", " 4 ")

	if impactPrefixWidth != reachPrefixWidth {
		t.Fatalf("expected aligned value columns, got impact=%d reach=%d\n\n%s", impactPrefixWidth, reachPrefixWidth, strings.Join(strippedLines, "\n"))
	}
}

func TestGlobalSearchNoteRowRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = "漢字wide"

	note := &models.Note{Title: "漢字wide note title that should truncate cleanly"}

	m := &GlobalSearchModel{
		filtered: []filteredSearchResult{{
			result: models.SearchResult{Project: project, Note: note},
		}},
		listWidth: 24,
		cursor:    0,
	}

	rendered := m.renderTaskList(6)
	for line := range strings.SplitSeq(strings.TrimRight(rendered, "\n"), "\n") {
		if lipgloss.Width(ansi.Strip(line)) > m.listWidth {
			t.Fatalf("global search note row exceeds width %d: %q (width=%d)\n\n%s", m.listWidth, ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestTaskLineRenderRespectsVisualWidthWithDueColumn(t *testing.T) {
	task, err := models.NewTask("task-1", 9, models.TaskTypeTask, "修复跨项目任务行在窄宽度下的对齐问题")
	if err != nil {
		fatalf := t.Fatalf
		fatalf("NewTask: %v", err)
	}

	line := TaskLine{
		Task:      task,
		PreBadges: MutedStyle.Render("[界面修复项目] "),
		PostTitle: MutedStyle.Render(" · tag"),
		DueStr:    MutedStyle.Render("2d"),
		ViewWidth: 28,
		MaxTitle:  12,
	}.Render()

	if lipgloss.Width(ansi.Strip(line)) > 28 {
		t.Fatalf("task line exceeds width 28: %q (width=%d)", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)))
	}
}

func TestWeeklyReviewProjectLineRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "ui-fix-sprint", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Name = "界面修复冲刺计划项目"

	m := &WeeklyReviewModel{}
	m.width = 30
	line := m.renderProjectLine(
		WeeklyReviewProject{Project: project, InboxCount: 7, OverdueCount: 3, NoActivityDays: 9},
		WeeklySectionInboxBacklog,
		true,
	)

	if lipgloss.Width(ansi.Strip(line)) > 30 {
		t.Fatalf("weekly review line exceeds width 30: %q (width=%d)", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)))
	}
}

func TestLinkedTaskSelectorRelationLineRespectsVisualWidth(t *testing.T) {
	sourceProject, err := models.NewProject("source", "source-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject source: %v", err)
	}
	targetProject, err := models.NewProject("target", "target-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject target: %v", err)
	}
	targetProject.Name = "界面修复冲刺计划项目"

	task, err := models.NewTask("task-1", 12, models.TaskTypeTask, "修复跨项目链接列表在窄宽度下的标题截断问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Status = models.StatusInProgress

	m := &LinkedTaskSelectorModel{sourceTask: task, project: sourceProject}
	m.SetSize(32, 12)

	line := "> " + m.renderRelationLine(LinkedTaskRelation{Task: task, Project: targetProject}, false)
	if lipgloss.Width(ansi.Strip(line)) > 32 {
		t.Fatalf("linked task selector line exceeds width 32: %q (width=%d)", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)))
	}
	if !strings.Contains(ansi.Strip(line), "界面修复") {
		t.Fatalf("expected cross-project label to remain visible, got %q", ansi.Strip(line))
	}
}

func TestCrossProjectLinkerTaskSelectionRespectsVisualWidth(t *testing.T) {
	sourceProject, err := models.NewProject("source", "source-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject source: %v", err)
	}
	sourceTask, err := models.NewTask("source-task", 7, models.TaskTypeTask, "修复跨项目链接器在窄宽度下的布局")
	if err != nil {
		t.Fatalf("NewTask source: %v", err)
	}
	targetProject, err := models.NewProject("target", "target-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject target: %v", err)
	}
	targetTask, err := models.NewTask("target-task", 21, models.TaskTypeTask, "修复目标任务列表在窄视图中的宽字符截断问题")
	if err != nil {
		t.Fatalf("NewTask target: %v", err)
	}
	targetTask.Status = models.StatusInProgress

	m := &CrossProjectLinkerModel{
		sourceTask:    sourceTask,
		sourceProject: sourceProject,
		step:          StepSelectTask,
		targetProject: targetProject,
		filteredTasks: []*models.Task{targetTask},
	}
	m.SetSize(32, 14)

	rendered := m.viewTaskSelection()
	for line := range strings.SplitSeq(strings.TrimRight(rendered, "\n"), "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("cross-project linker line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestParentSelectorViewRespectsVisualWidth(t *testing.T) {
	parentA, err := models.NewTask("parent-a", 11, models.TaskTypeStory, "修复父任务选择器在窄宽度下的宽字符截断问题")
	if err != nil {
		t.Fatalf("NewTask A: %v", err)
	}
	parentB, err := models.NewTask("parent-b", 12, models.TaskTypeTask, "ASCII fallback parent title")
	if err != nil {
		t.Fatalf("NewTask B: %v", err)
	}

	m := NewParentSelectorModel([]*models.Task{parentA, parentB}, nil, false, ViewProjectTree)
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("parent selector line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestProjectPickerViewRespectsVisualWidth(t *testing.T) {
	projectA, err := models.NewProject("proj-a", "ui-fix-sprint", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject A: %v", err)
	}
	projectA.Name = "界面修复冲刺计划项目"
	projectA.Context = "work"

	projectB, err := models.NewProject("proj-b", "ops-project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject B: %v", err)
	}
	projectB.Context = "ops"

	m := NewProjectPickerModel([]*models.Project{projectA, projectB})
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("project picker line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestPriorityPickerViewRespectsVisualWidth(t *testing.T) {
	task, err := models.NewTask("task-1", 5, models.TaskTypeTask, "修复优先级选择器标题在窄宽度下的溢出问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Priority = 3

	m := NewPriorityPickerModel(task, nil)
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("priority picker line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestTaskJumperViewRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "task-jumper-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	task, err := models.NewTask("task-1", 17, models.TaskTypeTask, "修复任务跳转器在窄宽度下的宽字符截断问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Status = models.StatusInProgress
	project.Tasks = []*models.Task{task}

	m := NewTaskJumperModel(project)
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("task jumper line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestNotesSearchViewRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "notes-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	note := &models.Note{Title: "修复笔记搜索列表在窄宽度下的宽字符截断问题", Body: "body"}

	m := &NotesSearchModel{
		project:  project,
		filtered: []notesSearchResult{{note: note}},
	}
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("notes search line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestRenderInboxTaskDisplayRespectsVisualWidth(t *testing.T) {
	task, err := models.NewTask("task-1", 23, models.TaskTypeTask, "修复收件箱卡片在窄宽度下的宽字符标题截断问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Description = "这是一段需要在窄宽度下正确截断的描述内容，用于验证收件箱卡片布局。"

	rendered := renderInboxTaskDisplay(task, 32, true, "界面修复项目", testutil.NewTestConfig())
	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("inbox task display line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestFormatSidebarLinkedTaskRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "sidebar-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Name = "界面修复冲刺计划项目"
	task, err := models.NewTask("task-1", 31, models.TaskTypeTask, "修复侧边栏关联任务在窄宽度下的宽字符截断问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	line := formatSidebarLinkedTask(sidebarLinkedTask{Task: task, Project: project}, 24, true, true)
	if lipgloss.Width(ansi.Strip(line)) > 24 {
		t.Fatalf("sidebar linked task line exceeds width 24: %q (width=%d)", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)))
	}
	if !strings.Contains(ansi.Strip(line), "[界面") {
		t.Fatalf("expected project label to remain visible, got %q", ansi.Strip(line))
	}
}

func TestPriorityWaitingOnTaskRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "priority-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	dep, err := models.NewTask("dep-1", 41, models.TaskTypeTask, "等待中的依赖任务标题很长需要截断")
	if err != nil {
		t.Fatalf("NewTask dep: %v", err)
	}
	blocked, err := models.NewTask("task-1", 42, models.TaskTypeTask, "修复等待中任务行在窄宽度下的宽字符截断问题")
	if err != nil {
		t.Fatalf("NewTask blocked: %v", err)
	}
	blocked.DependsOn = []string{dep.ID}
	project.Tasks = []*models.Task{dep, blocked}

	m := NewPriorityViewModel(project, nil, nil)
	m.SetSize(32, 14)
	line := m.renderWaitingOnTask(&PriorityTask{Task: blocked})
	if lipgloss.Width(ansi.Strip(line)) > 32 {
		t.Fatalf("priority waiting-on line exceeds width 32: %q (width=%d)", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)))
	}
}

func TestGlobalNoteMoveViewRespectsVisualWidth(t *testing.T) {
	projectA, err := models.NewProject("proj-a", "ui-fix-sprint", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject A: %v", err)
	}
	projectA.Name = "界面修复冲刺计划项目"
	projectA.Context = "work"

	projectB, err := models.NewProject("proj-b", "ops-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject B: %v", err)
	}
	projectB.Context = "ops"

	m := NewGlobalNoteMoveProjectSelectorModel([]*models.Project{projectA, projectB})
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("global note move line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestLinkedTaskCreatorViewRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "creator-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	sourceTask, err := models.NewTask("task-1", 55, models.TaskTypeTask, "修复关联任务创建器在窄宽度下的宽字符标题截断问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	m := NewLinkedTaskCreatorModel(sourceTask, project, LinkTypeBlocker)
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("linked task creator line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestRenderTextAttrPreviewUsesVisualWidth(t *testing.T) {
	rendered := renderTextAttrPreview("修复属性预览在窄宽度下的宽字符截断问题", false)
	if lipgloss.Width(ansi.Strip(rendered)) > 40 {
		t.Fatalf("text attr preview exceeds width 40: %q (width=%d)", ansi.Strip(rendered), lipgloss.Width(ansi.Strip(rendered)))
	}
}

func TestInlinePreviewLinesRespectsVisualWidth(t *testing.T) {
	task, err := models.NewTask("task-1", 61, models.TaskTypeTask, "Inline preview")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Description = "修复内联预览在窄宽度下的宽字符描述截断问题"
	task.Checklist = []models.ChecklistItem{{Text: "修复清单项宽字符截断问题", Done: false}}

	lines := inlinePreviewLines(task, "│   ", 20)
	for _, line := range lines {
		if lipgloss.Width(ansi.Strip(line)) > 20 {
			t.Fatalf("inline preview line exceeds width 20: %q (width=%d)", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)))
		}
	}
}

func TestRenderTruncatedTagsUsesVisualWidth(t *testing.T) {
	rendered := renderTruncatedTags([]string{"界面修复冲刺计划", "次要标签"}, 2, 6)
	plain := ansi.Strip(rendered)

	if !strings.Contains(plain, "#") {
		t.Fatalf("expected tag rendering, got %q", plain)
	}
	for part := range strings.SplitSeq(plain, " ") {
		if strings.HasPrefix(part, "#") && lipgloss.Width(strings.TrimPrefix(part, "#")) > 6 {
			t.Fatalf("tag part exceeds width 6: %q (width=%d)", part, lipgloss.Width(strings.TrimPrefix(part, "#")))
		}
	}
}

func TestWorkflowRulesListRespectsVisualWidth(t *testing.T) {
	m := &WorkflowRulesModel{
		filteredRules: []workflow.WorkflowRule{{
			ID:      "rule-1",
			Name:    "修复工作流规则列表在窄宽度下的宽字符截断问题",
			Enabled: true,
			Trigger: workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "todo"},
			Actions: []workflow.RuleAction{{ActionType: workflow.ActionSetPriority, Value: "1"}},
		}},
	}
	m.SetSize(24, 10)
	rendered := m.renderRulesList(4)

	for line := range strings.SplitSeq(strings.TrimRight(rendered, "\n"), "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 24 {
			t.Fatalf("workflow rules line exceeds width 24: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestTemplateListRespectsVisualWidth(t *testing.T) {
	m := &TaskTemplatesModel{
		filteredTemplates: []template.TaskTemplate{{
			ID:        "tmpl-1",
			Name:      "修复模板列表在窄宽度下的宽字符截断问题",
			Enabled:   true,
			Type:      models.TaskTypeTask,
			Checklist: []string{"one", "two"},
		}},
	}
	m.SetSize(24, 10)
	rendered := m.renderTemplatesList(4)

	for line := range strings.SplitSeq(strings.TrimRight(rendered, "\n"), "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 24 {
			t.Fatalf("template list line exceeds width 24: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestCommandPaletteSetSizeAllowsNarrowWidths(t *testing.T) {
	m := &CommandPaletteModel{searchInput: textinput.New()}
	m.SetSize(18, 10)

	innerWidth := max(18-10, 1)
	used := m.listWidth + m.previewWidth
	if m.previewWidth > 0 {
		used++
	}
	if used > innerWidth {
		t.Fatalf("palette widths exceed inner width: list=%d preview=%d used=%d inner=%d", m.listWidth, m.previewWidth, used, innerWidth)
	}
	if !m.compactMode {
		t.Fatalf("expected compact mode on narrow palette width")
	}
}

func TestTaskChecklistViewRespectsVisualWidth(t *testing.T) {
	task, err := models.NewTask("task-1", 17, models.TaskTypeTask, "Checklist")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Checklist = []models.ChecklistItem{{Text: "修复检查清单列表在窄宽度下的宽字符截断问题", Done: false}}

	m := NewTaskChecklistModel(task, nil)
	m.SetSize(24, 12)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 24 {
			t.Fatalf("task checklist line exceeds width 24: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func prefixWidthBeforeToken(t *testing.T, lines []string, contains string, token string) int {
	t.Helper()

	for _, line := range lines {
		if !strings.Contains(line, contains) {
			continue
		}
		idx := strings.Index(line, token)
		if idx == -1 {
			t.Fatalf("line %q missing token %q", line, token)
		}
		return lipgloss.Width(line[:idx])
	}

	t.Fatalf("no line found containing %q", contains)
	return 0
}
