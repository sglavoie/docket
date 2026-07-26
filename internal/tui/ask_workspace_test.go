package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/aiharness"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

func newAskWorkspaceFixture(t *testing.T, consent bool) (*config.RuntimeState, *MockPersister, *models.Project, *models.Project) {
	t.Helper()
	alpha, err := models.NewProject("alpha", "Active Alpha", "")
	if err != nil {
		t.Fatal(err)
	}
	beta, err := models.NewProject("beta", "Active Beta", "")
	if err != nil {
		t.Fatal(err)
	}
	alphaTask, err := models.NewTask("alpha-task", 1, models.TaskTypeTask, "Alpha private task")
	if err != nil {
		t.Fatal(err)
	}
	betaTask, err := models.NewTask("beta-task", 1, models.TaskTypeBug, "Beta private task")
	if err != nil {
		t.Fatal(err)
	}
	alpha.Tasks = []*models.Task{alphaTask}
	beta.Tasks = []*models.Task{betaTask}

	domains := config.NewDomains()
	domains.Catalog.UpsertProject(alpha)
	domains.Catalog.UpsertProject(beta)
	if consent {
		if err := domains.Settings.AIConsent.GrantFor(aiharness.Claude, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	runtime := config.NewRuntimeState(t.TempDir(), domains)
	persister := NewMockPersister()
	persister.Projects[alpha.ID] = alpha
	persister.Projects[beta.ID] = beta
	return runtime, persister, alpha, beta
}

func askWorkspaceLoadedMessage(t *testing.T, cmd tea.Cmd) AskWorkspaceLoadedMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("nil active-workspace load command")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("workspace command = %#v, want tea.BatchMsg", cmd())
	}
	msg, ok := batch[0]().(AskWorkspaceLoadedMsg)
	if !ok {
		t.Fatalf("workspace result = %T, want AskWorkspaceLoadedMsg", batch[0]())
	}
	return msg
}

func TestAskViewProductionWorkspaceSnapshotIgnoresStaleDashboardProject(t *testing.T) {
	runtime, persister, alpha, beta := newAskWorkspaceFixture(t, true)
	const staleDashboardProjectName = "SDR"
	runner := &fakeAskRunner{streams: []external.Stream{
		newScriptedAskStream([]byte(`{"version":2,"summary":"ok","actions":[]}`), 32, nil),
	}}
	m := NewAskViewModelForWorkspaceWithRunner("", runtime, nil, persister, runner)

	if m.project != nil {
		t.Fatalf("production Ask retained dashboard project %#v", m.project)
	}
	if m.workspaceLoader == nil {
		t.Fatal("production Ask did not install an active-workspace loader")
	}
	loadCmd := m.submitQuery("show my active work")
	if len(runner.Requests()) != 0 || len(persister.LoadCalls) != 0 {
		t.Fatalf("submission performed I/O before its Tea command: requests=%d loads=%d", len(runner.Requests()), len(persister.LoadCalls))
	}
	loaded := askWorkspaceLoadedMessage(t, loadCmd)
	if loaded.Err != nil {
		t.Fatalf("workspace load = %v", loaded.Err)
	}
	_, providerCmd := m.Update(loaded)
	driveAskCommand(t, m, providerCmd)

	if got := len(persister.LoadCalls); got != 2 {
		t.Fatalf("active project loads = %d, want 2", got)
	}
	requests := runner.Requests()
	if len(requests) != 1 {
		t.Fatalf("provider requests = %d, want 1", len(requests))
	}
	request := requests[0]
	for _, want := range []string{alpha.Name, beta.Name, alpha.Tasks[0].Title, beta.Tasks[0].Title} {
		if !strings.Contains(request.SystemPrompt, want) {
			t.Fatalf("workspace context missing %q: %q", want, request.SystemPrompt)
		}
	}
	if strings.Contains(request.SystemPrompt, staleDashboardProjectName) {
		t.Fatalf("stale dashboard project leaked into workspace context: %q", request.SystemPrompt)
	}
	if request.WorkspaceDir != runtime.DataDir() {
		t.Fatalf("workspace directory = %q, want %q", request.WorkspaceDir, runtime.DataDir())
	}
}

func TestAskViewWorkspaceCapturesRuntimeProjectsBeforeTeaCommand(t *testing.T) {
	runtime, persister, alpha, beta := newAskWorkspaceFixture(t, true)
	m := NewAskViewModelForWorkspaceWithRunner("", runtime, nil, persister, &fakeAskRunner{})

	loadCmd := m.submitQuery("show my active work")
	replacement, err := models.NewProject("replacement", "Replacement", "")
	if err != nil {
		t.Fatal(err)
	}
	replacementDomains := config.NewDomains()
	replacementDomains.Catalog.UpsertProject(replacement)
	if err := replacementDomains.Settings.AIConsent.GrantFor(aiharness.Claude, time.Now()); err != nil {
		t.Fatal(err)
	}
	runtime.SetDomainsInMemory(replacementDomains)

	loaded := askWorkspaceLoadedMessage(t, loadCmd)
	if loaded.Err != nil {
		t.Fatalf("workspace load = %v", loaded.Err)
	}
	if got := len(persister.LoadCalls); got != 2 {
		t.Fatalf("captured active project loads = %d, want 2", got)
	}
	for i, want := range []string{alpha.ID, beta.ID} {
		if got := persister.LoadCalls[i].ID; got != want {
			t.Fatalf("loaded project %d = %q, want captured %q", i, got, want)
		}
	}
}

func TestAskViewWorkspaceContextGatesLoadingAndInspection(t *testing.T) {
	for _, mode := range []external.TaskContextMode{external.TaskContextFull, external.TaskContextRedacted, external.TaskContextNone} {
		t.Run(string(mode), func(t *testing.T) {
			runtime, persister, alpha, _ := newAskWorkspaceFixture(t, true)
			runner := &fakeAskRunner{streams: []external.Stream{
				newScriptedAskStream([]byte(`{"version":2,"summary":"ok","actions":[]}`), 32, nil),
			}}
			m := NewAskViewModelForWorkspaceWithRunner("", runtime, nil, persister, runner)
			m.contextMode = mode

			cmd := m.submitQuery("private query")
			if mode == external.TaskContextNone {
				driveAskCommand(t, m, cmd)
				if len(persister.LoadCalls) != 0 {
					t.Fatalf("no-context request loaded %d projects", len(persister.LoadCalls))
				}
			} else {
				loaded := askWorkspaceLoadedMessage(t, cmd)
				_, providerCmd := m.Update(loaded)
				driveAskCommand(t, m, providerCmd)
				if len(persister.LoadCalls) != 2 {
					t.Fatalf("%s request loaded %d projects, want 2", mode, len(persister.LoadCalls))
				}
			}

			request := runner.Requests()[0]
			hasContext := strings.Contains(request.SystemPrompt, "<task_context_json>")
			switch mode {
			case external.TaskContextFull:
				if !hasContext || request.WorkspaceDir != runtime.DataDir() {
					t.Fatalf("full request = %+v", request)
				}
			case external.TaskContextRedacted:
				if !hasContext || strings.Contains(request.SystemPrompt, alpha.Name) || request.WorkspaceDir != "" {
					t.Fatalf("redacted request disclosed workspace data: %+v", request)
				}
			case external.TaskContextNone:
				if hasContext || request.WorkspaceDir != "" {
					t.Fatalf("no-context request disclosed workspace data: %+v", request)
				}
			}
		})
	}
}

func TestAskViewWorkspaceConsentResumeAndLoadFailure(t *testing.T) {
	t.Run("consent resume loads only after saved grant", func(t *testing.T) {
		runtime, persister, _, _ := newAskWorkspaceFixture(t, false)
		domains, err := runtime.CloneDomains()
		if err != nil {
			t.Fatal(err)
		}
		// The focused consent-resume test does not need on-disk domain commits;
		// keep the same active catalog while exercising the in-memory save path.
		runtime = config.NewRuntimeState("", domains)
		runner := &fakeAskRunner{streams: []external.Stream{
			newScriptedAskStream([]byte(`{"version":2,"summary":"ok","actions":[]}`), 32, nil),
		}}
		m := NewAskViewModelForWorkspaceWithRunner("", runtime, nil, persister, runner)
		m.textarea.SetValue("resume with context")

		_, preferenceCmd := m.handleKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
		if next := driveAskPreferenceSave(t, m, preferenceCmd); next != nil || m.state != askStateConsent {
			t.Fatalf("initial submission = state %v next %v", m.state, next)
		}
		if len(persister.LoadCalls) != 0 {
			t.Fatal("workspace loaded before consent was remembered")
		}
		consentMsg, ok := m.saveContextConsent()().(AskConsentSavedMsg)
		if !ok || consentMsg.Err != nil {
			t.Fatalf("save consent = %#v", consentMsg)
		}
		_, loadCmd := m.Update(consentMsg)
		loaded := askWorkspaceLoadedMessage(t, loadCmd)
		_, providerCmd := m.Update(loaded)
		driveAskCommand(t, m, providerCmd)

		if !runtime.Settings().AIConsent.GrantedFor(aiharness.Claude) || len(persister.LoadCalls) != 2 {
			t.Fatalf("resumed consent = %+v; loads=%d", runtime.Settings().AIConsent, len(persister.LoadCalls))
		}
	})

	t.Run("active project load failure never reaches provider", func(t *testing.T) {
		runtime, persister, _, _ := newAskWorkspaceFixture(t, true)
		persister.LoadError = errors.New("missing tasks file\x1b[31m")
		runner := &fakeAskRunner{}
		m := NewAskViewModelForWorkspaceWithRunner("", runtime, nil, persister, runner)

		loaded := askWorkspaceLoadedMessage(t, m.submitQuery("do not send partial context"))
		_, next := m.Update(loaded)
		if next != nil || m.state != askStateResult || len(runner.Requests()) != 0 {
			t.Fatalf("load failure = state %v next %v requests %#v", m.state, next, runner.Requests())
		}
		if !strings.Contains(m.response, "no AI request was sent") || strings.Contains(m.response, "\x1b") {
			t.Fatalf("unsafe load failure response: %q", m.response)
		}
	})
}

func TestAskViewWorkspaceResponsePreparesMultiProjectActionPlan(t *testing.T) {
	runtime, persister, alpha, beta := newAskWorkspaceFixture(t, true)
	service := app.NewService(newTestAppRepo(alpha, beta), nil, nil)
	title := "Create in beta"
	runner := &fakeAskRunner{streams: []external.Stream{
		newScriptedAskStream([]byte(`{"version":2,"summary":"planned","actions":[{"type":"create_task","project_id":"beta","title":"`+title+`"}]}`), 64, nil),
	}}
	m := NewAskViewModelForWorkspaceWithRunner("", runtime, service, persister, runner)
	loaded := askWorkspaceLoadedMessage(t, m.submitQuery("add a beta task"))
	_, providerCmd := m.Update(loaded)
	driveAskCommand(t, m, providerCmd)

	if m.prepared == nil {
		t.Fatalf("multi-project action plan was not prepared: %q", m.response)
	}
	preview := m.prepared.Preview()
	if len(preview.Actions) != 1 || !strings.Contains(preview.Actions[0].Description, beta.Name) {
		t.Fatalf("prepared action preview = %#v", preview)
	}
}
