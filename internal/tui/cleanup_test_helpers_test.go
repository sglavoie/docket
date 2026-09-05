package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/operation"
)

func captureDestinationNamesForTest() []string {
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDestination)
	if !ok {
		return nil
	}
	return definition.Values
}

func pendingChordHintRowForTest(m *AppModel) string {
	if m == nil || m.operations.Registry == nil || len(m.operations.Chord) == 0 {
		return ""
	}
	ctx := m.operationCommandContext()
	ctx.View = m.currentView()
	hints := m.operations.Registry.DecoratedChordHintsForPrefix(
		operation.InputStrategySimplified,
		m.operations.Chord,
		ctx,
		m.crossOutliner.DecoratePendingChordHint,
	)
	parts := []string{strings.Join(m.operations.Chord, "") + "…"}
	for _, hint := range hints {
		parts = append(parts, hint.Key+" "+hint.Label)
	}
	return strings.Join(parts, " · ")
}

func jumpToOutlinerRootForTest(m *AppModel) {
	if m == nil {
		return
	}
	_, _ = m.handleProjectViewMessages(launchOutlinerMsg{})
}
