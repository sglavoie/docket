package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/actionplan"
	"github.com/sglavoie/tt/internal/aiharness"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var (
	askApply     bool
	askNoContext bool
	askRedact    bool
	askHarness   string
	askConfigDir string
)

var askCmd = &cobra.Command{
	Use:   "ask [query]",
	Short: "Ask the AI assistant about your tasks",
	Long: `Ask the AI assistant a question about your project and tasks.

With explicit remembered consent, the assistant can receive context about every
active project. For a consented full-context request, Codex can inspect the
active workspace directory read-only. Responses are strict structured action
plans; --apply previews and confirms those actions before one transactional,
undoable commit.
Claude, Codex, and GitHub Copilot are supported. The TUI selection is the CLI
default; --harness and --config-dir override one request without changing it.

Examples:
  tt ask "what are my high priority tasks?"
  tt ask --harness codex "turn this plan into tasks"
  tt ask --harness copilot --config-dir ~/.copilot "what should I do next?"
  tt ask "create a bug for the login timeout issue" --apply
  tt ask --no-context "how do I use tt to manage dependencies?"`,
	RunE: runAsk,
}

func init() {
	rootCmd.AddCommand(askCmd)

	askCmd.Flags().BoolVar(&askApply, "apply", false, "preview and apply accepted structured actions as one transaction")
	askCmd.Flags().StringVar(&askHarness, "harness", "", "AI CLI harness: claude, codex, or copilot (default: remembered selection)")
	askCmd.Flags().StringVar(&askConfigDir, "config-dir", "", "one-request configuration directory override for the selected harness")
	askCmd.Flags().BoolVar(&askNoContext, "no-context", false, "skip project context in the prompt")
	askCmd.Flags().BoolVar(&askRedact, "redact-context", false, "send task IDs and metadata but redact names and text")
	_ = askCmd.RegisterFlagCompletionFunc("harness", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return aiharness.AllowedValues(), cobra.ShellCompDirectiveNoFileComp
	})
}

func runAsk(cmd *cobra.Command, args []string) error {
	return runAskWithRunner(cmd, args, external.DefaultRunner, askInputIsInteractive(cmd.InOrStdin()))
}

type askRunner interface {
	Run(context.Context, external.Request) (string, error)
}

// runAskWithRunner keeps the AI process boundary injectable without requiring
// tests to replace package-global executables or the shared external runner.
func runAskWithRunner(cmd *cobra.Command, args []string, runner askRunner, interactive bool) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("question")
	}
	if askNoContext && askRedact {
		return fmt.Errorf("--no-context and --redact-context cannot be used together")
	}
	if runner == nil {
		return fmt.Errorf("AI harness runner is unavailable")
	}

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}
	harness, err := resolveAskHarness(workspace.Runtime.Machine(), askHarness)
	if err != nil {
		return err
	}
	initialHarness := harness
	cliConfigDirSet := askConfigDir != "" || cmd.Flags().Changed("config-dir")
	normalizedCLIConfigDir := askConfigDir
	if cliConfigDirSet {
		normalizedCLIConfigDir, err = aiharness.NormalizeExplicitConfigDir(askConfigDir)
		if err != nil {
			return fmt.Errorf("invalid --config-dir: %w", err)
		}
	}

	reader := bufio.NewReader(cmd.InOrStdin())
	out := cmd.OutOrStdout()

	query := strings.Join(args, " ")
	contextMode := external.TaskContextFull
	if askRedact {
		contextMode = external.TaskContextRedacted
	}
	var cliContext string
	if CLIReference != "" {
		cliContext = external.BuildCLIContext(CLIReference, query)
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	service := newWorkspaceAppService(workspace)
	prepared, err := requestAskActionPlan(askPlanRequest{
		cmd:          cmd,
		workspace:    workspace,
		runner:       runner,
		reader:       reader,
		out:          out,
		service:      service,
		ctx:          ctx,
		query:        query,
		cliContext:   cliContext,
		contextMode:  contextMode,
		initial:      initialHarness,
		cliConfigDir: normalizedCLIConfigDir,
		useCLIConfig: cliConfigDirSet,
		noContext:    askNoContext,
		interactive:  interactive,
	})
	if err != nil {
		return err
	}

	preview := prepared.Preview()
	printActionPlanPreview(out, preview)
	if !askApply || len(preview.Actions) == 0 {
		return nil
	}

	confirmSafe, confirmDestructive, err := confirmCLIActionPlan(reader, out, preview)
	if err != nil {
		return err
	}

	accepted := selectCLIActionPlan(prepared, confirmSafe, confirmDestructive)
	if len(accepted.Preview().Actions) == 0 {
		if _, writeErr := fmt.Fprintln(out, "No actions applied."); writeErr != nil {
			return fmt.Errorf("write action-plan result: %w", writeErr)
		}
		return nil
	}

	result, warning, err := applyCLIActionPlan(ctx, workspace.Runtime, service, accepted, app.ApplyActionPlanOptions{
		ConfirmSafe:        true,
		ConfirmDestructive: confirmDestructive,
	})
	if err != nil {
		if isStaleActionPlanError(err) {
			return fmt.Errorf("the workspace changed while this action plan was awaiting confirmation; request a fresh AI response and review its new preview: %w", err)
		}
		return translateAppError(err)
	}
	if warning != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: action plan committed, but local reference repair was incomplete: %v\n", warning)
	}
	if _, writeErr := fmt.Fprintf(out, "\nApplied %d action(s). Transaction ID: %s; Undo ID: %s\n", len(result.Actions), result.TransactionID, result.UndoID); writeErr != nil {
		return fmt.Errorf("write action-plan result: %w", writeErr)
	}

	return nil
}

type askPlanRequest struct {
	cmd          *cobra.Command
	workspace    *CLIWorkspace
	runner       askRunner
	reader       *bufio.Reader
	out          io.Writer
	service      *app.Service
	ctx          context.Context
	query        string
	cliContext   string
	contextMode  external.TaskContextMode
	initial      aiharness.Name
	cliConfigDir string
	useCLIConfig bool
	noContext    bool
	interactive  bool
}

func requestAskActionPlan(request askPlanRequest) (*app.PreparedActionPlan, error) {
	harness := request.initial
	tried := make(map[aiharness.Name]bool, len(aiharness.Names()))
	contextDeclined := false
	taskContextLoaded := false
	var taskContext string

	for {
		configDir, err := resolveAskConfigDir(
			request.workspace.Runtime.Machine(),
			harness,
			request.cliConfigDir,
			request.useCLIConfig && harness == request.initial,
		)
		if err != nil && len(tried) == 0 {
			return nil, err
		}

		tried[harness] = true
		var attemptErr error
		if err != nil {
			attemptErr = err
		} else {
			requestTaskContext := ""
			workspaceDir := ""
			if !request.noContext && !contextDeclined {
				consented, consentErr := ensureAIContextConsent(request.cmd, request.workspace, request.reader, harness)
				if consentErr != nil {
					return nil, consentErr
				}
				if !consented {
					contextDeclined = true
				} else {
					if !taskContextLoaded {
						projects, loadErr := loadRuntimeActiveProjects(request.workspace)
						if loadErr != nil {
							safeLoadErr := actionplan.TerminalSafeDisplayText(loadErr.Error(), actionplan.DisplayTextSingleLine)
							return nil, fmt.Errorf("failed to load all active projects for AI context: %s; no AI request was sent; rerun with --no-context to continue without workspace context", safeLoadErr)
						}
						taskContext = external.BuildWorkspaceTaskContextWithMode(projects, request.contextMode)
						taskContextLoaded = true
					}
					requestTaskContext = taskContext
					if request.contextMode == external.TaskContextFull && request.workspace != nil && request.workspace.Runtime != nil {
						workspaceDir = request.workspace.Runtime.DataDir()
					}
				}
			}

			systemPrompt := external.BuildActionPlanPrompt(requestTaskContext, request.cliContext)
			if harness == aiharness.Codex && workspaceDir != "" {
				systemPrompt = external.BuildCodexWorkspaceActionPlanPrompt(requestTaskContext, request.cliContext)
			}
			response, runErr := request.runner.Run(request.ctx, external.Request{
				Harness:      harness,
				ConfigDir:    configDir,
				WorkspaceDir: workspaceDir,
				Query:        request.query,
				SystemPrompt: systemPrompt,
			})
			if runErr != nil {
				attemptErr = fmt.Errorf("%s request failed: %w", harness.Label(), runErr)
				if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
					return nil, attemptErr
				}
			} else {
				plan, parseErr := external.ParseActionPlan(response)
				if parseErr != nil {
					attemptErr = fmt.Errorf("%s returned an invalid action plan: %w", harness.Label(), parseErr)
				} else {
					prepared, prepareErr := request.service.PrepareActionPlan(plan)
					if prepareErr == nil {
						return prepared, nil
					}
					attemptErr = fmt.Errorf("%s action plan was rejected during preparation: %w", harness.Label(), translateAppError(prepareErr))
				}
			}
		}

		remaining := remainingAskHarnesses(tried)
		if len(remaining) == 0 {
			return nil, fmt.Errorf("%w; all supported AI harnesses were tried once", attemptErr)
		}
		if !request.interactive {
			return nil, fmt.Errorf("%w; stdin is non-interactive, so retry is disabled; rerun with %s", attemptErr, askHarnessFlagGuidance(remaining))
		}
		next, retry, promptErr := promptAskRetry(request.reader, request.out, attemptErr, remaining)
		if promptErr != nil {
			return nil, promptErr
		}
		if !retry {
			return nil, attemptErr
		}
		harness = next
	}
}

func askInputIsInteractive(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(file.Fd()) || isatty.IsCygwinTerminal(file.Fd())
}

func resolveAskHarness(machine *config.MachineState, override string) (aiharness.Name, error) {
	if override != "" {
		name, err := aiharness.Parse(override)
		if err != nil {
			return "", fmt.Errorf("invalid --harness: %w", err)
		}
		return name, nil
	}
	return machine.GetAIHarness(), nil
}

// resolveAskConfigDir returns only an explicit directory override for the
// process request. Environment and native-default sources intentionally remain
// empty so the selected provider retains its inherited behavior. A one-request
// CLI override belongs to the initial provider; interactive retries use each
// alternative provider's remembered override instead of reusing another
// provider's configuration directory.
func resolveAskConfigDir(machine *config.MachineState, harness aiharness.Name, cliOverride string, useCLIOverride bool) (string, error) {
	override := machine.GetAIConfigDir(harness)
	if useCLIOverride {
		override = cliOverride
	}
	resolved, err := aiharness.ResolveConfigDir(harness, override)
	if err != nil {
		return "", fmt.Errorf("resolve %s configuration directory: %w", harness.Label(), err)
	}
	if resolved.Source != aiharness.ConfigDirMachineOverride {
		return "", nil
	}
	return resolved.Path, nil
}

func remainingAskHarnesses(tried map[aiharness.Name]bool) []aiharness.Name {
	remaining := make([]aiharness.Name, 0, len(aiharness.Names()))
	for _, name := range aiharness.Names() {
		if !tried[name] {
			remaining = append(remaining, name)
		}
	}
	return remaining
}

func askHarnessFlagGuidance(harnesses []aiharness.Name) string {
	flags := make([]string, len(harnesses))
	for i, harness := range harnesses {
		flags[i] = "--harness " + harness.String()
	}
	return strings.Join(flags, " or ")
}

func promptAskRetry(reader *bufio.Reader, out io.Writer, failure error, remaining []aiharness.Name) (aiharness.Name, bool, error) {
	if _, err := fmt.Fprintf(out, "\nAI request failed before a valid preview: %s\n", actionplan.TerminalSafeDisplayText(failure.Error(), actionplan.DisplayTextSingleLine)); err != nil {
		return "", false, fmt.Errorf("write AI retry failure: %w", err)
	}
	choices := make([]string, len(remaining))
	allowed := make(map[aiharness.Name]bool, len(remaining))
	for i, harness := range remaining {
		choices[i] = harness.String()
		allowed[harness] = true
	}
	for {
		if _, err := fmt.Fprintf(out, "Retry with another harness? [%s/N] ", strings.Join(choices, "/")); err != nil {
			return "", false, fmt.Errorf("write AI retry prompt: %w", err)
		}
		answer, err := reader.ReadString('\n')
		if err != nil && answer == "" {
			return "", false, fmt.Errorf("%w; failed to read retry choice: %w", failure, err)
		}
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "" || answer == "n" || answer == "no" {
			return "", false, nil
		}
		selected, parseErr := aiharness.Parse(answer)
		if parseErr == nil && allowed[selected] {
			return selected, true, nil
		}
		if _, writeErr := fmt.Fprintf(out, "Choose %s, or N to stop.\n", strings.Join(choices, ", ")); writeErr != nil {
			return "", false, fmt.Errorf("write AI retry validation: %w", writeErr)
		}
	}
}

func selectCLIActionPlan(prepared *app.PreparedActionPlan, includeSafe, includeDestructive bool) *app.PreparedActionPlan {
	var risks []app.ActionRisk
	if includeSafe {
		risks = append(risks, app.ActionRiskSafe)
	}
	if includeDestructive {
		risks = append(risks, app.ActionRiskDestructive)
	}
	return prepared.SelectRisks(risks...)
}

func isStaleActionPlanError(err error) bool {
	return errors.Is(err, storage.ErrStaleRevision) || errors.Is(err, storage.ErrStaleArtifact)
}

func confirmCLIActionPlan(reader *bufio.Reader, out io.Writer, preview app.PlanPreview) (bool, bool, error) {
	confirmSafe := preview.SafeCount == 0
	if preview.SafeCount > 0 {
		if _, err := fmt.Fprintf(out, "\nApply %d safe action(s) as one transaction? [y/N] ", preview.SafeCount); err != nil {
			return false, false, fmt.Errorf("write safe-action confirmation: %w", err)
		}
		answer, err := reader.ReadString('\n')
		if err != nil {
			return false, false, fmt.Errorf("failed to read confirmation: %w", err)
		}
		answer = strings.TrimSpace(strings.ToLower(answer))
		confirmSafe = answer == "y" || answer == "yes"
	}

	confirmDestructive := false
	if preview.DestructiveCount > 0 {
		if _, err := fmt.Fprintf(out, "\n%d destructive action(s) require separate approval. Type DELETE to include them: ", preview.DestructiveCount); err != nil {
			return false, false, fmt.Errorf("write destructive-action confirmation: %w", err)
		}
		answer, err := reader.ReadString('\n')
		if err != nil {
			return false, false, fmt.Errorf("failed to read destructive confirmation: %w", err)
		}
		confirmDestructive = strings.TrimSpace(answer) == "DELETE"
	}
	return confirmSafe, confirmDestructive, nil
}

// applyCLIActionPlan joins AI task deletion to the same planning and machine
// reference contract as the ordinary delete command. The prepared plan retains
// the exact reviewed deletion subtree, then invokes the artifact callback before
// committing so Focus Queue, Daily Plan, and dismissals ride the task transaction
// and workspace undo ledger. Machine-local visits are repaired only after that
// durable commit succeeds; their failure is returned separately as a warning so
// callers never retry an already-applied plan.
func applyCLIActionPlan(
	ctx context.Context,
	runtime *config.RuntimeState,
	service *app.Service,
	prepared *app.PreparedActionPlan,
	opts app.ApplyActionPlanOptions,
) (app.ActionPlanResult, error, error) {
	if service == nil {
		return app.ActionPlanResult{}, nil, fmt.Errorf("application service is unavailable")
	}

	var pendingDomains *config.Domains
	priorBuilder := opts.DeletedTaskArtifacts
	opts.DeletedTaskArtifacts = func(deletedTaskIDs []string) ([]storage.ArtifactChange, error) {
		var artifacts []storage.ArtifactChange
		if priorBuilder != nil {
			priorArtifacts, err := priorBuilder(deletedTaskIDs)
			if err != nil {
				return nil, err
			}
			artifacts = append(artifacts, priorArtifacts...)
		}
		if runtime == nil {
			return nil, fmt.Errorf("runtime state is unavailable")
		}
		domains, err := runtime.CloneDomains()
		if err != nil {
			return nil, err
		}
		pendingDomains = domains
		if domains.Planning.RemoveDurableTaskRefs(actionPlanTaskIDSet(deletedTaskIDs)) == 0 || runtime.DataDir() == "" {
			return artifacts, nil
		}
		artifact, err := runtime.CheckedPlanningArtifact(domains.Planning)
		if err != nil {
			return nil, err
		}
		return append(artifacts, artifact.StorageChange()), nil
	}

	result, err := service.ApplyPreparedActionPlan(ctx, prepared, opts)
	if err != nil {
		return app.ActionPlanResult{}, nil, err
	}
	warning := repairCLIActionPlanTaskRefs(runtime, result.DeletedTaskIDs, pendingDomains)
	return result, warning, nil
}

func repairCLIActionPlanTaskRefs(runtime *config.RuntimeState, deletedTaskIDs []string, pendingDomains *config.Domains) error {
	if len(deletedTaskIDs) == 0 {
		return nil
	}
	if runtime == nil {
		return fmt.Errorf("runtime state is unavailable")
	}
	deleted := actionPlanTaskIDSet(deletedTaskIDs)
	if runtime.DataDir() == "" {
		if pendingDomains == nil {
			var err error
			pendingDomains, err = runtime.CloneDomains()
			if err != nil {
				return err
			}
		}
		pendingDomains.Machine.RemoveMachineLocalTaskRefs(deleted)
		runtime.SetDomainsInMemory(pendingDomains)
		return nil
	}

	var warnings []error
	if err := runtime.RefreshAfterCommit(); err != nil {
		warnings = append(warnings, fmt.Errorf("refresh planning state: %w", err))
	}
	domains, err := runtime.CloneDomains()
	if err != nil {
		warnings = append(warnings, err)
		return errors.Join(warnings...)
	}
	if domains.Machine.RemoveMachineLocalTaskRefs(deleted) > 0 {
		if err := runtime.CommitMachine(domains.Machine); err != nil {
			warnings = append(warnings, fmt.Errorf("update recent visits: %w", err))
		}
	}
	return errors.Join(warnings...)
}

func actionPlanTaskIDSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func ensureAIContextConsent(cmd *cobra.Command, workspace *CLIWorkspace, reader *bufio.Reader, harness aiharness.Name) (bool, error) {
	if workspace.Runtime.Settings().AIConsent.GrantedFor(harness) {
		return true, nil
	}
	out := cmd.OutOrStdout()
	if _, writeErr := fmt.Fprintf(out, "AI context disclosure: tt can send names and task content from all active projects to the configured %s CLI provider. For a consented full-context request, Codex can inspect the active workspace directory read-only. It cannot modify tasks directly; every answer and action still uses the strict JSON action-plan review and confirmation flow.\n", harness.Label()); writeErr != nil {
		return false, fmt.Errorf("write AI context disclosure: %w", writeErr)
	}
	if _, writeErr := fmt.Fprint(out, "Remember consent and include task context? [y/N] "); writeErr != nil {
		return false, fmt.Errorf("write AI context confirmation: %w", writeErr)
	}
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read AI context consent: %w", err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		if _, writeErr := fmt.Fprintln(out, "Continuing without task context."); writeErr != nil {
			return false, fmt.Errorf("write AI context result: %w", writeErr)
		}
		return false, nil
	}
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return false, err
	}
	now := time.Now().UTC()
	if err := domains.Settings.AIConsent.GrantFor(harness, now); err != nil {
		return false, err
	}
	if err := commitCLISettings(workspace, domains.Settings); err != nil {
		return false, err
	}
	return true, nil
}

func printActionPlanPreview(out interface{ Write([]byte) (int, error) }, preview app.PlanPreview) {
	if preview.Summary != "" {
		_, _ = fmt.Fprintf(out, "%s\n", actionplan.TerminalSafeDisplayText(preview.Summary, actionplan.DisplayTextSingleLine))
	}
	if len(preview.Actions) == 0 {
		_, _ = fmt.Fprintln(out, "No actions proposed.")
		return
	}
	_, _ = fmt.Fprintln(out, "\nProposed actions:")
	for _, action := range preview.Actions {
		_, _ = fmt.Fprintf(out, "  %d. [%s] %s\n", action.Index, action.Risk, actionplan.TerminalSafeDisplayText(action.Description, actionplan.DisplayTextMultiline))
	}
}
