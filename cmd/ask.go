package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/actionplan"
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
)

var askCmd = &cobra.Command{
	Use:   "ask [query]",
	Short: "Ask the AI assistant about your tasks",
	Long: `Ask the AI assistant a question about your project and tasks.

With explicit remembered consent, the assistant can receive context about the
current project. Responses are strict structured action plans; --apply previews
and confirms those actions before one transactional, undoable commit.

Examples:
  tt ask "what are my high priority tasks?"
  tt ask "create a bug for the login timeout issue" --apply
  tt ask --no-context "how do I use tt to manage dependencies?"`,
	RunE: runAsk,
}

func init() {
	rootCmd.AddCommand(askCmd)

	askCmd.Flags().BoolVar(&askApply, "apply", false, "preview and apply accepted structured actions as one transaction")
	askCmd.Flags().BoolVar(&askNoContext, "no-context", false, "skip project context in the prompt")
	askCmd.Flags().BoolVar(&askRedact, "redact-context", false, "send task IDs and metadata but redact names and text")
}

func runAsk(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("question")
	}

	if !external.ClaudeAvailable() {
		return fmt.Errorf("claude CLI not found. Install it from https://claude.ai/cli")
	}
	if askNoContext && askRedact {
		return fmt.Errorf("--no-context and --redact-context cannot be used together")
	}

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}
	reader := bufio.NewReader(cmd.InOrStdin())
	out := cmd.OutOrStdout()

	query := strings.Join(args, " ")
	contextMode := external.TaskContextFull
	if askRedact {
		contextMode = external.TaskContextRedacted
	}
	var taskContext string

	if !askNoContext {
		consented, consentErr := ensureAIContextConsent(cmd, workspace, reader)
		if consentErr != nil {
			return consentErr
		}
		if consented {
			project, loadErr := loadRuntimeCurrentProject(workspace)
			if loadErr == nil {
				taskContext = external.BuildTaskContextWithMode(project, contextMode)
			}
		}
	}

	var cliContext string
	if CLIReference != "" {
		cliContext = external.BuildCLIContext(CLIReference, query)
	}
	systemPrompt := external.BuildActionPlanPrompt(taskContext, cliContext)

	ctx := context.Background()
	response, err := external.ClaudeAsk(ctx, query, systemPrompt)
	if err != nil {
		return fmt.Errorf("failed to get response: %w", err)
	}

	plan, err := external.ParseActionPlan(response)
	if err != nil {
		return err
	}
	service := newWorkspaceAppService(workspace)
	prepared, err := service.PrepareActionPlan(plan)
	if err != nil {
		return translateAppError(err)
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

func ensureAIContextConsent(cmd *cobra.Command, workspace *CLIWorkspace, reader *bufio.Reader) (bool, error) {
	if workspace.Runtime.Settings().AIConsent.Granted {
		return true, nil
	}
	out := cmd.OutOrStdout()
	if _, writeErr := fmt.Fprintln(out, "AI context disclosure: tt can send the current project name and task content to the configured Claude CLI provider."); writeErr != nil {
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
	domains.Settings.AIConsent.Granted = true
	domains.Settings.AIConsent.GrantedAt = &now
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
