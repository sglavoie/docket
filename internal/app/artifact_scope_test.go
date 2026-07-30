package app_test

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
)

// TestClassifyArtifact pins the durable-versus-machine-local split so a durable
// artifact is never routed to machine.json and a machine-local artifact is never
// committed to the shared workspace.
func TestClassifyArtifact(t *testing.T) {
	durable := []app.ArtifactKind{
		app.ArtifactTasks, app.ArtifactNotes,
		app.ArtifactProjectMetadata, app.ArtifactToday, app.ArtifactNextOrder,
	}
	for _, k := range durable {
		if app.ClassifyArtifact(k) != app.ScopeDurable {
			t.Errorf("ClassifyArtifact(%q) = %v, want ScopeDurable", k, app.ClassifyArtifact(k))
		}
		if !k.IsDurable() || k.IsMachineLocal() {
			t.Errorf("%q IsDurable/IsMachineLocal misclassified", k)
		}
	}

	machineLocal := []app.ArtifactKind{
		app.ArtifactEditors, app.ArtifactIntegrations, app.ArtifactNavigation,
		app.ArtifactHistories, app.ArtifactLayout, app.ArtifactRecentVisits,
	}
	for _, k := range machineLocal {
		if app.ClassifyArtifact(k) != app.ScopeMachineLocal {
			t.Errorf("ClassifyArtifact(%q) = %v, want ScopeMachineLocal", k, app.ClassifyArtifact(k))
		}
		if !k.IsMachineLocal() || k.IsDurable() {
			t.Errorf("%q IsMachineLocal/IsDurable misclassified", k)
		}
	}

	if got := app.ClassifyArtifact(app.ArtifactKind("nonsense")); got != app.ScopeUnknown {
		t.Errorf("ClassifyArtifact(nonsense) = %v, want ScopeUnknown", got)
	}
}
