package cli

import (
	"context"
	"fmt"

	"git-config-manager/internal/audit"
	"git-config-manager/internal/profile"
	providerpkg "git-config-manager/internal/provider"
	"git-config-manager/internal/ssh"
	"git-config-manager/pkg/ui"

	"github.com/spf13/cobra"
)

func newSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Manage SSH keys",
		RunE: func(_ *cobra.Command, _ []string) error {
			return sshListRun()
		},
	}

	cmd.AddCommand(newSSHGenerateCmd())
	cmd.AddCommand(newSSHListCmd())
	cmd.AddCommand(newSSHTestCmd())
	cmd.AddCommand(newSSHCopyCmd())
	cmd.AddCommand(newSSHUploadCmd())

	return cmd
}

func newSSHGenerateCmd() *cobra.Command {
	var (
		keyType    string
		bits       int
		comment    string
		passphrase string
	)

	cmd := &cobra.Command{
		Use:   "generate <profile>",
		Short: "Generate a new SSH key for a profile",
		Long: `Generate a new SSH key pair and associate it with a profile.

Examples:
  gcm ssh generate work --type ed25519
  gcm ssh generate work --type rsa --bits 4096`,
		Args: requireArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]

			p, err := ctr.ProfileManager.Get(profileName)
			if err != nil {
				ui.Error("profile %q not found", profileName)
				ui.Blank()
				ui.Print("  To see available profiles: gcm profile list")
				ui.Print("  To create a new profile:   gcm profile create " + profileName + " -i")
				return nil
			}
			keyProfileName := sshKeyProfileName(profileName, p)

			if cmd.Flags().Changed("passphrase") && passphrase != "" {
				ui.Warning("Passphrase provided via --passphrase flag may appear in shell history.")
				ui.Print("  For secure passphrase entry, omit the flag and you will be prompted interactively.")
			}

			sp := ui.NewSpinner("Generating SSH key...")
			sp.Start()

			keyInfo, err := ctr.SSHManager.Generate(ssh.GenerateOptions{
				Profile:    keyProfileName,
				KeyType:    keyType,
				Bits:       bits,
				Comment:    comment,
				Passphrase: passphrase,
			})
			if err != nil {
				sp.StopError("Failed to generate SSH key")
				ctr.AuditLogger.Log(audit.ActionSSHGenerate, profileName,
					map[string]string{"type": keyType}, err)
				return err
			}

			sp.Stop("SSH key generated!")
			ctr.AuditLogger.Log(audit.ActionSSHGenerate, profileName,
				map[string]string{"type": keyInfo.Type, "path": keyInfo.Path}, nil)

			ui.Blank()
			ui.Detail("Path", keyInfo.Path)
			ui.Detail("Type", keyInfo.Type)
			ui.Detail("Fingerprint", keyInfo.Fingerprint)
			ui.Blank()
			ui.Print("Public key:")
			ui.Print(keyInfo.PublicKey)

			// Update profile if it exists
			if p != nil {
				p.SSH = &profile.SSHConfig{
					KeyPath:     keyInfo.Path,
					KeyType:     profile.KeyType(keyInfo.Type),
					Fingerprint: keyInfo.Fingerprint,
					Comment:     keyInfo.Comment,
				}
				_ = ctr.ProfileManager.Update(p)
			}

			for _, def := range authenticatedProvidersForProfile(profileName, p, providerpkg.CapabilitySSHKeys) {
				if setupSSHKeyUploadForProvider(cmd.Context(), profileName, p, def, keyInfo.PublicKey, keyInfo.Type) {
					ctr.AuditLogger.Log(audit.ActionSSHGenerate, profileName,
						map[string]string{"type": keyInfo.Type, "path": keyInfo.Path, "uploaded": "true", "provider": string(def.ID)}, nil)
				}
			}

			ui.NextSteps([]string{
				fmt.Sprintf("Test connection: gcm ssh test %s", profileName),
			})

			return nil
		},
	}

	cmd.Flags().StringVarP(&keyType, "type", "t", "ed25519", "Key type (ed25519, rsa, ecdsa)")
	cmd.Flags().IntVarP(&bits, "bits", "b", 4096, "Key bits (RSA only)")
	cmd.Flags().StringVarP(&comment, "comment", "c", "", "Key comment")
	cmd.Flags().StringVarP(&passphrase, "passphrase", "p", "", "Key passphrase")

	return cmd
}

func newSSHListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List SSH keys", Aliases: []string{"ls"},
		RunE: func(_ *cobra.Command, _ []string) error {
			return sshListRun()
		},
	}
}

func sshListRun() error {
	keys, err := ctr.SSHManager.List()
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		ui.Info("No SSH keys found")
		return nil
	}

	headers := []string{"Key", "Type", "Fingerprint", "Agent"}
	var rows [][]string
	for _, k := range keys {
		agent := ui.Red("✗")
		if k.InAgent {
			agent = ui.Green("✓")
		}
		rows = append(rows, []string{k.Path, k.Type, k.Fingerprint, agent})
	}

	ui.SimpleTable(headers, rows)
	return nil
}

func newSSHTestCmd() *cobra.Command {
	var providerName string
	cmd := &cobra.Command{
		Use:   "test <profile>",
		Short: "Test SSH connection to a provider",
		Long: `Test SSH authentication for a profile against a configured provider host.

Examples:
	gcm ssh test work-github --provider github
	gcm ssh test work-gitlab --provider gitlab`,
		Args: requireArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			p, err := ctr.ProfileManager.Get(args[0])
			if err != nil {
				ui.Error("profile %q not found", args[0])
				ui.Blank()
				ui.Print("  To see available profiles: gcm profile list")
				return nil
			}
			if p.SSH == nil || p.SSH.KeyPath == "" {
				ui.Error("profile %q has no SSH key configured", args[0])
				ui.Blank()
				ui.Print("  To generate one: gcm ssh generate %s", args[0])
				return nil
			}

			def, err := selectProfileProviderWithCapability(args[0], p, providerName, providerpkg.CapabilitySSHKeys)
			if err != nil {
				return err
			}
			host := def.SSHHost
			if host == "" {
				host = firstProviderHost(def)
			}

			sp := ui.NewSpinner(fmt.Sprintf("Testing SSH connection to %s...", def.DisplayName))
			sp.Start()

			output, testErr := ctr.SSHManager.TestConnectionToHost(p.SSH.KeyPath, host, def.SSHPort)
			if testErr != nil {
				sp.StopError("SSH test failed")
				ui.Error("%s", output)
				return testErr
			}

			sp.Stop(fmt.Sprintf("SSH connection to %s successful!", def.DisplayName))
			ui.Print(output)
			return nil
		},
	}
	cmd.Flags().StringVar(&providerName, "provider", "", "Provider to test (github, gitlab)")
	return cmd
}

func newSSHCopyCmd() *cobra.Command {
	return &cobra.Command{
		Use: "copy <profile>", Short: "Show public key", Args: requireArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			p, err := ctr.ProfileManager.Get(args[0])
			if err != nil {
				ui.Error("profile %q not found", args[0])
				ui.Blank()
				ui.Print("  To see available profiles: gcm profile list")
				return nil
			}
			if p.SSH == nil || p.SSH.KeyPath == "" {
				ui.Error("profile %q has no SSH key configured", args[0])
				ui.Blank()
				ui.Print("  To generate one: gcm ssh generate %s", args[0])
				return nil
			}
			pubKey, err := ctr.SSHManager.GetPublicKey(p.SSH.KeyPath)
			if err != nil {
				return err
			}
			ui.Print(pubKey)
			return nil
		},
	}
}

func newSSHUploadCmd() *cobra.Command {
	var (
		force        bool
		providerName string
	)

	cmd := &cobra.Command{
		Use:   "upload <profile>",
		Short: "Upload SSH key to a provider",
		Long: `Upload the profile's SSH public key to GitHub, GitLab, or another configured provider.

Checks for duplicates before uploading. Use --force to skip the check.

Examples:
	  gcm ssh upload work-github --provider github
	  gcm ssh upload work-gitlab --provider gitlab
	  gcm ssh upload work-gitlab --provider gitlab --force`,
		Args: requireArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			profileName := args[0]

			p, err := ctr.ProfileManager.Get(profileName)
			if err != nil {
				ui.Error("profile %q not found", profileName)
				return nil
			}
			if p.SSH == nil || p.SSH.KeyPath == "" {
				ui.Error("profile %q has no SSH key configured", profileName)
				ui.Blank()
				ui.Print("  To generate one: gcm ssh generate %s", profileName)
				return nil
			}

			def, err := selectProfileProviderWithCapability(profileName, p, providerName, providerpkg.CapabilitySSHKeys)
			if err != nil {
				return err
			}

			token, err := loadProviderToken(profileName, def, p)
			if err != nil || token.AccessToken == "" {
				ui.Error("No %s token found for profile %q", def.DisplayName, profileName)
				ui.Blank()
				ui.Print("  Connect first: gcm connect %s --provider %s", profileName, def.ID)
				return nil
			}

			pubKey, err := ctr.SSHManager.GetPublicKey(p.SSH.KeyPath)
			if err != nil {
				return fmt.Errorf("could not read public key: %w", err)
			}

			ctx := context.Background()

			// Check for duplicates
			if !force {
				sp := ui.NewSpinner(fmt.Sprintf("Checking if key already exists on %s...", def.DisplayName))
				sp.Start()

				exists, checkErr := providerSSHKeyExists(ctx, def, token, pubKey)
				if checkErr != nil {
					sp.StopError("Could not check existing keys")
					ui.Warning("Check failed: %v", checkErr)
					ui.Print("  Use --force to skip the duplicate check")
					return nil
				}
				if exists {
					sp.Stop(fmt.Sprintf("Key already exists on %s", def.DisplayName))
					ui.Info("This SSH key is already uploaded — no action needed.")
					return nil
				}
				sp.Stop(fmt.Sprintf("Key not found on %s — uploading", def.DisplayName))
			}

			sp2 := ui.NewSpinner(fmt.Sprintf("Uploading SSH key to %s...", def.DisplayName))
			sp2.Start()

			title := providerResourceName(profileName, def, "ssh", string(p.SSH.KeyType))
			if uploadErr := uploadProviderSSHKey(ctx, def, token, title, pubKey); uploadErr != nil {
				sp2.StopError("Failed to upload SSH key")
				ui.Warning("Upload failed: %v", uploadErr)
				ui.Print("  You can upload manually at: %s", providerManualKeyURL(def, "ssh"))
				return nil
			}

			sp2.Stop(fmt.Sprintf("SSH key uploaded to %s!", def.DisplayName))
			ui.Detail("Title", title)
			ctr.AuditLogger.Log(audit.ActionSSHGenerate, profileName,
				map[string]string{"action": "upload", "uploaded": "true", "provider": string(def.ID), "title": title}, nil)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip duplicate check")
	cmd.Flags().StringVar(&providerName, "provider", "", "Provider to upload to (github, gitlab)")
	return cmd
}
