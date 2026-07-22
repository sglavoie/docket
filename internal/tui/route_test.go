package tui

import "testing"

func TestPresentationInventoryMatchesNames(t *testing.T) {
	presentations := allPresentations()
	if len(presentations) != len(viewTypeNames) {
		t.Fatalf("presentation inventory has %d entries, names have %d", len(presentations), len(viewTypeNames))
	}
	for _, presentation := range presentations {
		if _, ok := viewTypeNames[presentation]; !ok {
			t.Errorf("presentation %d has no name", presentation)
		}
	}
}

func TestPresentationScopes(t *testing.T) {
	tests := []struct {
		presentation PresentationID
		want         RouteScope
	}{
		{ViewDashboard, ScopeApp},
		{ViewCrossProjectOutliner, ScopeApp},
		{ViewOutliner, ScopeProject},
		{ViewTaskDetail, ScopeTask},
	}
	for _, test := range tests {
		if got := presentationScope(test.presentation); got != test.want {
			t.Errorf("presentationScope(%s) = %d, want %d", test.presentation, got, test.want)
		}
	}
}

func TestBuildViewAccentMap(t *testing.T) {
	buildViewAccentMap()
	if got := ViewAccentColor(ViewDashboard); got != AccentDashboard {
		t.Errorf("dashboard accent = %v, want %v", got, AccentDashboard)
	}
	if got := ViewAccentColor(ViewCommandPalette); got != AccentSearch {
		t.Errorf("palette accent = %v, want %v", got, AccentSearch)
	}
	if got := ViewAccentColor(ViewProjectCreator); got != nil {
		t.Errorf("form accent = %v, want inherited nil", got)
	}
}
