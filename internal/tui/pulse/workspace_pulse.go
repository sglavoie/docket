package pulse

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// WorkspacePulse is the compact operational readout shared by task-workspace
// presentations. Inbox, Today, and Overdue count actionable work and exclude
// terminal and deferred tasks; Done is the current local day's progress count.
type WorkspacePulse struct {
	Inbox   int
	Today   int
	Overdue int
	Done    int

	// lit is how the readout draws a count that just changed. It is a
	// render-only annotation the shell adds to the copy it hands a view, never
	// part of the measured pulse, so it changes paint and never a width or a
	// ladder rung. See Lit.
	lit PulseChange
}

// PulseComponent names one count of the readout, as a bit of a set.
type PulseComponent uint8

const (
	PulseInbox PulseComponent = 1 << iota
	PulseToday
	PulseOverdue
	PulseDone
)

// PulseChange is which counts moved between two pulses and which way, plus the
// phase they are drawn in. Fresh is the first phase, lit in the component's
// accent; settling is the second, bold in its ordinary color, after which the
// count reads as any other.
type PulseChange struct {
	Raised   PulseComponent
	Lowered  PulseComponent
	Settling bool
	// Ticking names the counts drawn at an in-between value, ticker, ahead
	// of the fresh phase: a count that jumped by several steps counts its way
	// there rather than snapping. See TickerAt.
	Ticking PulseComponent
	ticker  [4]int
}

// pulseComponents is the readout's counts in a fixed order, the order ticker
// stores them in.
var pulseComponents = [...]PulseComponent{PulseInbox, PulseToday, PulseOverdue, PulseDone}

func (p WorkspacePulse) count(component PulseComponent) int {
	switch component {
	case PulseInbox:
		return p.Inbox
	case PulseToday:
		return p.Today
	case PulseOverdue:
		return p.Overdue
	default:
		return p.Done
	}
}

// pulseTickerMaxFrames bounds how many in-between values a count draws, so a
// sync that moves a count by forty still settles within a few frames.
const pulseTickerMaxFrames = 4

// TickerFrames is how many in-between values the change from before to after
// draws: one fewer than the largest step any count took, capped. A count that
// moved by one has nothing between, and a count whose digits change in number
// — 9 to 12 — never ticks, since an in-between value would be a different
// width than the settled one and the pulse changes paint, never width. A
// count at zero after is not drawn, so it never ticks either.
func TickerFrames(before, after WorkspacePulse) int {
	frames := 0
	for _, component := range pulseComponents {
		from, to := before.count(component), after.count(component)
		if !tickable(from, to) {
			continue
		}
		frames = max(frames, min(absInt(to-from)-1, pulseTickerMaxFrames))
	}
	return frames
}

func tickable(from, to int) bool {
	return to > 0 && from > 0 && absInt(to-from) > 1 && len(fmt.Sprint(from)) == len(fmt.Sprint(to))
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// TickerAt is the change drawing frame of frames in-between values from
// before to after: each tickable count at its share of the way, eased so the
// first frames cover most of the distance. A frame past the last draws none.
func (c PulseChange) TickerAt(before, after WorkspacePulse, frame, frames int) PulseChange {
	c.Ticking = 0
	if frame < 0 || frame >= frames {
		return c
	}
	t := float64(frame+1) / float64(frames+1)
	eased := 1 - (1-t)*(1-t)
	for index, component := range pulseComponents {
		from, to := before.count(component), after.count(component)
		if !tickable(from, to) {
			continue
		}
		value := from + int(float64(to-from)*eased)
		if value == to {
			value = to - sign(to-from)
		}
		c.Ticking |= component
		c.ticker[index] = value
	}
	return c
}

func sign(n int) int {
	if n < 0 {
		return -1
	}
	return 1
}

// shown is the value one count draws this frame: its in-between value while
// it ticks, else its own.
func (p WorkspacePulse) shown(component PulseComponent) int {
	if p.lit.Ticking&component != 0 {
		for index, candidate := range pulseComponents {
			if candidate == component {
				return p.lit.ticker[index]
			}
		}
	}
	return p.count(component)
}

// Any reports whether some count moved.
func (c PulseChange) Any() bool { return c.Raised|c.Lowered != 0 }

// PulseChangeBetween names the counts that differ from before to after. Only
// the values compare; neither pulse's own annotation is read.
func PulseChangeBetween(before, after WorkspacePulse) PulseChange {
	var change PulseChange
	for _, pair := range []struct {
		component     PulseComponent
		before, after int
	}{
		{PulseInbox, before.Inbox, after.Inbox},
		{PulseToday, before.Today, after.Today},
		{PulseOverdue, before.Overdue, after.Overdue},
		{PulseDone, before.Done, after.Done},
	} {
		switch {
		case pair.after > pair.before:
			change.Raised |= pair.component
		case pair.after < pair.before:
			change.Lowered |= pair.component
		}
	}
	return change
}

// Lit returns the pulse annotated to draw change. A component the change does
// not name, or one at zero and so not drawn, renders as it always does.
func (p WorkspacePulse) Lit(change PulseChange) WorkspacePulse {
	p.lit = change
	return p
}

// componentStyle is how one count is drawn: base unless it just changed. A
// fresh change takes the accent its direction earns — success for Done rising,
// the warning ink for Overdue rising, the primary otherwise — and a settling
// one keeps base's color, bold. Neither adds a cell.
func (p WorkspacePulse) componentStyle(component PulseComponent, base lipgloss.Style) lipgloss.Style {
	raised, lowered := p.lit.Raised&component != 0, p.lit.Lowered&component != 0
	if !raised && !lowered {
		return base
	}
	if p.lit.Settling {
		return base.Bold(true)
	}
	switch {
	case raised && component == PulseDone:
		return base.Foreground(theme.ColorSuccess).Bold(true)
	case raised && component == PulseOverdue:
		return base.Foreground(theme.ColorWarning).Bold(true)
	}
	return base.Foreground(theme.ColorPrimary).Bold(true)
}

type workspacePulseLevel uint8

const (
	workspacePulseFull workspacePulseLevel = iota
	workspacePulseWithoutDone
	workspacePulseWithoutInbox
	workspacePulseTodayOnly
	workspacePulseHidden
)

// WorkspacePulseState is embedded by full-screen presentations that own a
// workspace-pulse title. The app shell refreshes it immediately before paint.
// Keeping this state separate from view-specific scope makes task- and
// project-scoped screens label the readout as workspace-wide.
type WorkspacePulseState struct {
	workspacePulse WorkspacePulse
}

func (s *WorkspacePulseState) SetWorkspacePulse(pulse WorkspacePulse) {
	s.workspacePulse = pulse
}

func (s *WorkspacePulseState) CurrentWorkspacePulse() WorkspacePulse {
	return s.workspacePulse
}

// WorkspacePulseAware is implemented by views that render the workspace pulse.
// AppModel injects the latest cached value before each render.
type WorkspacePulseAware interface {
	SetWorkspacePulse(WorkspacePulse)
}

// ComputeWorkspacePulse counts actionable work across an already-loaded base
// task workspace at one injected reference time. Selecting and loading that
// population is the caller's, because the snapshot loader sits well above this
// package; passing the partial result through preserves the cross-scope
// loader's partial failure behavior, since a project that failed to load is
// simply absent from projects and contributes nothing.
//
// Today includes the explicit plan and tasks due today or overdue, counted once per
// project-qualified identity. Stale references, ineligible tasks, and references to excluded
// projects do not contribute because only loaded eligible tasks are considered.
// Effectively hidden work never contributes, whatever the view's facets say: a
// hidden task is never owed attention.
func ComputeWorkspacePulse(projects []*models.Project, todayIDs map[taskref.TaskIdentity]bool, now time.Time) WorkspacePulse {
	var pulse WorkspacePulse
	countedToday := make(map[taskref.TaskIdentity]bool, len(todayIDs))
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.State().IsTerminal() || task.IsDeferredAt(now) ||
				models.TaskEffectivelyHidden(project, task) {
				continue
			}

			identity := taskref.For(task, project)
			if identity.Valid() && (todayIDs[identity] || task.IsDueTodayAt(now) || task.IsOverdueAt(now)) &&
				!countedToday[identity] && app.PlanningTodayEligible(task.State()) {
				pulse.Today++
				countedToday[identity] = true
			}
			if task.IsInState(models.StateInbox) {
				pulse.Inbox++
			}
			if TaskIsOverdueAt(task, now) {
				pulse.Overdue++
			}
		}
	}
	return pulse
}

func TaskIsOverdueAt(task *models.Task, now time.Time) bool {
	return task != nil && task.IsOverdueAt(now)
}

// formatWorkspacePulseComponents is the title-fitting seam for the fixed
// pressure ladder: Done, Inbox, Overdue, then Today. The full form keeps the
// semantic component order stable.
//
// A zero component is omitted rather than rendered, because "today 0 · done 0"
// spends the header's scarcest columns saying that nothing is there. The ladder
// is unchanged by that: it still drops Done, then Inbox, then Overdue, then
// Today, and it decides which of the nonzero components survive a narrow row.
// When none of them is nonzero there is no readout at all, so the "workspace: "
// prefix goes with them rather than standing over an empty list.
func formatWorkspacePulseComponents(pulse WorkspacePulse, level workspacePulseLevel) string {
	if level >= workspacePulseHidden {
		return ""
	}

	parts := make([]string, 0, 4)
	if level < workspacePulseWithoutInbox && pulse.Inbox > 0 {
		parts = append(parts, pulse.componentStyle(PulseInbox, theme.MutedStyle).Render(fmt.Sprintf("inbox %d", pulse.shown(PulseInbox))))
	}
	if pulse.Today > 0 {
		parts = append(parts, pulse.componentStyle(PulseToday, theme.MutedStyle).Render(fmt.Sprintf("today %d", pulse.shown(PulseToday))))
	}
	if level < workspacePulseTodayOnly && pulse.Overdue > 0 {
		parts = append(parts, pulse.componentStyle(PulseOverdue, theme.PreviewWarningStyle).Render(fmt.Sprintf("overdue %d", pulse.shown(PulseOverdue))))
	}
	if level < workspacePulseWithoutDone && pulse.Done > 0 {
		parts = append(parts, pulse.componentStyle(PulseDone, theme.MutedStyle).Render(fmt.Sprintf("done %d", pulse.shown(PulseDone))))
	}
	if len(parts) == 0 {
		return ""
	}
	return theme.MutedStyle.Render("workspace: ") + strings.Join(parts, theme.MutedStyle.Render(" · "))
}

// WindowTitle is the pulse as the terminal's window title: `Docket`, then each
// nonzero component in the readout's order and spelling, without Done, which
// is progress rather than something waiting. It is plain text and never
// shortened, since a title bar elides on its own. Docket.app reads the overdue
// component back for its Dock badge (DocketWindowTitle in macos/DocketApp/Shell/DocketAttention.swift),
// so the spelling `overdue N` is a contract with that parser.
func WindowTitle(pulse WorkspacePulse) string {
	parts := []string{"Docket"}
	if pulse.Inbox > 0 {
		parts = append(parts, fmt.Sprintf("inbox %d", pulse.shown(PulseInbox)))
	}
	if pulse.Today > 0 {
		parts = append(parts, fmt.Sprintf("today %d", pulse.shown(PulseToday)))
	}
	if pulse.Overdue > 0 {
		parts = append(parts, fmt.Sprintf("overdue %d", pulse.shown(PulseOverdue)))
	}
	return strings.Join(parts, " · ")
}
