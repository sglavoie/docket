package tui

import (
	"fmt"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// namedLensFixtureID is an arbitrary well-formed identity. The resolver never
// reads it, which is exactly what the identity-independence cases below prove.
const namedLensFixtureID = "1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed"

func namedLensFixture(mutate func(*config.NamedLens)) config.NamedLens {
	lens := config.NamedLens{
		ID:   namedLensFixtureID,
		Name: "Fixture",
		Mode: config.LensModeAll,
		Sort: config.LensSort{Field: config.LensSortDefault, Ascending: true},
	}
	if mutate != nil {
		mutate(&lens)
	}
	return lens
}

func namedLensStatesFixture(states ...string) config.NamedLens {
	return namedLensFixture(func(lens *config.NamedLens) {
		lens.Tasks.States = states
		lens.Tasks.StatesSet = true
	})
}

func groupRole(group models.WorkflowStateGroup) namedLensRole {
	return namedLensRole{Kind: namedLensRoleWorkflowGroup, Group: group}
}

func modeRole(mode OutlinerModeID) namedLensRole {
	return namedLensRole{Kind: namedLensRoleMode, Mode: mode}
}

// roleKindText spells one kind for a failure message. It is the only place a
// role is named, and it is deliberately test-only: production never reads a role
// back out of its spelling, so a name living beside the resolver would be code
// no build path reaches.
func roleKindText(kind namedLensRoleKind) string {
	switch kind {
	case namedLensRoleNone:
		return "none"
	case namedLensRoleOverdue:
		return "overdue"
	case namedLensRoleWaiting:
		return "waiting"
	case namedLensRoleDueToday:
		return "due-today"
	case namedLensRolePriority:
		return "priority"
	case namedLensRoleDue:
		return "due"
	case namedLensRoleDeferred:
		return "deferred"
	case namedLensRoleReview:
		return "review"
	case namedLensRoleCompleted:
		return "completed"
	case namedLensRoleWorkflowGroup:
		return "workflow-group"
	case namedLensRoleWarning:
		return "warning"
	case namedLensRoleMode:
		return "mode"
	}
	return fmt.Sprintf("kind(%d)", int(kind))
}

// roleText renders a whole role, naming the kind and appending only the payload
// fields that kind gives meaning to.
func roleText(role namedLensRole) string {
	text := roleKindText(role.Kind)
	switch role.Kind {
	case namedLensRolePriority:
		text += fmt.Sprintf("(%d)", role.Priority)
	case namedLensRoleWorkflowGroup:
		text += fmt.Sprintf("(%s)", role.Group)
	case namedLensRoleMode:
		text += fmt.Sprintf("(%s)", role.Mode)
	case namedLensRoleNone,
		namedLensRoleOverdue,
		namedLensRoleWaiting,
		namedLensRoleDueToday,
		namedLensRoleDue,
		namedLensRoleDeferred,
		namedLensRoleReview,
		namedLensRoleCompleted,
		namedLensRoleWarning:
		// Every other kind leaves the payload fields zero and says nothing more.
	}
	return text
}

func TestNamedLensSemanticRole(t *testing.T) {
	cases := []struct {
		name string
		lens config.NamedLens
		want namedLensRole
	}{
		// Rung 1: overdue outranks every other signal a definition can carry.
		{
			name: "overdue due bucket",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.DueDate = config.LensDueOverdue }),
			want: namedLensRole{Kind: namedLensRoleOverdue},
		},
		{
			name: "overdue outranks waiting priority and mode",
			lens: namedLensFixture(func(l *config.NamedLens) {
				l.Mode = config.LensModeDue
				l.Tasks.DueDate = config.LensDueOverdue
				l.Tasks.States = []string{string(models.StateWaiting)}
				l.Tasks.StatesSet = true
				l.Tasks.Priority = models.PriorityHigh
			}),
			want: namedLensRole{Kind: namedLensRoleOverdue},
		},

		// Rung 2: waiting is a hero attribute and outranks the number beside it.
		{
			name: "explicit waiting state",
			lens: namedLensStatesFixture(string(models.StateWaiting)),
			want: namedLensRole{Kind: namedLensRoleWaiting},
		},
		{
			name: "waiting outranks priority 1",
			lens: namedLensFixture(func(l *config.NamedLens) {
				l.Tasks.States = []string{string(models.StateWaiting)}
				l.Tasks.StatesSet = true
				l.Tasks.Priority = models.PriorityHigh
			}),
			want: namedLensRole{Kind: namedLensRoleWaiting},
		},
		{
			name: "waiting outranks due today",
			lens: namedLensFixture(func(l *config.NamedLens) {
				l.Tasks.States = []string{string(models.StateWaiting)}
				l.Tasks.StatesSet = true
				l.Tasks.DueDate = config.LensDueToday
			}),
			want: namedLensRole{Kind: namedLensRoleWaiting},
		},
		{
			name: "waiting inside a mixed state set still wins",
			lens: namedLensStatesFixture(string(models.StateWaiting), string(models.StatePaused)),
			want: namedLensRole{Kind: namedLensRoleWaiting},
		},

		// Rung 3 and 4/5: the due window, then the priority ladder.
		{
			name: "due today",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.DueDate = config.LensDueToday }),
			want: namedLensRole{Kind: namedLensRoleDueToday},
		},
		{
			name: "due today outranks priority",
			lens: namedLensFixture(func(l *config.NamedLens) {
				l.Tasks.DueDate = config.LensDueToday
				l.Tasks.Priority = models.PriorityHigh
			}),
			want: namedLensRole{Kind: namedLensRoleDueToday},
		},
		{
			name: "priority 1",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.Priority = models.PriorityHigh }),
			want: namedLensRole{Kind: namedLensRolePriority, Priority: models.PriorityHigh},
		},
		{
			name: "priority 2",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.Priority = models.PriorityMedium }),
			want: namedLensRole{Kind: namedLensRolePriority, Priority: models.PriorityMedium},
		},
		{
			name: "priority 3",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.Priority = models.PriorityLow }),
			want: namedLensRole{Kind: namedLensRolePriority, Priority: models.PriorityLow},
		},
		{
			name: "priority outranks the deferred filter",
			lens: namedLensFixture(func(l *config.NamedLens) {
				l.Tasks.Priority = models.PriorityLow
				l.Tasks.Deferred = true
			}),
			want: namedLensRole{Kind: namedLensRolePriority, Priority: models.PriorityLow},
		},
		{
			name: "unset priority is not a signal",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.Priority = models.PriorityUnset }),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "out-of-range priority is not a signal",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.Priority = 5 }),
			want: namedLensRole{Kind: namedLensRoleNone},
		},

		// Rung 6: the remaining active window, and the two buckets excluded from it.
		{
			name: "due this week",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.DueDate = config.LensDueWeek }),
			want: namedLensRole{Kind: namedLensRoleDue},
		},
		{
			name: "due all is not an accent",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.DueDate = config.LensDueAll }),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "due none is not an accent",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.DueDate = config.LensDueNone }),
			want: namedLensRole{Kind: namedLensRoleNone},
		},

		// Rungs 7 to 9.
		{
			name: "deferred filter",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.Deferred = true }),
			want: namedLensRole{Kind: namedLensRoleDeferred},
		},
		{
			name: "deferred outranks the review tag",
			lens: namedLensFixture(func(l *config.NamedLens) {
				l.Tasks.Deferred = true
				l.Tasks.Tags = []string{reviewTaskTag}
			}),
			want: namedLensRole{Kind: namedLensRoleDeferred},
		},
		{
			name: "review task tag",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.Tags = []string{reviewTaskTag} }),
			want: namedLensRole{Kind: namedLensRoleReview},
		},
		{
			name: "review tag matches its normalized spelling",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.Tags = []string{"Review"} }),
			want: namedLensRole{Kind: namedLensRoleReview},
		},
		{
			name: "review tag beside a descriptive tag",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.Tags = []string{"ops", reviewTaskTag} }),
			want: namedLensRole{Kind: namedLensRoleReview},
		},
		{
			name: "descriptive tags alone assign no color",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.Tags = []string{"ops", "infra"} }),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "completion age filter",
			lens: namedLensFixture(func(l *config.NamedLens) {
				l.Tasks.CompletionAge = config.LensCompletionAge30Days
				l.Tasks.CompletionAgeSet = true
			}),
			want: namedLensRole{Kind: namedLensRoleCompleted},
		},
		{
			name: "completion age all is still an active facet",
			lens: namedLensFixture(func(l *config.NamedLens) {
				l.Tasks.CompletionAge = config.LensCompletionAgeAll
				l.Tasks.CompletionAgeSet = true
			}),
			want: namedLensRole{Kind: namedLensRoleCompleted},
		},
		{
			name: "all-terminal state set",
			lens: namedLensStatesFixture(string(models.StateDone), string(models.StateCanceled)),
			want: namedLensRole{Kind: namedLensRoleCompleted},
		},
		{
			name: "terminal beside a nonterminal state is not completed",
			lens: namedLensStatesFixture(string(models.StateDone), string(models.StateInbox)),
			want: namedLensRole{Kind: namedLensRoleNone},
		},

		// Rung 10: every coherent single-group state set.
		{
			name: "inbox states",
			lens: namedLensStatesFixture(string(models.StateInbox)),
			want: groupRole(models.StateGroupInbox),
		},
		{
			name: "backlog states",
			lens: namedLensStatesFixture(string(models.StateBacklog)),
			want: groupRole(models.StateGroupBacklog),
		},
		{
			name: "doing states",
			lens: namedLensStatesFixture(string(models.StateDoing)),
			want: groupRole(models.StateGroupDoing),
		},
		{
			name: "paused states",
			lens: namedLensStatesFixture(string(models.StatePaused)),
			want: groupRole(models.StateGroupPaused),
		},
		{
			name: "someday states",
			lens: namedLensStatesFixture(string(models.StateSomeday)),
			want: groupRole(models.StateGroupSomeday),
		},
		{
			name: "repeated state resolves to its one group",
			lens: namedLensStatesFixture(string(models.StateDoing), string(models.StateDoing)),
			want: groupRole(models.StateGroupDoing),
		},
		{
			name: "conflicting state groups fall through",
			lens: namedLensStatesFixture(string(models.StateInbox), string(models.StateDoing)),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "conflicting state groups fall through to the mode",
			lens: namedLensFixture(func(l *config.NamedLens) {
				l.Mode = config.LensModeInbox
				l.Tasks.States = []string{string(models.StateInbox), string(models.StateDoing)}
				l.Tasks.StatesSet = true
			}),
			want: modeRole(ModeInbox),
		},
		{
			name: "an active but empty state predicate is not a group",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.StatesSet = true }),
			want: namedLensRole{Kind: namedLensRoleNone},
		},

		// Rungs 11 and 12: population and staleness both read as a warning.
		{
			name: "parked-only population",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Projects.ParkedMode = config.LensPopulationOnly }),
			want: namedLensRole{Kind: namedLensRoleWarning},
		},
		{
			name: "parked-included population",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Projects.ParkedMode = config.LensPopulationInclude }),
			want: namedLensRole{Kind: namedLensRoleWarning},
		},
		{
			name: "parked-excluded population is the default",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Projects.ParkedMode = config.LensPopulationExclude }),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "archived-only population",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Projects.ArchivedMode = config.LensPopulationOnly }),
			want: namedLensRole{Kind: namedLensRoleWarning},
		},
		{
			name: "pinned population",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.Pinned = true }),
			want: namedLensRole{Kind: namedLensRoleWarning},
		},
		{
			name: "stale filter",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.Stale = true }),
			want: namedLensRole{Kind: namedLensRoleWarning},
		},
		{
			name: "a state group outranks the stale filter",
			lens: namedLensFixture(func(l *config.NamedLens) {
				l.Tasks.States = []string{string(models.StateDoing)}
				l.Tasks.StatesSet = true
				l.Tasks.Stale = true
			}),
			want: groupRole(models.StateGroupDoing),
		},

		// Rung 13: every non-default mode, and the default that is not a signal.
		{
			name: "inbox mode",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeInbox }),
			want: modeRole(ModeInbox),
		},
		{
			name: "today mode",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeToday }),
			want: modeRole(ModeToday),
		},
		{
			name: "next mode",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeNext }),
			want: modeRole(ModeNext),
		},
		{
			name: "backlog mode",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeBacklog }),
			want: modeRole(ModeBacklog),
		},
		{
			name: "due mode without a due bucket",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeDue }),
			want: modeRole(ModeDue),
		},
		{
			name: "recent mode",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeRecent }),
			want: modeRole(ModeRecent),
		},
		{
			name: "all mode is the default and is not a signal",
			lens: namedLensFixture(nil),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "stale outranks the mode it opens in",
			lens: namedLensFixture(func(l *config.NamedLens) {
				l.Mode = config.LensModeRecent
				l.Tasks.Stale = true
			}),
			want: namedLensRole{Kind: namedLensRoleWarning},
		},

		// Facets that must never assign a color on their own.
		{
			name: "a query alone assigns no color",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Query = "invoice" }),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "a hierarchy filter alone assigns no color",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.Hierarchy = config.LensHierarchyLeaf }),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "project tags and untagged assign no color",
			lens: namedLensFixture(func(l *config.NamedLens) {
				l.Projects.Tags = []string{"client"}
				l.Projects.Untagged = true
			}),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "a sort setting assigns no color",
			lens: namedLensFixture(func(l *config.NamedLens) {
				l.Sort = config.LensSort{Field: config.LensSortTitle, Ascending: false}
			}),
			want: namedLensRole{Kind: namedLensRoleNone},
		},

		// Unknown tokens and zero values resolve to neutral rather than to a guess.
		{
			name: "unknown state token",
			lens: namedLensStatesFixture("almost-done"),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "an unknown state disqualifies the whole set",
			lens: namedLensStatesFixture(string(models.StateDoing), "almost-done"),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "unknown due token",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Tasks.DueDate = "fortnight" }),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "unknown population token",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Projects.ParkedMode = "maybe" }),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "unknown mode token",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Mode = "dashboard" }),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "zero-value lens",
			lens: config.NamedLens{},
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "nil facet slices",
			lens: config.NamedLens{ID: namedLensFixtureID, Name: "Nil", Mode: config.LensModeAll},
			want: namedLensRole{Kind: namedLensRoleNone},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := namedLensSemanticRole(tc.lens); got != tc.want {
				t.Errorf("role = %s, want %s", roleText(got), roleText(tc.want))
			}
		})
	}

	t.Run("built-in catalog", func(t *testing.T) {
		want := map[string]namedLensRole{
			"Waiting":       {Kind: namedLensRoleWaiting},
			"High Priority": {Kind: namedLensRolePriority, Priority: models.PriorityHigh},
			"Stale":         {Kind: namedLensRoleWarning},
			"Overdue":       {Kind: namedLensRoleOverdue},
			"Active Work":   groupRole(models.StateGroupDoing),
			"Review Queue":  {Kind: namedLensRoleReview},
			"Deferred":      {Kind: namedLensRoleDeferred},
			"Cleanup 90d":   {Kind: namedLensRoleCompleted},
			"Parked Work":   {Kind: namedLensRoleWarning},
		}

		catalog := config.BuiltinNamedLensTemplates()
		if len(catalog) != len(want) {
			t.Fatalf("catalog has %d templates, want %d", len(catalog), len(want))
		}
		for _, lens := range catalog {
			expected, ok := want[lens.Name]
			if !ok {
				t.Errorf("catalog entry %q has no expected role", lens.Name)
				continue
			}
			if got := namedLensSemanticRole(lens); got != expected {
				t.Errorf("%q role = %s, want %s", lens.Name, roleText(got), roleText(expected))
			}
		}
	})

	t.Run("identity never reaches the resolver", func(t *testing.T) {
		original := config.BuiltinNamedLensTemplates()[4] // Active Work
		want := namedLensSemanticRole(original)

		renamed := original.Clone()
		renamed.Name = "My Own Work"
		renamed.ID = namedLensFixtureID
		if got := namedLensSemanticRole(renamed); got != want {
			t.Errorf("renamed foreign-ID twin role = %s, want %s", roleText(got), roleText(want))
		}

		handBuilt := namedLensStatesFixture(string(models.StateDoing))
		handBuilt.Sort = original.Sort
		if got := namedLensSemanticRole(handBuilt); got != want {
			t.Errorf("hand-built twin role = %s, want %s", roleText(got), roleText(want))
		}
	})

	t.Run("definition changes under one identity", func(t *testing.T) {
		before := namedLensStatesFixture(string(models.StateDoing))
		after := before.Clone()
		after.Tasks.States = []string{string(models.StateWaiting)}

		if before.ID != after.ID || before.Name != after.Name {
			t.Fatalf("fixtures must share one identity")
		}
		gotBefore, gotAfter := namedLensSemanticRole(before), namedLensSemanticRole(after)
		if gotBefore == gotAfter {
			t.Fatalf("edited definition kept role %s", roleText(gotAfter))
		}
		if gotAfter != (namedLensRole{Kind: namedLensRoleWaiting}) {
			t.Errorf("edited role = %s, want waiting", roleText(gotAfter))
		}
	})
}

func TestNamedLensPrimaryStyle(t *testing.T) {
	// The theme is global mutable state, so these cases run in sequence and hand
	// the package back its default dark palette.
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	styled := []struct {
		name string
		lens config.NamedLens
	}{
		{"overdue", namedLensFixture(func(l *config.NamedLens) { l.Tasks.DueDate = config.LensDueOverdue })},
		{"waiting", namedLensStatesFixture(string(models.StateWaiting))},
		{"due today", namedLensFixture(func(l *config.NamedLens) { l.Tasks.DueDate = config.LensDueToday })},
		{"priority 1", namedLensFixture(func(l *config.NamedLens) { l.Tasks.Priority = models.PriorityHigh })},
		{"priority 2", namedLensFixture(func(l *config.NamedLens) { l.Tasks.Priority = models.PriorityMedium })},
		{"priority 3", namedLensFixture(func(l *config.NamedLens) { l.Tasks.Priority = models.PriorityLow })},
		{"due week", namedLensFixture(func(l *config.NamedLens) { l.Tasks.DueDate = config.LensDueWeek })},
		{"deferred", namedLensFixture(func(l *config.NamedLens) { l.Tasks.Deferred = true })},
		{"review", namedLensFixture(func(l *config.NamedLens) { l.Tasks.Tags = []string{reviewTaskTag} })},
		{"completed", namedLensStatesFixture(string(models.StateDone), string(models.StateCanceled))},
		{"inbox group", namedLensStatesFixture(string(models.StateInbox))},
		{"doing group", namedLensStatesFixture(string(models.StateDoing))},
		{"backlog group", namedLensStatesFixture(string(models.StateBacklog))},
		{"paused group", namedLensStatesFixture(string(models.StatePaused))},
		{"someday group", namedLensStatesFixture(string(models.StateSomeday))},
		{"warning", namedLensFixture(func(l *config.NamedLens) { l.Tasks.Stale = true })},
		{"today mode", namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeToday })},
		{"recent mode", namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeRecent })},
	}

	neutral := []struct {
		name string
		lens config.NamedLens
	}{
		{"zero value", config.NamedLens{}},
		{"all mode only", namedLensFixture(nil)},
		{"descriptive tags", namedLensFixture(func(l *config.NamedLens) { l.Tasks.Tags = []string{"ops"} })},
		{"unknown mode", namedLensFixture(func(l *config.NamedLens) { l.Mode = "dashboard" })},
		{"unknown state", namedLensStatesFixture("almost-done")},
	}

	for _, theme := range []string{"dark", "light"} {
		t.Run(theme, func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: theme})

			for _, tc := range styled {
				t.Run(tc.name, func(t *testing.T) {
					style, ok := namedLensPrimaryStyle(tc.lens)
					if !ok {
						t.Fatalf("no style for a definition with a semantic role")
					}
					assertForegroundOnlyStyle(t, style)
				})
			}

			for _, tc := range neutral {
				t.Run("neutral "+tc.name, func(t *testing.T) {
					if style, ok := namedLensPrimaryStyle(tc.lens); ok {
						t.Fatalf("neutral definition wears %v", style.GetForeground())
					}
				})
			}
		})
	}

	t.Run("accents follow the theme", func(t *testing.T) {
		lens := namedLensFixture(func(l *config.NamedLens) { l.Tasks.DueDate = config.LensDueOverdue })

		ApplyTheme(ThemeConfig{Name: "dark"})
		dark, ok := namedLensPrimaryStyle(lens)
		if !ok {
			t.Fatalf("dark theme resolved no style")
		}
		ApplyTheme(ThemeConfig{Name: "light"})
		light, ok := namedLensPrimaryStyle(lens)
		if !ok {
			t.Fatalf("light theme resolved no style")
		}
		if dark.GetForeground() == light.GetForeground() {
			t.Errorf("overdue accent is %v in both themes", dark.GetForeground())
		}
	})

	// A mode borrows its group's accent and Today shares Backlog's the way their
	// badges already do, so the accents themselves — not the definitions that
	// reach them — are what must read apart.
	t.Run("accents read apart", func(t *testing.T) {
		for _, theme := range []string{"dark", "light"} {
			ApplyTheme(ThemeConfig{Name: theme})

			accents := map[string]lipgloss.Style{
				"plan":      lensAccentPlanStyle,
				"work":      lensAccentWorkStyle,
				"inbox":     lensAccentInboxStyle,
				"next":      lensAccentNextStyle,
				"waiting":   lensAccentWaitingStyle,
				"paused":    lensAccentPausedStyle,
				"someday":   lensAccentSomedayStyle,
				"complete":  lensAccentCompleteStyle,
				"canceled":  lensAccentCanceledStyle,
				"overdue":   lensAccentOverdueStyle,
				"due today": lensAccentDueTodayStyle,
				"due":       lensAccentDueStyle,
				"deferred":  lensAccentDeferredStyle,
				"review":    lensAccentReviewStyle,
			}

			seen := make(map[string]string, len(accents))
			for name, style := range accents {
				assertForegroundOnlyStyle(t, style)
				key := lipgloss.NewStyle().Foreground(style.GetForeground()).Render("x")
				if previous, clash := seen[key]; clash {
					t.Errorf("%s theme: %s wears the same accent as %s", theme, name, previous)
					continue
				}
				seen[key] = name
			}

			// An accent that landed exactly on a priority title color would make a
			// state lens and a priority lens indistinguishable.
			for _, priority := range []int{models.PriorityHigh, models.PriorityMedium, models.PriorityLow} {
				title := GetTitlePriorityStyle(priority).GetForeground()
				for name, style := range accents {
					if style.GetForeground() == title {
						t.Errorf("%s theme: %s accent equals the priority %d title color", theme, name, priority)
					}
				}
			}
		}
	})

	t.Run("priority accents follow the palette", func(t *testing.T) {
		ApplyTheme(ThemeConfig{Name: "dark"})
		rebuildPalette(config.PaletteColors{Priority1Title: "#123456"})
		rebuildPriorityTitleStyles()

		style, ok := namedLensPrimaryStyle(namedLensFixture(func(l *config.NamedLens) {
			l.Tasks.Priority = models.PriorityHigh
		}))
		if !ok {
			t.Fatalf("priority definition resolved no style")
		}
		if got := style.GetForeground(); got != lipgloss.Color("#123456") {
			t.Errorf("priority 1 accent = %v, want the overridden #123456", got)
		}
		assertForegroundOnlyStyle(t, style)
	})
}

// assertForegroundOnlyStyle holds a lens accent to the shared design contract: a
// color and nothing else, so the row's width, weight, and fill stay the list's.
func assertForegroundOnlyStyle(t *testing.T, style lipgloss.Style) {
	t.Helper()

	if _, unset := style.GetForeground().(lipgloss.NoColor); unset {
		t.Errorf("style carries no foreground")
	}
	if _, unset := style.GetBackground().(lipgloss.NoColor); !unset {
		t.Errorf("style carries background %v", style.GetBackground())
	}
	if style.GetBold() {
		t.Errorf("style is bold")
	}
	if style.GetItalic() {
		t.Errorf("style is italic")
	}
	if style.GetFaint() {
		t.Errorf("style is faint")
	}
	if style.GetUnderline() {
		t.Errorf("style is underlined")
	}
	if style.GetStrikethrough() {
		t.Errorf("style is struck through")
	}
	if style.GetWidth() != 0 {
		t.Errorf("style sets width %d", style.GetWidth())
	}
	top, right, bottom, left := style.GetPaddingTop(), style.GetPaddingRight(), style.GetPaddingBottom(), style.GetPaddingLeft()
	if top != 0 || right != 0 || bottom != 0 || left != 0 {
		t.Errorf("style pads %d/%d/%d/%d", top, right, bottom, left)
	}
}
