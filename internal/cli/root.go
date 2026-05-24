// Package cli implements all Cobra CLI commands for GCM.
package cli

import (
	"git-config-manager/internal/container"
	"git-config-manager/pkg/ui"

	"github.com/spf13/cobra"
)

// ctr is set by SetContainer before commands execute.
var ctr *container.Container

// SetContainer injects the dependency container into CLI commands.
func SetContainer(c *container.Container) {
	ctr = c
}

// NewRootCmd creates the root gcm command.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "gcm",
		Short: "Git Config Manager",
		Long: `Git Config Manager (GCM) - Manage your Git identities with ease.

GCM helps you manage multiple Git identities, SSH keys, GPG keys,
and GitHub/GitLab accounts from a single, intuitive CLI tool.

` + ui.Bold("Getting Started") + `
  gcm setup                     Guided first-time setup (start here!)
  gcm status                    See your current state at a glance

` + ui.Bold("Daily Use") + `
  gcm use <profile>             Switch to a profile
  gcm current                   Show which profile is active
  gcm profile list              See all profiles

` + ui.Bold("Management") + `
  gcm profile create <name> -i  Create a new profile interactively
  gcm ssh generate <profile>    Generate SSH key for a profile
  gcm github login <profile>    Authenticate with GitHub

Run "gcm <command> --help" for details on any command.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// ─── Getting Started ───
	rootCmd.AddCommand(newSetupCmd())
	rootCmd.AddCommand(newStatusCmd())

	// ─── Daily Use ───
	rootCmd.AddCommand(newUseCmd())
	rootCmd.AddCommand(newCurrentCmd())
	rootCmd.AddCommand(newRefreshCmd())

	// ─── Profile Management ───
	rootCmd.AddCommand(newProfileCmd())

	// ─── Key Management ───
	rootCmd.AddCommand(newSSHCmd())
	rootCmd.AddCommand(newGPGCmd())

	// ─── GitHub ───
	rootCmd.AddCommand(newGitHubCmd())
	rootCmd.AddCommand(newGitLabCmd())

	// ─── Shell ───
	rootCmd.AddCommand(newInitCmd())

	// ─── Templates ───
	rootCmd.AddCommand(newTemplateCmd())

	// ─── Backup ───
	rootCmd.AddCommand(newBackupCmd())

	// ─── Diagnostics ───
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newDoctorCmd())

	// ─── Utilities ───
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newCleanCmd())

	// ─── Internal (used by git) ───
	rootCmd.AddCommand(newCredentialHelperCmd())

	return rootCmd
}
