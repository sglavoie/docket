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

	rendered := renderInboxTaskDisplay(task, 32, true, &models.Project{Name: "界面修复项目", ColorSlot: 1}, newTestConfig().Settings())
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

	cfg := newTestConfig()
	m := &DashboardModel{runtime: testRuntimeAt(cfg.DataDir(), cfg), projects: projects}
	m.SetSize(28, 10)
	rendered := m.renderProjectList(projects, 6, 28)
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

// TestRenderInboxFooterRespectsVisualWidthAcrossLayouts checks the triage
// footer fits both a normal terminal width and a narrow one. The footer is
// the primary visual surface for the six-key triage vocabulary established
// in session 2, so it must not wrap or overflow at either size.
func TestRenderInboxFooterRespectsVisualWidthAcrossLayouts(t *testing.T) {
	for _, width := range []int{120, 48} {
		rendered := renderInboxFooter(width)
		assertRenderedLinesWithinVisualWidth(t, fmt.Sprintf("inbox footer @%d", width), rendered, width)
	}
}

// TestRenderProjectListNarrowModeRespectsVisualWidth exercises the dashboard
// narrow-mode rendering path: wide rows split into name + stats lines and
// must respect cardWidth even with active inbox indicators and long names.
func TestRenderProjectListNarrowModeRespectsVisualWidth(t *testing.T) {
	cfg := newTestConfig()
	projects := make([]*models.Project, 0, 6)
	for i := range 6 {
		p := testutil.NewTestProject(fmt.Sprintf("界面修复冲刺计划项目 %d", i))
		p.Context = "work"
		// Add a couple of inbox tasks so the project row carries the inbox indicator.
		for j := range 2 {
			task, err := models.NewTask(fmt.Sprintf("p%d-t%d", i, j), j+1, models.TaskTypeTask, "inbox item")
			if err != nil {
				t.Fatalf("NewTask: %v", err)
			}
			task.SetSection(models.SectionInbox)
			p.Tasks = append(p.Tasks, task)
		}
		projects = append(projects, p)
	}

	m := &DashboardModel{runtime: testRuntimeAt(cfg.DataDir(), cfg), projects: projects}
	cardWidth := DashboardNarrowModeWidth - 4
	m.SetSize(cardWidth+6, 30)
	rendered := m.renderProjectList(projects, 12, cardWidth)
	assertRenderedLinesWithinVisualWidth(t, "dashboard project list narrow mode", rendered, cardWidth)
}

// TestHelpRenderByIntentContentRespectsVisualWidth confirms that the per-view
// intent-grouped help layout sizes its two-column grid from viewport width and
// does not overflow at narrow widths.
func TestHelpRenderByIntentContentRespectsVisualWidth(t *testing.T) {
	items := []HelpItem{
		{Key: "n", Description: "Skip task (move to next without processing)", Category: "Navigation"},
		{Key: "e", Description: "Edit task title/description inline", Category: "Operations"},
		{Key: "D", Description: "Mark task done and advance", Category: "Operations"},
		{Key: "T", Description: "Manage tags for the current task", Category: "Operations"},
		{Key: "u", Description: "Undo last operation on current task", Category: "Operations"},
		{Key: "?", Description: "Toggle help", Category: "Navigation"},
		{Key: ",s", Description: "Set status start", Category: "Status"},
		{Key: "ms", Description: "Move to Someday", Category: "Move (m prefix)"},
		{Key: ";d", Description: "Open Daily Review", Category: "Navigation"},
	}

	for _, width := range []int{120, 60} {
		m := NewHelpViewModel(items, ViewInboxProcessor, "Inbox Processor")
		m.SetSize(width, 32)
		rendered := m.renderByIntentContent()
		// Allow the inner viewport width as the budget — same constraint
		// the layout code uses to compute the column widths.
		assertRenderedLinesWithinVisualWidth(t, fmt.Sprintf("help byIntent @%d", width), rendered, max(width-2, 1))
	}
}

// TestQuickCaptureViewNarrowLayoutRespectsVisualWidth renders the full
// QuickCapture fullscreen view at a narrow width and confirms no line
// overflows. This guards the calm placeholder + gated syntax guide regime
// against regressions where helper text could reintroduce wide hints.
func TestQuickCaptureViewNarrowLayoutRespectsVisualWidth(t *testing.T) {
	project := testutil.NewTestProject("narrow capture")
	m := NewQuickCaptureModel(project, nil, false)
	for _, width := range []int{120, 48} {
		m.SetSize(width, 24)
		rendered := m.View().Content
		assertRenderedLinesWithinVisualWidth(t, fmt.Sprintf("quick capture view @%d", width), rendered, width)
	}
}
