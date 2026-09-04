package lens

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// A named lens wears one color, resolved from its definition alone. This file
// holds what the suite shares: the definition fixtures and the test-only
// spellings a failure message reads a role back through. The precedence ladder
// itself is in named_lens_semantic_role_test.go and the styles those roles reach
// in named_lens_semantic_style_test.go.

// namedLensFixtureID is an arbitrary well-formed identity. The resolver never
// reads it, which is exactly what the identity-independence cases prove.
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

// namedLensQueryFixture spells a narrowing as the saved query string, which is
// the only place the resolver reads a facet from.
func namedLensQueryFixture(query string) config.NamedLens {
	return namedLensFixture(func(lens *config.NamedLens) { lens.Query = query })
}

func namedLensStatesFixture(states ...string) config.NamedLens {
	tokens := make([]string, 0, len(states))
	for _, state := range states {
		tokens = append(tokens, "$"+state)
	}
	return namedLensQueryFixture(strings.Join(tokens, " "))
}

func groupRole(group models.WorkflowStateGroup) namedLensRole {
	return namedLensRole{Kind: namedLensRoleWorkflowGroup, Group: group}
}

func modeRole(mode chrome.OutlinerModeID) namedLensRole {
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
