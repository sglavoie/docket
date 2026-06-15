package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/workflow"
)

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

func TestWorkflowRulesListScrollbarRespectsVisualWidth(t *testing.T) {
	rules := make([]workflow.WorkflowRule, 0, 6)
	for i := range 6 {
		rules = append(rules, workflow.WorkflowRule{
			ID:      fmt.Sprintf("rule-%d", i),
			Name:    fmt.Sprintf("修复工作流规则列表滚动条宽字符截断问题 %d", i),
			Enabled: i%2 == 0,
			Trigger: workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "todo"},
			Actions: []workflow.RuleAction{{ActionType: workflow.ActionSetPriority, Value: "1"}},
		})
	}
	m := &WorkflowRulesModel{filteredRules: rules}
	m.cursor = 5
	m.SetSize(24, 10)

	rendered := m.renderRulesList(3)
	assertRenderedLinesWithinVisualWidth(t, "workflow rules scrollbar", rendered, 24)
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

func TestTemplateListScrollbarRespectsVisualWidth(t *testing.T) {
	templates := make([]template.TaskTemplate, 0, 6)
	for i := range 6 {
		templates = append(templates, template.TaskTemplate{
			ID:        fmt.Sprintf("tmpl-%d", i),
			Name:      fmt.Sprintf("修复模板列表滚动条宽字符截断问题 %d", i),
			Enabled:   i%2 == 0,
			Type:      models.TaskTypeTask,
			Checklist: []string{"one", "two"},
		})
	}
	m := &TaskTemplatesModel{filteredTemplates: templates}
	m.cursor = 5
	m.SetSize(24, 10)

	rendered := m.renderTemplatesList(3)
	assertRenderedLinesWithinVisualWidth(t, "template list scrollbar", rendered, 24)
}

func TestDashboardProjectListScrollbarRespectsVisualWidthWithEmoji(t *testing.T) {
	icons.SetUseEmoji(true)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	projects := make([]*models.Project, 0, 10)
	for i := range 10 {
		project := testutil.NewTestProject(fmt.Sprintf("界面修复冲刺计划项目 %d", i))
		project.Context = "work"
		project.StatusIcon = "active"
		projects = append(projects, project)
	}

	m := &DashboardModel{config: testutil.NewTestConfig(), projects: projects}
	m.SetSize(28, 10)
	rendered := m.renderProjectList(projects, 0, 28, 0)
	assertRenderedLinesWithinVisualWidth(t, "dashboard project list scrollbar", rendered, 24)
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

func assertRenderedLinesWithinVisualWidth(t *testing.T, label, rendered string, width int) {
	t.Helper()
	for line := range strings.SplitSeq(strings.TrimRight(rendered, "\n"), "\n") {
		if lipgloss.Width(ansi.Strip(line)) > width {
			t.Fatalf("%s line exceeds width %d: %q (width=%d)\n\n%s", label, width, ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}
