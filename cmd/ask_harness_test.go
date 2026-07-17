package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/aiharness"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

const emptyAskPlan = `{"version":1,"summary":"Done","actions":[]}`

type askRunnerFunc func(context.Context, external.Request) (string, error)

func (f askRunnerFunc) Run(ctx context.Context, request external.Request) (string, error) {
	return f(ctx, request)
}

func setAskTestFlags(t *testing.T, apply, noContext, redact bool, harness, configDir string) {
	t.Helper()
	oldApply, oldNoContext, oldRedact := askApply, askNoContext, askRedact
	oldHarness, oldConfigDir := askHarness, askConfigDir
	askApply, askNoContext, askRedact = apply, noContext, redact
	askHarness, askConfigDir = harness, configDir
	t.Cleanup(func() {
		askApply, askNoContext, askRedact = oldApply, oldNoContext, oldRedact
		askHarness, askConfigDir = oldHarness, oldConfigDir
	})
}

func newAskTestCommand(input string) (*cobra.Command, *bytes.Buffer) {
	cmd := &cobra.Command{Use: "ask"}
	cmd.SetIn(strings.NewReader(input))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	return cmd, &out
}

func setupAskTest(t *testing.T, mutate func(*config.RuntimeState)) *testDeps {
	t.Helper()
	deps := setupTestDeps(t)
	if mutate != nil {
		mutate(deps.Config)
	}
	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })
	testutil.AssertNoError(t, saveTestRuntime(config.NewManager(deps.ConfigPath), deps.Config))
	oldRepository := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldRepository })
	return deps
}

func TestAskCommandRegistersHarnessFlags(t *testing.T) {
	for _, name := range []string{"harness", "config-dir"} {
		if askCmd.Flags().Lookup(name) == nil {
			t.Fatalf("tt ask flag --%s is not registered", name)
		}
	}
}

func TestResolveAskHarnessPrecedenceAndValidation(t *testing.T) {
	machine := config.NewMachineState()
	if got, err := resolveAskHarness(machine, ""); err != nil || got != aiharness.Claude {
		t.Fatalf("legacy default = %q, %v; want claude", got, err)
	}
	testutil.AssertNoError(t, machine.SetAIHarness(aiharness.Codex))
	if got, err := resolveAskHarness(machine, ""); err != nil || got != aiharness.Codex {
		t.Fatalf("remembered default = %q, %v; want codex", got, err)
	}
	if got, err := resolveAskHarness(machine, "copilot"); err != nil || got != aiharness.Copilot {
		t.Fatalf("CLI override = %q, %v; want copilot", got, err)
	}
	if _, err := resolveAskHarness(machine, "gemini"); err == nil || !strings.Contains(err.Error(), "invalid --harness") {
		t.Fatalf("invalid override error = %v", err)
	}
}

func TestResolveAskConfigDirPrecedence(t *testing.T) {
	remembered := t.TempDir()
	cliOverride := t.TempDir()
	machine := config.NewMachineState()
	testutil.AssertNoError(t, machine.SetAIConfigDir(aiharness.Codex, remembered))

	if got, err := resolveAskConfigDir(machine, aiharness.Codex, "", false); err != nil || got != remembered {
		t.Fatalf("remembered config dir = %q, %v; want %q", got, err, remembered)
	}
	if got, err := resolveAskConfigDir(machine, aiharness.Codex, cliOverride, true); err != nil || got != cliOverride {
		t.Fatalf("CLI config dir = %q, %v; want %q", got, err, cliOverride)
	}
	t.Setenv("COPILOT_HOME", filepath.Join(t.TempDir(), "provider-owned"))
	if got, err := resolveAskConfigDir(machine, aiharness.Copilot, "", false); err != nil || got != "" {
		t.Fatalf("inherited config dir request override = %q, %v; want empty", got, err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	machine.AIConfigDirs = map[string]string{"claude": missing}
	if _, err := resolveAskConfigDir(machine, aiharness.Claude, "", false); err == nil {
		t.Fatal("missing remembered config directory was accepted")
	}
}

func TestRunAskConsumesPersistedTUIDefaults(t *testing.T) {
	rememberedConfig := t.TempDir()
	setupAskTest(t, func(runtime *config.RuntimeState) {
		// The TUI persists these values through the same machine domain before it
		// starts a request. A CLI invocation without flags must consume them.
		testutil.AssertNoError(t, runtime.Machine().SetAIHarness(aiharness.Copilot))
		testutil.AssertNoError(t, runtime.Machine().SetAIConfigDir(aiharness.Copilot, rememberedConfig))
	})
	setAskTestFlags(t, false, true, false, "", "")
	cmd, _ := newAskTestCommand("")
	var request external.Request
	runner := askRunnerFunc(func(_ context.Context, got external.Request) (string, error) {
		request = got
		return emptyAskPlan, nil
	})

	if err := runAskWithRunner(cmd, []string{"question"}, runner, false); err != nil {
		t.Fatalf("runAskWithRunner: %v", err)
	}
	if request.Harness != aiharness.Copilot || request.ConfigDir != rememberedConfig {
		t.Fatalf("request = %+v, want persisted Copilot defaults", request)
	}
}

func TestRunAskUsesCLIOverridesWithoutPersistingThem(t *testing.T) {
	rememberedCodex := t.TempDir()
	rememberedCopilot := t.TempDir()
	cliConfig := t.TempDir()
	deps := setupAskTest(t, func(runtime *config.RuntimeState) {
		testutil.AssertNoError(t, runtime.Machine().SetAIHarness(aiharness.Codex))
		testutil.AssertNoError(t, runtime.Machine().SetAIConfigDir(aiharness.Codex, rememberedCodex))
		testutil.AssertNoError(t, runtime.Machine().SetAIConfigDir(aiharness.Copilot, rememberedCopilot))
	})
	setAskTestFlags(t, false, true, false, "copilot", cliConfig)
	cmd, _ := newAskTestCommand("")
	var requests []external.Request
	runner := askRunnerFunc(func(_ context.Context, request external.Request) (string, error) {
		requests = append(requests, request)
		return emptyAskPlan, nil
	})

	if err := runAskWithRunner(cmd, []string{"question"}, runner, false); err != nil {
		t.Fatalf("runAskWithRunner: %v", err)
	}
	if len(requests) != 1 || requests[0].Harness != aiharness.Copilot || requests[0].ConfigDir != cliConfig {
		t.Fatalf("requests = %#v, want one Copilot request using CLI config", requests)
	}
	reloaded, err := config.NewManager(deps.ConfigPath).LoadRuntimeState()
	testutil.AssertNoError(t, err)
	if reloaded.Machine().GetAIHarness() != aiharness.Codex {
		t.Fatalf("CLI harness override persisted as %q", reloaded.Machine().GetAIHarness())
	}
	if got := reloaded.Machine().GetAIConfigDir(aiharness.Copilot); got != rememberedCopilot {
		t.Fatalf("CLI config override persisted as %q, want remembered %q", got, rememberedCopilot)
	}
}

func TestRunAskRetryUsesAlternativeHarnessRememberedConfig(t *testing.T) {
	cliConfig := t.TempDir()
	rememberedCopilot := t.TempDir()
	setupAskTest(t, func(runtime *config.RuntimeState) {
		testutil.AssertNoError(t, runtime.Machine().SetAIConfigDir(aiharness.Copilot, rememberedCopilot))
	})
	setAskTestFlags(t, false, true, false, "codex", cliConfig)
	cmd, _ := newAskTestCommand("copilot\n")
	var requests []external.Request
	runner := askRunnerFunc(func(_ context.Context, request external.Request) (string, error) {
		requests = append(requests, request)
		if request.Harness == aiharness.Codex {
			return "", errors.New("codex failed")
		}
		return emptyAskPlan, nil
	})

	if err := runAskWithRunner(cmd, []string{"question"}, runner, true); err != nil {
		t.Fatalf("runAskWithRunner: %v", err)
	}
	if len(requests) != 2 || requests[0].ConfigDir != cliConfig || requests[1].ConfigDir != rememberedCopilot {
		t.Fatalf("retry config selection = %#v", requests)
	}
}

func TestRunAskRetryContinuesAfterAlternativeConfigDirDisappears(t *testing.T) {
	rememberedCodex := filepath.Join(t.TempDir(), "codex-\x1b[31mremoved")
	testutil.AssertNoError(t, os.Mkdir(rememberedCodex, 0o700))
	setupAskTest(t, func(runtime *config.RuntimeState) {
		testutil.AssertNoError(t, runtime.Machine().SetAIConfigDir(aiharness.Codex, rememberedCodex))
	})
	testutil.AssertNoError(t, os.Remove(rememberedCodex))
	setAskTestFlags(t, false, true, false, "", "")
	cmd, out := newAskTestCommand("codex\ncopilot\n")
	var requests []external.Request
	runner := askRunnerFunc(func(_ context.Context, request external.Request) (string, error) {
		requests = append(requests, request)
		if request.Harness == aiharness.Claude {
			return "", errors.New("claude failed")
		}
		return emptyAskPlan, nil
	})

	if err := runAskWithRunner(cmd, []string{"question"}, runner, true); err != nil {
		t.Fatalf("runAskWithRunner: %v\noutput: %s", err, out.String())
	}
	want := []aiharness.Name{aiharness.Claude, aiharness.Copilot}
	got := make([]aiharness.Name, len(requests))
	for i, request := range requests {
		got[i] = request.Harness
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("provider requests = %v, want %v", got, want)
	}
	if gotOutput := out.String(); !strings.Contains(gotOutput, "resolve Codex configuration directory") ||
		!strings.Contains(gotOutput, `codex-\x1b[31mremoved`) || strings.Contains(gotOutput, "\x1b") ||
		strings.Count(gotOutput, "Retry with another harness? [codex/copilot/N]") != 1 ||
		strings.Count(gotOutput, "Retry with another harness? [copilot/N]") != 1 {
		t.Fatalf("retry output did not report the config failure and continue: %q", gotOutput)
	}
}

func TestRunAskConfigDirErrorsAreTerminalSafeAcrossRetryExits(t *testing.T) {
	t.Run("initial harness", func(t *testing.T) {
		rememberedClaude := filepath.Join(t.TempDir(), "claude-\x1b[31mremoved")
		testutil.AssertNoError(t, os.Mkdir(rememberedClaude, 0o700))
		setupAskTest(t, func(runtime *config.RuntimeState) {
			testutil.AssertNoError(t, runtime.Machine().SetAIConfigDir(aiharness.Claude, rememberedClaude))
		})
		testutil.AssertNoError(t, os.Remove(rememberedClaude))
		setAskTestFlags(t, false, true, false, "", "")
		cmd, _ := newAskTestCommand("")
		calls := 0
		runner := askRunnerFunc(func(context.Context, external.Request) (string, error) {
			calls++
			return emptyAskPlan, nil
		})

		err := runAskWithRunner(cmd, []string{"question"}, runner, false)
		assertTerminalSafeConfigDirError(t, err, rememberedClaude)
		if calls != 0 {
			t.Fatalf("runner called %d times, want 0", calls)
		}
	})

	t.Run("declined retry", func(t *testing.T) {
		rememberedCodex := filepath.Join(t.TempDir(), "codex-\x1b[31mremoved")
		testutil.AssertNoError(t, os.Mkdir(rememberedCodex, 0o700))
		setupAskTest(t, func(runtime *config.RuntimeState) {
			testutil.AssertNoError(t, runtime.Machine().SetAIConfigDir(aiharness.Codex, rememberedCodex))
		})
		testutil.AssertNoError(t, os.Remove(rememberedCodex))
		setAskTestFlags(t, false, true, false, "", "")
		cmd, out := newAskTestCommand("codex\nn\n")
		calls := 0
		runner := askRunnerFunc(func(context.Context, external.Request) (string, error) {
			calls++
			return "", errors.New("provider unavailable")
		})

		err := runAskWithRunner(cmd, []string{"question"}, runner, true)
		assertTerminalSafeConfigDirError(t, err, rememberedCodex)
		if calls != 1 {
			t.Fatalf("runner called %d times, want only the initial harness", calls)
		}
		if got := out.String(); strings.Contains(got, "\x1b") || !strings.Contains(got, strconv.Quote(rememberedCodex)) {
			t.Fatalf("retry output was not terminal-safe: %q", got)
		}
	})

	t.Run("final harness exhaustion", func(t *testing.T) {
		rememberedCopilot := filepath.Join(t.TempDir(), "copilot-\x1b[31mremoved")
		testutil.AssertNoError(t, os.Mkdir(rememberedCopilot, 0o700))
		setupAskTest(t, func(runtime *config.RuntimeState) {
			testutil.AssertNoError(t, runtime.Machine().SetAIConfigDir(aiharness.Copilot, rememberedCopilot))
		})
		testutil.AssertNoError(t, os.Remove(rememberedCopilot))
		setAskTestFlags(t, false, true, false, "", "")
		cmd, _ := newAskTestCommand("codex\ncopilot\n")
		calls := 0
		runner := askRunnerFunc(func(context.Context, external.Request) (string, error) {
			calls++
			return "", errors.New("provider unavailable")
		})

		err := runAskWithRunner(cmd, []string{"question"}, runner, true)
		assertTerminalSafeConfigDirError(t, err, rememberedCopilot)
		if !strings.Contains(err.Error(), "all supported AI harnesses were tried once") {
			t.Fatalf("exhaustion error = %q", err)
		}
		if calls != 2 {
			t.Fatalf("runner called %d times, want the two valid-config harnesses", calls)
		}
	})
}

func assertTerminalSafeConfigDirError(t *testing.T, err error, path string) {
	t.Helper()
	if err == nil {
		t.Fatal("config-directory failure unexpectedly succeeded")
	}
	got := err.Error()
	assertTerminalSafeSingleLine(t, got)
	wantPath := strconv.Quote(filepath.Clean(path))
	if !strings.Contains(got, wantPath) {
		t.Fatalf("error = %q, want quoted normalized path %s", got, wantPath)
	}
}

func TestRunAskRejectsInvalidCLIConfigBeforeInvokingProvider(t *testing.T) {
	setupAskTest(t, nil)
	setAskTestFlags(t, false, true, false, "codex", filepath.Join(t.TempDir(), "missing"))
	cmd, _ := newAskTestCommand("")
	calls := 0
	runner := askRunnerFunc(func(context.Context, external.Request) (string, error) {
		calls++
		return emptyAskPlan, nil
	})

	err := runAskWithRunner(cmd, []string{"question"}, runner, false)
	if err == nil || !strings.Contains(err.Error(), "invalid --config-dir") || calls != 0 {
		t.Fatalf("invalid config error = %v, calls = %d", err, calls)
	}
}

func TestRunAskProviderSpecificRedactedConsent(t *testing.T) {
	deps, project := setupStateTest(t)
	settings := deps.Config.Settings()
	settings.AIConsent.Granted = true
	settings.AIConsent.GrantedAt = nil
	deps.Config.SetSettingsInMemory(settings)
	testutil.AssertNoError(t, deps.Config.Machine().SetAIHarness(aiharness.Codex))
	testutil.AssertNoError(t, saveTestRuntime(config.NewManager(deps.ConfigPath), deps.Config))
	setAskTestFlags(t, false, false, true, "", "")
	cmd, out := newAskTestCommand("yes\n")
	var request external.Request
	runner := askRunnerFunc(func(_ context.Context, got external.Request) (string, error) {
		request = got
		return emptyAskPlan, nil
	})

	if err := runAskWithRunner(cmd, []string{"question"}, runner, true); err != nil {
		t.Fatalf("runAskWithRunner: %v", err)
	}
	if request.Harness != aiharness.Codex {
		t.Fatalf("request harness = %q, want codex", request.Harness)
	}
	if !strings.Contains(out.String(), "configured Codex CLI provider") ||
		!strings.Contains(out.String(), "all active projects") ||
		!strings.Contains(out.String(), "read-only") ||
		!strings.Contains(out.String(), "strict JSON action-plan") {
		t.Fatalf("consent disclosure did not name Codex: %q", out.String())
	}
	if !strings.Contains(request.SystemPrompt, "<task_context_json>") || strings.Contains(request.SystemPrompt, project.Name) {
		t.Fatalf("redacted task context was not sent correctly: %q", request.SystemPrompt)
	}
	reloaded, err := config.NewManager(deps.ConfigPath).LoadRuntimeState()
	testutil.AssertNoError(t, err)
	if !reloaded.Settings().AIConsent.GrantedFor(aiharness.Codex) || reloaded.Settings().AIConsent.GrantedFor(aiharness.Copilot) {
		t.Fatalf("provider consent leaked or was not persisted: %+v", reloaded.Settings().AIConsent)
	}
}

func TestRunAskContextModesAndActionPlansAreHarnessIndependent(t *testing.T) {
	for _, harness := range aiharness.Names() {
		for _, mode := range []external.TaskContextMode{
			external.TaskContextFull,
			external.TaskContextRedacted,
			external.TaskContextNone,
		} {
			t.Run(harness.String()+"/"+string(mode), func(t *testing.T) {
				deps, project := setupStateTest(t)
				testutil.AssertNoError(t, deps.Config.Machine().SetAIHarness(harness))
				settings := deps.Config.Settings()
				testutil.AssertNoError(t, settings.AIConsent.GrantFor(harness, time.Now()))
				deps.Config.SetSettingsInMemory(settings)
				testutil.AssertNoError(t, saveTestRuntime(config.NewManager(deps.ConfigPath), deps.Config))

				setAskTestFlags(t, false, mode == external.TaskContextNone, mode == external.TaskContextRedacted, "", "")
				cmd, _ := newAskTestCommand("")
				var request external.Request
				runner := askRunnerFunc(func(_ context.Context, got external.Request) (string, error) {
					request = got
					return emptyAskPlan, nil
				})

				if err := runAskWithRunner(cmd, []string{"question"}, runner, false); err != nil {
					t.Fatalf("runAskWithRunner: %v", err)
				}
				if request.Harness != harness {
					t.Fatalf("request harness = %q, want %q", request.Harness, harness)
				}
				hasContext := strings.Contains(request.SystemPrompt, "<task_context_json>")
				switch mode {
				case external.TaskContextFull:
					if !hasContext || !strings.Contains(request.SystemPrompt, project.Name) || request.WorkspaceDir != deps.Config.DataDir() {
						t.Fatalf("full context missing from prompt: %q", request.SystemPrompt)
					}
				case external.TaskContextRedacted:
					if !hasContext || strings.Contains(request.SystemPrompt, project.Name) || request.WorkspaceDir != "" {
						t.Fatalf("redacted context mismatch: %q", request.SystemPrompt)
					}
				case external.TaskContextNone:
					if hasContext || request.WorkspaceDir != "" {
						t.Fatalf("no-context request included task context: %q", request.SystemPrompt)
					}
				}
				containsCodexInspection := strings.Contains(request.SystemPrompt, "tt project list --json")
				if wantCodexInspection := harness == aiharness.Codex && mode == external.TaskContextFull; containsCodexInspection != wantCodexInspection {
					t.Fatalf("Codex live-inspection prompt = %t, want %t: %q", containsCodexInspection, wantCodexInspection, request.SystemPrompt)
				}
			})
		}
	}
}

func TestRunAskBuildsAllActiveProjectContextInsteadOfCurrentProject(t *testing.T) {
	deps, current := setupStateTest(t)
	other, err := models.NewProject("ask-other-active", "Other Active Project", "")
	testutil.AssertNoError(t, err)
	otherTask, err := models.NewTask("ask-other-task", 1, models.TaskTypeTask, "Other active task")
	testutil.AssertNoError(t, err)
	other.Tasks = []*models.Task{otherTask}
	other.NextShortID = 2
	deps.Repository.Projects[other.ID] = other
	addTestProject(deps.Config, other)
	settings := deps.Config.Settings()
	testutil.AssertNoError(t, settings.AIConsent.GrantFor(aiharness.Claude, time.Now()))
	deps.Config.SetSettingsInMemory(settings)
	testutil.AssertNoError(t, saveTestRuntime(config.NewManager(deps.ConfigPath), deps.Config))

	setAskTestFlags(t, false, false, false, "", "")
	cmd, _ := newAskTestCommand("")
	var request external.Request
	runner := askRunnerFunc(func(_ context.Context, got external.Request) (string, error) {
		request = got
		return emptyAskPlan, nil
	})

	if err := runAskWithRunner(cmd, []string{"summarize active work"}, runner, false); err != nil {
		t.Fatalf("runAskWithRunner: %v", err)
	}
	for _, want := range []string{current.Name, current.Tasks[0].Title, other.Name, otherTask.Title, `"projects":[`} {
		if !strings.Contains(request.SystemPrompt, want) {
			t.Fatalf("active workspace context missing %q: %q", want, request.SystemPrompt)
		}
	}
	if request.WorkspaceDir != deps.Config.DataDir() {
		t.Fatalf("workspace directory = %q, want %q", request.WorkspaceDir, deps.Config.DataDir())
	}
}

func TestRunAskDeclinedContextAppliesToEveryRetry(t *testing.T) {
	setupAskTest(t, nil)
	setAskTestFlags(t, false, false, false, "", "")
	cmd, out := newAskTestCommand("n\ncodex\n")
	var requests []external.Request
	runner := askRunnerFunc(func(_ context.Context, request external.Request) (string, error) {
		requests = append(requests, request)
		if request.Harness == aiharness.Claude {
			return "", errors.New("provider unavailable")
		}
		return emptyAskPlan, nil
	})

	if err := runAskWithRunner(cmd, []string{"question"}, runner, true); err != nil {
		t.Fatalf("runAskWithRunner: %v", err)
	}
	if len(requests) != 2 || requests[0].Harness != aiharness.Claude || requests[1].Harness != aiharness.Codex {
		t.Fatalf("retry requests = %#v", requests)
	}
	for _, request := range requests {
		if strings.Contains(request.SystemPrompt, "<task_context_json>") {
			t.Fatalf("declined task context sent to %s", request.Harness)
		}
	}
	if got := strings.Count(out.String(), "AI context disclosure:"); got != 1 {
		t.Fatalf("consent disclosure count = %d, want 1; output=%q", got, out.String())
	}
}

func TestRunAskConsentedContextLoadFailureAbortsBeforeProvider(t *testing.T) {
	deps, _ := setupStateTest(t)
	settings := deps.Config.Settings()
	testutil.AssertNoError(t, settings.AIConsent.GrantFor(aiharness.Claude, time.Now()))
	deps.Config.SetSettingsInMemory(settings)
	testutil.AssertNoError(t, saveTestRuntime(config.NewManager(deps.ConfigPath), deps.Config))
	deps.Repository.LoadError = errors.New("missing tasks file\x1b[31m\nprovider must not run")

	setAskTestFlags(t, false, false, false, "", "")
	cmd, out := newAskTestCommand("")
	calls := 0
	runner := askRunnerFunc(func(context.Context, external.Request) (string, error) {
		calls++
		return emptyAskPlan, nil
	})

	err := runAskWithRunner(cmd, []string{"question"}, runner, true)
	if err == nil {
		t.Fatal("corrupt consented context unexpectedly reached the provider")
	}
	if calls != 0 {
		t.Fatalf("runner called %d times, want 0", calls)
	}
	got := err.Error()
	assertTerminalSafeSingleLine(t, got)
	for _, want := range []string{"no AI request was sent", "--no-context", `\x1b[31m\nprovider must not run`} {
		if !strings.Contains(got, want) {
			t.Fatalf("context-load error = %q, want %q", got, want)
		}
	}
	if strings.Contains(out.String(), "Retry with another harness?") {
		t.Fatalf("provider-independent context failure entered retry: %q", out.String())
	}
}

func assertTerminalSafeSingleLine(t *testing.T, value string) {
	t.Helper()
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			t.Fatalf("value contains unsafe terminal character U+%04X: %q", r, value)
		}
	}
}

func TestRunAskRetriesEachHarnessAtMostOnce(t *testing.T) {
	setupAskTest(t, nil)
	setAskTestFlags(t, false, true, false, "", "")
	cmd, _ := newAskTestCommand("codex\ncopilot\n")
	var attempted []aiharness.Name
	runner := askRunnerFunc(func(_ context.Context, request external.Request) (string, error) {
		attempted = append(attempted, request.Harness)
		return "", fmt.Errorf("%s failed", request.Harness)
	})

	err := runAskWithRunner(cmd, []string{"question"}, runner, true)
	if err == nil || !strings.Contains(err.Error(), "all supported AI harnesses were tried once") {
		t.Fatalf("retry error = %v", err)
	}
	want := []aiharness.Name{aiharness.Claude, aiharness.Codex, aiharness.Copilot}
	if fmt.Sprint(attempted) != fmt.Sprint(want) {
		t.Fatalf("attempted = %v, want %v", attempted, want)
	}
}

func TestRunAskNonInteractiveFailureReturnsHarnessGuidance(t *testing.T) {
	setupAskTest(t, nil)
	setAskTestFlags(t, false, true, false, "", "")
	cmd, out := newAskTestCommand("")
	calls := 0
	runner := askRunnerFunc(func(context.Context, external.Request) (string, error) {
		calls++
		return "", errors.New("not found")
	})

	err := runAskWithRunner(cmd, []string{"question"}, runner, false)
	if err == nil || !strings.Contains(err.Error(), "stdin is non-interactive") ||
		!strings.Contains(err.Error(), "--harness codex") || !strings.Contains(err.Error(), "--harness copilot") {
		t.Fatalf("non-interactive error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("runner called %d times, want 1", calls)
	}
	if strings.Contains(out.String(), "Retry with another harness?") {
		t.Fatalf("non-interactive invocation prompted: %q", out.String())
	}
}

func TestRunAskCancellationIsNotRetried(t *testing.T) {
	setupAskTest(t, nil)
	setAskTestFlags(t, false, true, false, "", "")
	cmd, _ := newAskTestCommand("codex\n")
	calls := 0
	runner := askRunnerFunc(func(context.Context, external.Request) (string, error) {
		calls++
		return "", context.Canceled
	})

	err := runAskWithRunner(cmd, []string{"question"}, runner, true)
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("cancellation error = %v, calls = %d", err, calls)
	}
}

func TestRunAskRetriesOnlyBeforePreviewAndAppliesOnce(t *testing.T) {
	deps, project := setupStateTest(t)
	project.BuildTaskTree()
	initialCount := len(project.Tasks)
	setAskTestFlags(t, true, true, false, "", "")
	cmd, out := newAskTestCommand("codex\ny\n")
	var attempted []aiharness.Name
	runner := askRunnerFunc(func(_ context.Context, request external.Request) (string, error) {
		attempted = append(attempted, request.Harness)
		if request.Harness == aiharness.Claude {
			return `{"version":1,"summary":"Bad target","actions":[{"type":"create_task","project_id":"missing","title":"Must not apply"}]}`, nil
		}
		return fmt.Sprintf(`{"version":1,"summary":"Ready","actions":[{"type":"create_task","project_id":%q,"title":"Created once"}]}`, project.ID), nil
	})

	if err := runAskWithRunner(cmd, []string{"create one"}, runner, true); err != nil {
		t.Fatalf("runAskWithRunner: %v\noutput: %s", err, out.String())
	}
	if fmt.Sprint(attempted) != fmt.Sprint([]aiharness.Name{aiharness.Claude, aiharness.Codex}) {
		t.Fatalf("attempted = %v", attempted)
	}
	saved := deps.Repository.Projects[project.ID]
	if len(saved.Tasks) != initialCount+1 {
		t.Fatalf("task count = %d, want %d", len(saved.Tasks), initialCount+1)
	}
	created := 0
	for _, task := range saved.Tasks {
		if task.Title == "Created once" {
			created++
		}
		if task.Title == "Must not apply" {
			t.Fatal("rejected pre-preview action was applied")
		}
	}
	if created != 1 || !strings.Contains(out.String(), "Applied 1 action(s)") {
		t.Fatalf("created = %d, output = %q", created, out.String())
	}
}
