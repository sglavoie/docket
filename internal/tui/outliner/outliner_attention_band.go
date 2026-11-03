package outliner

import (
	"image/color"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
)

const attentionBand = "▐"

// rowAttention reuses the badge ladder; a late check-in wins a tie between
// reviews when a folded group contains both late and current requests.
type rowAttention struct {
	hero       chrome.HeroKind
	lateReview bool
}

func attentionForTask(task *models.Task, now time.Time) rowAttention {
	hero := chrome.PickHeroAttribute(task, now)
	return rowAttention{
		hero: hero,
		lateReview: hero == chrome.HeroReview && task.CheckInIsDueAt(now) &&
			util.DaysBetween(*task.NextCheckIn, now) > 0,
	}
}

func strongestAttention(tasks []*models.Task, now time.Time) rowAttention {
	var strongest rowAttention
	for _, task := range tasks {
		candidate := attentionForTask(task, now)
		if candidate.hero != chrome.HeroNone && (strongest.hero == chrome.HeroNone ||
			candidate.hero < strongest.hero ||
			(candidate.hero == strongest.hero && candidate.lateReview)) {
			strongest = candidate
		}
	}
	return strongest
}

func (a rowAttention) color() color.Color {
	switch a.hero {
	case chrome.HeroOverdue:
		return theme.ColorError
	case chrome.HeroReview:
		if a.lateReview {
			return theme.ColorWarning
		}
		return theme.LensAccentDueTodayStyle.GetForeground()
	case chrome.HeroDueToday:
		return theme.LensAccentDueTodayStyle.GetForeground()
	case chrome.HeroP1:
		return theme.ColorPriority1Title
	default:
		return nil
	}
}

// label spells the same winning signal the band colors. Peek uses it when a
// folded header hides the task badges that would otherwise explain the color.
func (a rowAttention) label() string {
	switch a.hero {
	case chrome.HeroOverdue:
		return "Overdue deadline"
	case chrome.HeroReview:
		if a.lateReview {
			return "Late check-in"
		}
		return "Review needed"
	case chrome.HeroDueToday:
		return "Deadline today"
	case chrome.HeroP1:
		return "Priority 1"
	default:
		return ""
	}
}

func (m *OutlinerModel) headerAttentionLabel(row OutlinerRow, now time.Time) string {
	if !row.IsCollapsibleHeader() || !m.IsSectionRowCollapsed(row) {
		return ""
	}
	return strongestAttention(row.attentionTasks, now).label()
}

func (a rowAttention) render() string {
	if accent := a.color(); accent != nil {
		return lipgloss.NewStyle().Foreground(accent).Render(attentionBand)
	}
	return " "
}

func (a rowAttention) renderColumn(paint cursorPaint) string {
	if paint.marked {
		return paint.halfBlock(a.color())
	}
	return a.render()
}

// halfBlock shares one cell: ▌ paints the left half with the cursor color,
// leaving its background to paint the attention band's right half. Both halves
// share the attention accent when present, including during flashes. With no
// color, the left-half glyph still identifies the cursor on its own.
func (p cursorPaint) halfBlock(attention color.Color) string {
	accent := theme.ColorCursorBarBg
	if p.flash {
		accent = theme.ColorFlashBg
	}
	style := lipgloss.NewStyle().Foreground(accent)
	if attention != nil {
		style = style.Foreground(attention).Background(attention)
	}
	return style.Render("▌")
}

// A header retains only its admitted members, before folds hide them. Reading
// their signals at render time also keeps a folded summary current at midnight.
func (m *OutlinerModel) renderHeaderAttention(row OutlinerRow, paint cursorPaint) string {
	var attention rowAttention
	if m.IsSectionRowCollapsed(row) {
		attention = strongestAttention(row.attentionTasks, time.Now())
	}
	return attention.renderColumn(paint)
}

func projectGroupAttentionTasks(groups []projectRowGroup) []*models.Task {
	var tasks []*models.Task
	for _, group := range groups {
		tasks = append(tasks, group.forest.tasks()...)
	}
	return tasks
}
