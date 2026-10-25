package lens

import (
	"fmt"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/query"
)

// The token tables are lens vocabulary rather than Outliner vocabulary: they are
// the only conversion between a TUI enum and the durable spelling a saved lens
// carries, so no other TUI code reads a persisted spelling and no config code
// learns a TUI enum. They live here because both readers — the Outliner state
// this package is applied to, and the summary and accent this package renders —
// are above them, and a table in either one would make the other reach sideways.

// axis maps one TUI enum onto its durable token set. The table is the single
// source of truth for that axis in both directions; the reverse lookup scans it
// rather than duplicating it into a second map that could drift out of step.
type axis[T comparable] struct {
	name   string
	tokens map[T]string
	// fallback is the value the empty token decodes to. config writes the empty
	// string for an axis left at its default, so decoding must read it back as
	// that default rather than as garbage.
	fallback T
	// requiresToken marks the one axis, mode, that has no default rendering: an
	// empty token there is malformed rather than "left alone".
	requiresToken bool
}

// Token renders one value as the durable spelling.
func (a axis[T]) Token(value T) string {
	if token, ok := a.tokens[value]; ok {
		return token
	}
	if a.requiresToken {
		return ""
	}
	return a.tokens[a.fallback]
}

// Value decodes one durable spelling, refusing a token this build cannot name.
func (a axis[T]) Value(token string) (T, error) {
	if token == "" && !a.requiresToken {
		return a.fallback, nil
	}
	for value, candidate := range a.tokens {
		if candidate == token {
			return value, nil
		}
	}
	var zero T
	return zero, fmt.Errorf("lens %s %q is not a known value", a.name, token)
}

// ModeAxis is the mode half of a saved definition.
var ModeAxis = axis[chrome.OutlinerModeID]{
	name: "mode",
	tokens: map[chrome.OutlinerModeID]string{
		chrome.ModeAll:     config.LensModeAll,
		chrome.ModeInbox:   config.LensModeInbox,
		chrome.ModeToday:   config.LensModeToday,
		chrome.ModeNext:    config.LensModeNext,
		chrome.ModeBacklog: config.LensModeBacklog,
		chrome.ModeDue:     config.LensModeDue,
		chrome.ModeRecent:  config.LensModeRecent,
	},
	fallback:      chrome.ModeAll,
	requiresToken: true,
}

// SortAxis is the manual-sort half of a saved definition.
var SortAxis = axis[query.SortField]{
	name: "sort field",
	tokens: map[query.SortField]string{
		query.SortByDefault:   config.LensSortDefault,
		query.SortByPriority:  config.LensSortPriority,
		query.SortByDueDate:   config.LensSortDue,
		query.SortByCreatedAt: config.LensSortCreated,
		query.SortByUpdatedAt: config.LensSortUpdated,
		query.SortByTitle:     config.LensSortTitle,
	},
	fallback: query.SortByDefault,
}
