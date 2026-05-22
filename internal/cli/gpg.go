package cli

import (
	"context"
	"fmt"

	"github-config-manager/internal/audit"
	"github-config-manager/internal/gpg"
	"github-config-manager/internal/profile"
	"github-config-manager/pkg/ui"

	"github.com/spf13/cobra"
)

func newGPGCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gpg",
		Short: "Manage GPG keys",
		RunE: func(_ *cobra.Command, _ []string) error {
			return gpgListRun()
		},
	}

	cmd.AddCommand(newGPGGenerateCmd())
	cmd.AddCommand(newGPGListCmd())
	cmd.AddCommand(newGPGSignCmd())
	cmd.AddCommand(newGPGTestCmd())
	cmd.AddCommand(newGPGUploadCmd())

	return cmd
}

func newGPGGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate <profile>",
		Short: "Generate a new GPG key for a profile",
		Args:  requireArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			profileName := args[0]

			p, err := ctr.ProfileManager.Get(profileName)
			if err != nil {
				ui.Error("profile %q not found", profileName)
				ui.Blank()
				ui.Print("  To see available profiles: gcm profile list")
				return nil
			}

			sp := ui.NewSpinner("Generating GPG key...")
			sp.Start()

			keyInfo, err := ctr.GPGManager.Generate(gpg.GenerateOptions{
				Name:  p.Git.User.Name,
				Email: p.Git.User.Email,
			})
			if err != nil {
				sp.StopError("Failed to generate GPG key")
				ctr.AuditLogger.Log(audit.ActionGPGGenerate, profileName, nil, err)
				return err
			}

			sp.Stop("GPG key generated!")
			ctr.AuditLogger.Log(audit.ActionGPGGenerate, profileName,
				map[string]string{"key_id": keyInfo.KeyID}, nil)

			ui.Blank()
			ui.Detail("Key ID", keyInfo.KeyID)
			ui.Detail("Fingerprint", keyInfo.Fingerprint)
			ui.Detail("Email", keyInfo.Email)

			// Update profile
			p.GPG = &profile.GPGConfig{KeyID: keyInfo.KeyID}
			p.Git.Commit.GPGSign = profile.BoolPtr(true)
			p.Git.User.SigningKey = keyInfo.KeyID
			_ = ctr.ProfileManager.Update(p)

			// Auto-upload GPG key to GitHub if token is available
			if token, err := ctr.GitHubClient.LoadToken(profileName); err == nil && token != "" {
				ctr.GitHubClient.SetToken(token)
				ctx := context.Background()

				// Check if key already exists
				exists, checkErr := ctr.GitHubClient.GPGKeyExists(ctx, keyInfo.KeyID)
				if checkErr == nil && exists {
					ui.Blank()
					ui.Info("GPG key already exists on GitHub — skipping upload.")
				} else {
					ui.Blank()
					upload, askErr := ui.AskConfirm("Upload GPG key to GitHub automatically?", true)
					if askErr == nil && upload {
						sp2 := ui.NewSpinner("Uploading GPG key to GitHub...")
						sp2.Start()

						armoredKey, exportErr := ctr.GPGManager.GetPublicKey(keyInfo.KeyID)
						if exportErr != nil {
							sp2.StopError("Failed to export GPG public key")
							ui.Warning("Could not export key: %v", exportErr)
						} else {
							if uploadErr := ctr.GitHubClient.UploadGPGKey(ctx, armoredKey); uploadErr != nil {
								sp2.StopError("Failed to upload GPG key")
								ui.Warning("Upload failed: %v", uploadErr)
								ui.Print("  You can upload manually at: https://github.com/settings/keys")
							} else {
								sp2.Stop("GPG key uploaded to GitHub!")
								ctr.AuditLogger.Log(audit.ActionGPGGenerate, profileName,
									map[string]string{"key_id": keyInfo.KeyID, "uploaded": "true"}, nil)
							}
						}
					}
				}
			}

			ui.NextSteps([]string{
				fmt.Sprintf("Test signing: gcm gpg test %s", profileName),
			})

			return nil
		},
	}

	return cmd
}

func newGPGListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List GPG keys", Aliases: []string{"ls"},
		RunE: func(_ *cobra.Command, _ []string) error {
			return gpgListRun()
		},
	}
}

func gpgListRun() error {
	if !ctr.GPGManager.IsInstalled() {
		ui.Warning("GPG is not installed on your system")
		ui.Info("Install GPG: https://gnupg.org/download/")
		return nil
	}

	keys, err := ctr.GPGManager.List()
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		ui.Info("No GPG keys found")
		return nil
	}

	headers := []string{"Key ID", "Name", "Email", "Created", "Trust"}
	var rows [][]string
	for _, k := range keys {
		rows = append(rows, []string{
			k.KeyID, k.Name, k.Email,
			k.Created.Format("2006-01-02"), k.Trust,
		})
	}

	ui.SimpleTable(headers, rows)
	return nil
}

func newGPGSignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sign",
		Short: "Manage commit signing",
	}

	cmd.AddCommand(&cobra.Command{
		Use: "enable <profile>", Short: "Enable commit signing", Args: requireArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			p, err := ctr.ProfileManager.Get(args[0])
			if err != nil {
				ui.Error("profile %q not found", args[0])
				ui.Blank()
				ui.Print("  To see available profiles: gcm profile list")
				return nil
			}
			if p.GPG == nil || p.GPG.KeyID == "" {
				return fmt.Errorf("profile %q has no GPG key. Generate one: gcm gpg generate %s", args[0], args[0])
			}
			p.Git.Commit.GPGSign = profile.BoolPtr(true)
			p.Git.User.SigningKey = p.GPG.KeyID
			if err := ctr.ProfileManager.Update(p); err != nil {
				return err
			}
			ui.Success("Commit signing enabled for %q", args[0])
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use: "disable <profile>", Short: "Disable commit signing", Args: requireArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			p, err := ctr.ProfileManager.Get(args[0])
			if err != nil {
				ui.Error("profile %q not found", args[0])
				ui.Blank()
				ui.Print("  To see available profiles: gcm profile list")
				return nil
			}
			p.Git.Commit.GPGSign = profile.BoolPtr(false)
			if err := ctr.ProfileManager.Update(p); err != nil {
				return err
			}
			ui.Success("Commit signing disabled for %q", args[0])
			return nil
		},
	})

	return cmd
}

func newGPGTestCmd() *cobra.Command {
	return &cobra.Command{
		Use: "test <profile>", Short: "Test GPG signing", Args: requireArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			p, err := ctr.ProfileManager.Get(args[0])
			if err != nil {
				ui.Error("profile %q not found", args[0])
				ui.Blank()
				ui.Print("  To see available profiles: gcm profile list")
				return nil
			}
			if p.GPG == nil || p.GPG.KeyID == "" {
				return fmt.Errorf("profile %q has no GPG key configured", args[0])
			}

			sp := ui.NewSpinner("Testing GPG signing...")
			sp.Start()

			if err := ctr.GPGManager.TestSigning(p.GPG.KeyID); err != nil {
				sp.StopError("GPG signing test failed")
				return err
			}

			sp.Stop("GPG signing works!")
			return nil
		},
	}
}

func newGPGUploadCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "upload <profile>",
		Short: "Upload GPG key to GitHub",
		Long: `Upload the profile's GPG public key to GitHub for commit verification.

Checks for duplicates before uploading. Use --force to skip the check.

Examples:
  gcm gpg upload work
  gcm gpg upload work --force`,
		Args: requireArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			profileName := args[0]

			p, err := ctr.ProfileManager.Get(profileName)
			if err != nil {
				ui.Error("profile %q not found", profileName)
				return nil
			}
			if p.GPG == nil || p.GPG.KeyID == "" {
				ui.Error("profile %q has no GPG key configured", profileName)
				ui.Blank()
				ui.Print("  To generate one: gcm gpg generate %s", profileName)
				return nil
			}

			token, err := ctr.GitHubClient.LoadToken(profileName)
			if err != nil || token == "" {
				ui.Error("No GitHub token found for profile %q", profileName)
				ui.Blank()
				ui.Print("  Login first: gcm github login %s", profileName)
				return nil
			}

			ctr.GitHubClient.SetToken(token)
			ctx := context.Background()

			// Check for duplicates
			if !force {
				sp := ui.NewSpinner("Checking if GPG key already exists on GitHub...")
				sp.Start()

				exists, checkErr := ctr.GitHubClient.GPGKeyExists(ctx, p.GPG.KeyID)
				if checkErr != nil {
					sp.StopError("Could not check existing keys")
					ui.Warning("Check failed: %v", checkErr)
					ui.Print("  Use --force to skip the duplicate check")
					return nil
				}
				if exists {
					sp.Stop("GPG key already exists on GitHub")
					ui.Info("This GPG key is already uploaded — no action needed.")
					return nil
				}
				sp.Stop("Key not found on GitHub — uploading")
			}

			armoredKey, exportErr := ctr.GPGManager.GetPublicKey(p.GPG.KeyID)
			if exportErr != nil {
				ui.Error("Could not export GPG public key: %v", exportErr)
				return nil
			}

			sp2 := ui.NewSpinner("Uploading GPG key to GitHub...")
			sp2.Start()

			if uploadErr := ctr.GitHubClient.UploadGPGKey(ctx, armoredKey); uploadErr != nil {
				sp2.StopError("Failed to upload GPG key")
				ui.Warning("Upload failed: %v", uploadErr)
				ui.Print("  You can upload manually at: https://github.com/settings/keys")
				return nil
			}

			sp2.Stop("GPG key uploaded to GitHub!")
			ctr.AuditLogger.Log(audit.ActionGPGGenerate, profileName,
				map[string]string{"action": "upload", "key_id": p.GPG.KeyID, "uploaded": "true"}, nil)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip duplicate check")
	return cmd
}
