package config

import "testing"

func TestGetInfoPanelMode_DefaultInfoCards(t *testing.T) {
	cfg := NewConfig()

	mode := cfg.GetInfoPanelMode()
	if mode != InfoPanelInfoCards {
		t.Errorf("expected InfoPanelInfoCards (1), got %d", mode)
	}
}

func TestGetInfoPanelMode_LegacyFalse(t *testing.T) {
	cfg := NewConfig()
	f := false
	cfg.ShowInfoPanel = &f

	mode := cfg.GetInfoPanelMode()
	if mode != InfoPanelOff {
		t.Errorf("expected InfoPanelOff (0), got %d", mode)
	}
}

func TestGetInfoPanelMode_LegacyTrue(t *testing.T) {
	cfg := NewConfig()
	tr := true
	cfg.ShowInfoPanel = &tr

	mode := cfg.GetInfoPanelMode()
	if mode != InfoPanelInfoCards {
		t.Errorf("expected InfoPanelInfoCards (1), got %d", mode)
	}
}

func TestGetInfoPanelMode_ExplicitMode(t *testing.T) {
	cfg := NewConfig()
	m := int(InfoPanelProjectSummary)
	cfg.InfoPanelMode = &m

	mode := cfg.GetInfoPanelMode()
	if mode != InfoPanelProjectSummary {
		t.Errorf("expected InfoPanelProjectSummary (2), got %d", mode)
	}
}

func TestGetInfoPanelMode_ClampOutOfRange(t *testing.T) {
	cfg := NewConfig()
	m := 99
	cfg.InfoPanelMode = &m

	mode := cfg.GetInfoPanelMode()
	if mode != InfoPanelInfoCards {
		t.Errorf("expected InfoPanelInfoCards (1) for out-of-range, got %d", mode)
	}
}

func TestGetInfoPanelMode_ClampNegative(t *testing.T) {
	cfg := NewConfig()
	m := -1
	cfg.InfoPanelMode = &m

	mode := cfg.GetInfoPanelMode()
	if mode != InfoPanelInfoCards {
		t.Errorf("expected InfoPanelInfoCards (1) for negative, got %d", mode)
	}
}

func TestCycleInfoPanelMode(t *testing.T) {
	cfg := NewConfig()

	mode := cfg.CycleInfoPanelMode()
	if mode != InfoPanelProjectSummary {
		t.Errorf("expected InfoPanelProjectSummary (2), got %d", mode)
	}

	mode = cfg.CycleInfoPanelMode()
	if mode != InfoPanelOff {
		t.Errorf("expected InfoPanelOff (0), got %d", mode)
	}

	mode = cfg.CycleInfoPanelMode()
	if mode != InfoPanelInfoCards {
		t.Errorf("expected InfoPanelInfoCards (1), got %d", mode)
	}
}

func TestSetInfoPanelMode_ClearsLegacy(t *testing.T) {
	cfg := NewConfig()
	tr := true
	cfg.ShowInfoPanel = &tr

	cfg.SetInfoPanelMode(InfoPanelProjectSummary)

	if cfg.ShowInfoPanel != nil {
		t.Error("SetInfoPanelMode should clear legacy ShowInfoPanel")
	}
	if cfg.InfoPanelMode == nil || *cfg.InfoPanelMode != int(InfoPanelProjectSummary) {
		t.Error("InfoPanelMode should be set to ProjectSummary")
	}
}

func TestGetShowInfoPanel_DelegatesToMode(t *testing.T) {
	cfg := NewConfig()
	cfg.SetInfoPanelMode(InfoPanelOff)
	if cfg.GetShowInfoPanel() {
		t.Error("GetShowInfoPanel should return false when mode is Off")
	}

	cfg.SetInfoPanelMode(InfoPanelInfoCards)
	if !cfg.GetShowInfoPanel() {
		t.Error("GetShowInfoPanel should return true when mode is InfoCards")
	}

	cfg.SetInfoPanelMode(InfoPanelProjectSummary)
	if !cfg.GetShowInfoPanel() {
		t.Error("GetShowInfoPanel should return true when mode is ProjectSummary")
	}
}

func TestGetShowInfoPanel_DefaultTrue(t *testing.T) {
	cfg := NewConfig()

	if !cfg.GetShowInfoPanel() {
		t.Error("GetShowInfoPanel should default to true")
	}
}

func TestGetShowInfoPanel_ExplicitFalse(t *testing.T) {
	cfg := NewConfig()
	cfg.SetShowInfoPanel(false)

	if cfg.GetShowInfoPanel() {
		t.Error("GetShowInfoPanel should return false when set to false")
	}
}

func TestGetShowInfoPanel_ExplicitTrue(t *testing.T) {
	cfg := NewConfig()
	cfg.SetShowInfoPanel(true)

	if !cfg.GetShowInfoPanel() {
		t.Error("GetShowInfoPanel should return true when set to true")
	}
}

func TestSetShowInfoPanel(t *testing.T) {
	cfg := NewConfig()

	cfg.SetShowInfoPanel(false)
	if cfg.ShowInfoPanel == nil {
		t.Error("ShowInfoPanel should not be nil after SetShowInfoPanel")
	}
	if *cfg.ShowInfoPanel {
		t.Error("ShowInfoPanel should be false")
	}

	cfg.SetShowInfoPanel(true)
	if !*cfg.ShowInfoPanel {
		t.Error("ShowInfoPanel should be true")
	}
}
