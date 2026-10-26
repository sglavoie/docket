package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tterrors"
)

var (
	projectExportFile string
	projectExportAll  bool
	projectExportDir  string
)

// projectExportStdout is the --file value that sends the export to stdout.
const projectExportStdout = "-"

var projectExportCmd = &cobra.Command{
	Use:   "export [slug-or-id]",
	Short: "Export a project to clean Markdown",
	Long: `Export a project to a human-readable Markdown file in the exports/ folder
of your Docket data directory.

Name the project by slug or ID, or pass --all to export every project,
archived ones included, as one <slug>.md file each.
Use --file to write one project to an arbitrary path, or --file - to print it.
Use --dir with --all to write somewhere other than exports/.`,
	Example: `  docket project export my-project
  docket project export my-project --file /tmp/my-project.md
  docket project export my-project --file -
  docket project export --all
  docket project export --all --dir /tmp/docket-export`,
	Args:              atMostOneArg("project slug or ID"),
	RunE:              runProjectExport,
	ValidArgsFunction: completeProjectSlugs,
}

func init() {
	projectCmd.AddCommand(projectExportCmd)
	projectExportCmd.Flags().StringVar(&projectExportFile, "file", "", "write to this path instead of the default exports/<slug>.md (- for stdout)")
	projectExportCmd.Flags().BoolVar(&projectExportAll, "all", false, "export every project, archived included, one <slug>.md each")
	projectExportCmd.Flags().StringVar(&projectExportDir, "dir", "", "directory for --all (default: the exports/ folder)")
	_ = projectExportCmd.MarkFlagDirname("dir")
}

func runProjectExport(cmd *cobra.Command, args []string) error {
	if err := validateProjectExportFlags(args); err != nil {
		return usageRefusal(cmd, err)
	}
	if err := requireUpgradedWorkspace(); err != nil {
		return err
	}
	if projectExportAll {
		return runProjectExportAll(cmd)
	}

	project, workspace, err := LoadRuntimeProject(args[0])
	if err != nil {
		if errors.Is(err, tterrors.ErrProjectNotFound) {
			return fmt.Errorf("%w; name a project by its slug or ID, or pass --all to export every project", err)
		}
		return err
	}
	related := projectExportRelated(cmd, workspace, project)

	if projectExportFile == projectExportStdout {
		return storage.ExportProjectMarkdownTo(commandOut(cmd), project, related...)
	}

	dest := projectExportFile
	if dest == "" {
		dest = workspace.Manager.ExportPath(project.Slug)
		if err = os.MkdirAll(workspace.Manager.ExportsDir(), 0700); err != nil {
			return fmt.Errorf("create exports dir: %w", err)
		}
	}
	destPath, err := exportProjectTo(workspace, project, dest, related)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(commandOut(cmd), "Exported %q to %s\n", project.Name, config.DisplayPath(destPath))
	return nil
}

// validateProjectExportFlags refuses the argument and flag combinations that
// name two different destinations or none at all.
func validateProjectExportFlags(args []string) error {
	if projectExportAll {
		if len(args) > 0 {
			return errors.New("--all exports every project; do not also name one")
		}
		if projectExportFile != "" {
			return errors.New("--all writes one file per project; use --dir, not --file")
		}
		return nil
	}
	if projectExportDir != "" {
		return errors.New("--dir only applies with --all; use --file for one project")
	}
	if len(args) == 0 {
		return errors.New("project slug or ID required: name a project, or pass --all")
	}
	return nil
}

// runProjectExportAll writes every cataloged project, archived included, as
// <slug>.md into --dir, which defaults to the exports folder.
func runProjectExportAll(cmd *cobra.Command) error {
	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}
	dir := projectExportDir
	if dir == "" {
		dir = workspace.Manager.ExportsDir()
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create export dir: %w", err)
	}

	related := projectExportRelated(cmd, workspace, nil)
	out := commandOut(cmd)
	for _, project := range workspace.Runtime.AllProjects() {
		dest := filepath.Join(dir, filepath.Base(project.Slug)+".md")
		destPath, exportErr := exportProjectTo(workspace, project, dest, related)
		if exportErr != nil {
			return fmt.Errorf("export %q: %w", project.Slug, exportErr)
		}
		_, _ = fmt.Fprintf(out, "Exported %q to %s\n", project.Name, config.DisplayPath(destPath))
	}
	return nil
}

// exportProjectTo guards one file destination against the immutable migration
// snapshots and then writes the export there.
func exportProjectTo(workspace *CLIWorkspace, project *models.Project, dest string, related []*models.Project) (string, error) {
	if guardErr := config.RefusePreSimplificationV5SnapshotMutation(workspace.Runtime.DataDir(), dest); guardErr != nil {
		return "", fmt.Errorf("refusing project export destination: %w", guardErr)
	}
	return ExportProjectMarkdown(project, dest, related...)
}

// projectExportRelated is the label population for outgoing references,
// loaded once for however many projects the command exports. Archived projects
// join it deliberately. An export is a document rather than a live view, so
// naming a reference that points into archived work is strictly better than
// emitting its bare locator; nothing here loads an archived project for any
// other purpose. A project that cannot be read only costs its labels, so the
// export goes on, but it says so on stderr rather than degrading silently.
// exported, when set, is the one project being exported: it is loaded by the
// export itself, which reports its own failure.
func projectExportRelated(cmd *cobra.Command, workspace *CLIWorkspace, exported *models.Project) []*models.Project {
	var candidates []*models.Project
	for _, project := range append(workspace.Runtime.AvailableProjects(), workspace.Runtime.ArchivedProjects()...) {
		if exported == nil || project.ID != exported.ID {
			candidates = append(candidates, project)
		}
	}
	loaded, failures := storage.LoadExportLabelProjects(candidates)
	for _, failure := range failures {
		_, _ = fmt.Fprintf(commandErr(cmd), "warning: could not read project %q (%v); references into it are exported as their stored locators\n",
			failure.Project.Slug, failure.Err)
	}
	return loaded
}

// commandErr is the command's error writer, or stderr when run without one.
func commandErr(cmd *cobra.Command) io.Writer {
	if cmd == nil {
		return os.Stderr
	}
	return cmd.ErrOrStderr()
}

// commandOut is the command's output writer, or stdout when run without one.
func commandOut(cmd *cobra.Command) io.Writer {
	if cmd == nil {
		return os.Stdout
	}
	return cmd.OutOrStdout()
}

// ExportProjectMarkdown writes the project to destPath (which must be non-empty;
// callers resolve default export locations through config.Manager.ExportPath)
// and returns the final absolute path.
// LoadProject is called on p to ensure tasks are populated before exporting.
// related supplies display labels for outgoing task references that leave p.
func ExportProjectMarkdown(p *models.Project, destPath string, related ...*models.Project) (string, error) {
	return storage.ExportProjectMarkdown(p, destPath, related...)
}
