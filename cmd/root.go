package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/tui"
)

var noColor bool
var closeLogging = logging.Close

var (
	configPath string
	dataDir    string
	verbose    bool

	// Build information, populated via ldflags
	Version   = "dev"
	GitCommit = ""
	BuildTime = ""
)

var rootCmd = &cobra.Command{
	Use:   "tt",
	Short: "A terminal-based task and project manager",
	Long: `tt is a CLI-first task and project manager designed for developers.

It supports hierarchical tasks (epics, stories, tasks, bugs, ideas),
markdown-based storage, and integrates with tools like fzf and glow.

Examples:
  tt                           # Launch interactive TUI
  tt dash                      # Show dashboard summary
  tt project new "My Project"  # Create a new project
  tt task add "Fix login bug"  # Add a task to current project
  tt task list --type epic     # List all epics`,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return nil
	},
	RunE: runRoot,
}

// RootCmd returns the root cobra command for use by code generators.
func RootCmd() *cobra.Command {
	return rootCmd
}

// Execute runs the root command and returns any error. The caller is
// responsible for printing the error and terminating the process on failure.
func Execute() error {
	return executeCommand(rootCmd)
}

func executeCommand(command *cobra.Command) error {
	defer closeLogging()
	// SilenceUsage is enabled in PersistentPreRunE so usage is still printed
	// when cobra itself rejects args (wrong count, unknown flags, etc.) but
	// suppressed for errors that originate inside RunE.
	command.SilenceErrors = true
	return command.Execute()
}

func init() {
	cobra.OnInitialize(initConfig, initNoColor)
	logging.Init()
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", fmt.Sprintf("workspace data directory (env %s; default: %s)", config.DataDirEnvVar, defaultDataDirDisplay))
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "deprecated: isolate a workspace at this config file's directory")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable color output")
	rootCmd.Flags().Bool("version", false, "show version information")

	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = viper.BindPFlag("data-dir", rootCmd.PersistentFlags().Lookup("data-dir"))
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

func initConfig() {
	if configPath != "" {
		viper.Set("config", configPath)
	}
	if dataDir != "" {
		viper.Set("data-dir", dataDir)
	}
}

func initNoColor() {
	if noColor || os.Getenv("NO_COLOR") != "" {
		if err := os.Setenv("NO_COLOR", "1"); err != nil {
			fmt.Fprintf(os.Stderr, "failed to set NO_COLOR: %v\n", err)
		}
	}
}

func GetConfigPath() string {
	return viper.GetString("config")
}

// GetDataDir returns the explicitly selected workspace data directory: the
// --data-dir flag, or the TT_DATA_DIR environment variable when the flag is
// unset. It is empty when neither is provided, so callers can fall back to the
// active-workspace pointer or the legacy default.
func GetDataDir() string {
	if dd := viper.GetString("data-dir"); dd != "" {
		return dd
	}
	return os.Getenv(config.DataDirEnvVar)
}

// resolveConfigPath derives the config path the manager should use, honoring the
// documented workspace precedence: an explicit --config wins (its directory is
// the isolated workspace root); otherwise an explicit --data-dir/TT_DATA_DIR or
// the machine-local active-workspace pointer selects the data dir and its config
// path; otherwise an empty string lets NewManager resolve the legacy default
// (iCloud when available, else ~/.tt).
func resolveConfigPath() string {
	if cp := GetConfigPath(); cp != "" {
		return cp
	}
	dd := GetDataDir()
	if dd == "" {
		if ptr, ok := config.ReadActiveWorkspacePointer(); ok {
			dd = ptr
		}
	}
	if dd != "" {
		return config.ConfigPathForDataDir(filepath.Clean(dd))
	}
	return ""
}

// IsVerbose returns true if verbose mode is enabled via --verbose or -v flag.
func IsVerbose() bool {
	return viper.GetBool("verbose")
}

func runRoot(cmd *cobra.Command, args []string) error {
	showVersion, _ := cmd.Flags().GetBool("version")
	if showVersion {
		printVersion()
		return nil
	}

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}

	tui.AppVersion = Version
	return tui.RunApp(workspace.Runtime, workspace.Manager, tui.WithCLIReference(CLIReference))
}

func printVersion() {
	if GitCommit == "" && BuildTime == "" {
		fmt.Printf("tt version %s\n", Version)
		return
	}
	fmt.Printf("tt version %s (commit: %s, built: %s)\n", Version, GitCommit, BuildTime)
}
