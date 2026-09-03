package tui

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// This file walks the resolver's precedence ladder rung by rung: which single
// signal a definition earns, which signal outranks which, and which clauses
// earn none at all. The fixtures and the role renderings are in
// named_lens_semantic_color_test.go, and the styles those roles reach in
// named_lens_semantic_style_test.go.

func TestNamedLensSemanticRole(t *testing.T) {
	cases := []struct {
		name string
		lens config.NamedLens
		want namedLensRole
	}{
		// Rung 1: overdue outranks every other signal a definition can carry.
		{
			name: "overdue due bucket",
			lens: namedLensQueryFixture("@overdue"),
			want: namedLensRole{Kind: namedLensRoleOverdue},
		},
		{
			name: "overdue outranks waiting priority and mode",
			lens: namedLensFixture(func(l *config.NamedLens) {
				l.Mode = config.LensModeDue
				l.Query = fmt.Sprintf("@overdue $%s !%d", models.StateWaiting, models.PriorityHigh)
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
			lens: namedLensQueryFixture(
				fmt.Sprintf("$%s !%d", models.StateWaiting, models.PriorityHigh)),
			want: namedLensRole{Kind: namedLensRoleWaiting},
		},
		{
			name: "waiting outranks due today",
			lens: namedLensQueryFixture(fmt.Sprintf("$%s @today", models.StateWaiting)),
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
			lens: namedLensQueryFixture("@today"),
			want: namedLensRole{Kind: namedLensRoleDueToday},
		},
		{
			name: "due today outranks priority",
			lens: namedLensQueryFixture(fmt.Sprintf("@today !%d", models.PriorityHigh)),
			want: namedLensRole{Kind: namedLensRoleDueToday},
		},
		{
			name: "priority 1",
			lens: namedLensQueryFixture(fmt.Sprintf("!%d", models.PriorityHigh)),
			want: namedLensRole{Kind: namedLensRolePriority, Priority: models.PriorityHigh},
		},
		{
			name: "priority 2",
			lens: namedLensQueryFixture(fmt.Sprintf("!%d", models.PriorityMedium)),
			want: namedLensRole{Kind: namedLensRolePriority, Priority: models.PriorityMedium},
		},
		{
			name: "priority 3",
			lens: namedLensQueryFixture(fmt.Sprintf("!%d", models.PriorityLow)),
			want: namedLensRole{Kind: namedLensRolePriority, Priority: models.PriorityLow},
		},
		{
			name: "priority outranks the deferred filter",
			lens: namedLensQueryFixture(fmt.Sprintf("!%d ?deferred", models.PriorityLow)),
			want: namedLensRole{Kind: namedLensRolePriority, Priority: models.PriorityLow},
		},
		{
			// "!0" selects tasks carrying no priority, which is the absence of the
			// number the ladder colors by rather than a number of its own.
			name: "the no-priority clause is not a signal",
			lens: namedLensQueryFixture("!0"),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			// A priority outside the live range is not canonical vocabulary, so it
			// never becomes a clause and the ladder sees no priority at all.
			name: "an out-of-range priority is not a clause",
			lens: namedLensQueryFixture("!5"),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "a negated priority is not a signal",
			lens: namedLensQueryFixture(fmt.Sprintf("-!%d", models.PriorityHigh)),
			want: namedLensRole{Kind: namedLensRoleNone},
		},

		// Rung 6: the remaining active window, and the two buckets excluded from it.
		{
			name: "due this week",
			lens: namedLensQueryFixture("@week"),
			want: namedLensRole{Kind: namedLensRoleDue},
		},
		{
			name: "an exact calendar day is not an urgency accent",
			lens: namedLensQueryFixture("@2026-01-05"),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "the no-due clause is not an accent",
			lens: namedLensQueryFixture("@0"),
			want: namedLensRole{Kind: namedLensRoleNone},
		},

		// Rungs 7 to 9.
		{
			name: "deferred filter",
			lens: namedLensQueryFixture("?deferred"),
			want: namedLensRole{Kind: namedLensRoleDeferred},
		},
		{
			name: "deferred outranks the review flag",
			lens: namedLensQueryFixture("?deferred =review"),
			want: namedLensRole{Kind: namedLensRoleDeferred},
		},
		{
			name: "stored review flag",
			lens: namedLensQueryFixture("=review"),
			want: namedLensRole{Kind: namedLensRoleReview},
		},
		{
			name: "a review tag name is now ordinary vocabulary",
			lens: namedLensQueryFixture("#review"),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "review flag beside a descriptive tag",
			lens: namedLensQueryFixture("=review #ops"),
			want: namedLensRole{Kind: namedLensRoleReview},
		},
		{
			name: "a negated review flag assigns no color",
			lens: namedLensQueryFixture("-=review"),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "descriptive tags alone assign no color",
			lens: namedLensQueryFixture("#ops #infra"),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "completion age threshold",
			lens: namedLensQueryFixture("?completed:30d"),
			want: namedLensRole{Kind: namedLensRoleCompleted},
		},
		{
			name: "the bare completed facet is still an active facet",
			lens: namedLensQueryFixture("?completed"),
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
				l.Query = fmt.Sprintf("$%s $%s", models.StateInbox, models.StateDoing)
			}),
			want: modeRole(chrome.ModeInbox),
		},
		{
			// A negated state selects the complement of the group it names, so it is
			// deliberately not that group's color.
			name: "a negated state is not a group",
			lens: namedLensQueryFixture(fmt.Sprintf("-$%s", models.StateDoing)),
			want: namedLensRole{Kind: namedLensRoleNone},
		},

		// Rungs 11 and 12: population and staleness both read as a warning.
		{
			name: "parked-only population",
			lens: namedLensQueryFixture("+parked:only"),
			want: namedLensRole{Kind: namedLensRoleWarning},
		},
		{
			name: "parked-included population",
			lens: namedLensQueryFixture("+parked"),
			want: namedLensRole{Kind: namedLensRoleWarning},
		},
		{
			name: "a negated parked population is not a warning",
			lens: namedLensQueryFixture("-+parked"),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "archived-only population",
			lens: namedLensQueryFixture("+archived:only"),
			want: namedLensRole{Kind: namedLensRoleWarning},
		},
		{
			name: "pinned population",
			lens: namedLensQueryFixture("*pin"),
			want: namedLensRole{Kind: namedLensRoleWarning},
		},
		{
			name: "stale filter",
			lens: namedLensQueryFixture("?stale"),
			want: namedLensRole{Kind: namedLensRoleWarning},
		},
		{
			name: "a state group outranks the stale filter",
			lens: namedLensQueryFixture(fmt.Sprintf("$%s ?stale", models.StateDoing)),
			want: groupRole(models.StateGroupDoing),
		},

		// Rung 13: every non-default mode, and the default that is not a signal.
		{
			name: "inbox mode",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeInbox }),
			want: modeRole(chrome.ModeInbox),
		},
		{
			name: "today mode",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeToday }),
			want: modeRole(chrome.ModeToday),
		},
		{
			name: "next mode",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeNext }),
			want: modeRole(chrome.ModeNext),
		},
		{
			name: "backlog mode",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeBacklog }),
			want: modeRole(chrome.ModeBacklog),
		},
		{
			name: "due mode without a due bucket",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeDue }),
			want: modeRole(chrome.ModeDue),
		},
		{
			name: "recent mode",
			lens: namedLensFixture(func(l *config.NamedLens) { l.Mode = config.LensModeRecent }),
			want: modeRole(chrome.ModeRecent),
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
				l.Query = "?stale"
			}),
			want: namedLensRole{Kind: namedLensRoleWarning},
		},

		// Facets that must never assign a color on their own.
		{
			name: "unparseable query prose assigns no color",
			lens: namedLensQueryFixture("invoice"),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "a hierarchy filter alone assigns no color",
			lens: namedLensQueryFixture("?leaf"),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "project tags and untagged assign no color",
			lens: namedLensQueryFixture("+#client +#0"),
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
			// The unknown token never becomes a clause, so the recognized state
			// beside it is the whole set and still colors the row.
			name: "an unknown state leaves the recognized one standing",
			lens: namedLensStatesFixture(string(models.StateDoing), "almost-done"),
			want: groupRole(models.StateGroupDoing),
		},
		{
			name: "unknown due token",
			lens: namedLensQueryFixture("@fortnight"),
			want: namedLensRole{Kind: namedLensRoleNone},
		},
		{
			name: "unknown population token",
			lens: namedLensQueryFixture("+parked:maybe"),
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
			name: "an empty query",
			lens: config.NamedLens{ID: namedLensFixtureID, Name: "Empty", Mode: config.LensModeAll},
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
		after.Query = "$" + string(models.StateWaiting)

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
