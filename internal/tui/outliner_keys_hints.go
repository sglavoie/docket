package tui

func (m *OutlinerModel) HandledKeys() []string {
	if m.operationRegistry == nil {
		return nil
	}
	return m.operationRegistry.HandledKeysForInputStrategy(InputStrategySimplified)
}

func (m *OutlinerModel) GetHelpItems() []HelpItem {
	if m.operationRegistry == nil {
		return nil
	}
	return m.operationRegistry.HelpItemsForInputStrategy(
		InputStrategySimplified,
		m.operationCommandContext(),
	)
}

func (m *OutlinerModel) ContextualFooter() (StatusBarFooterSpec, bool) {
	if m.operationRegistry == nil {
		return StatusBarFooterSpec{}, false
	}
	hints := m.operationRegistry.FooterHintsForInputStrategy(
		InputStrategySimplified,
		m.operationCommandContext(),
		8,
	)
	if hints == "" {
		return StatusBarFooterSpec{}, false
	}
	if m.runtime != nil {
		if right := onboardingStatus(m.runtime.Settings().GetOnboardingProgress()); right != "" {
			hints += " · " + right
		}
	}
	return StatusBarFooterSpec{Left: hints}, true
}
