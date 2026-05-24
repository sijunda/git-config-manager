package cli

import (
	"context"
	"fmt"

	"git-config-manager/internal/audit"
	providerpkg "git-config-manager/internal/provider"
	"git-config-manager/pkg/ui"

	"github.com/spf13/cobra"
)

func newGitLabCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gitlab",
		Short:   "Manage GitLab integration",
		Aliases: []string{"gl"},
	}

	cmd.AddCommand(newGitLabLoginCmd())
	cmd.AddCommand(newGitLabLogoutCmd())
	cmd.AddCommand(newGitLabVerifyCmd())
	cmd.AddCommand(newGitLabUserCmd())
	cmd.AddCommand(newGitLabStatusCmd())

	return cmd
}

func gitLabProviderDefinition() (providerpkg.Definition, error) {
	def, ok := ctr.ProviderRegistry.Get(providerpkg.GitLabID)
	if !ok {
		return providerpkg.Definition{}, fmt.Errorf("GitLab provider is not configured")
	}
	return def, nil
}

func newGitLabLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login <profile>",
		Short: "Authenticate with GitLab using a Personal Access Token (PAT)",
		Long: `Authenticate with GitLab using a Personal Access Token.

For GitLab.com, create a token at https://gitlab.com/-/user_settings/personal_access_tokens.
For self-managed GitLab, configure providers.gitlab.api_url/web_url/git_hosts first.

Recommended scopes for this MVP: api, read_user, read_repository, write_repository.

Examples:
  gcm gitlab login work
  echo "$GITLAB_TOKEN" | gcm gitlab login work`,
		Args: requireArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]
			profileConfig, err := ctr.ProfileManager.Get(profileName)
			if err != nil {
				return fmt.Errorf("profile %q not found\n\n  To see available profiles: gcm profile list\n  To create a new profile:   gcm profile create %s -i", profileName, profileName)
			}

			def, err := gitLabProviderDefinition()
			if err != nil {
				return err
			}

			var token string
			stdinPiped := isStdinPiped()
			if stdinPiped {
				token, err = readStdinToken()
				if err != nil {
					return fmt.Errorf("could not read token from input\n\n  Make sure you're piping a valid token:\n  echo \"$GITLAB_TOKEN\" | gcm gitlab login %s", profileName)
				}
			} else {
				ui.Header("%s GitLab Login for Profile: %s", ui.IconKey, profileName)
				ui.Blank()
				ui.Print("Create a GitLab Personal Access Token with scopes: api, read_user, read_repository, write_repository")
				ui.Print("Provider: %s", ui.Cyan(def.WebURL))
				ui.Blank()
				token, err = ui.AskPassword("Enter token")
				if err != nil {
					return fmt.Errorf("could not read token input")
				}
			}

			if token == "" {
				return fmt.Errorf("token cannot be empty")
			}

			tokenSet := providerpkg.TokenSet{AccessToken: token, AuthMethod: providerpkg.AuthMethodPAT, TokenType: "pat"}
			ctr.GitLabClient.SetTokenSet(tokenSet)

			sp := ui.NewSpinner("Verifying token with GitLab...")
			sp.Start()
			user, err := ctr.GitLabClient.GetUser(cmd.Context())
			if err != nil {
				sp.StopError("Token is not valid")
				ui.Blank()
				ui.Print("The token was rejected by GitLab at %s.", def.APIURL)
				ui.Print("Common causes: missing scope, expired token, revoked token, or wrong self-managed URL.")
				return fmt.Errorf("GitLab token verification failed")
			}
			sp.Stop("Token verified!")

			if err := saveProviderToken(profileName, def, profileConfig, tokenSet); err != nil {
				ctr.AuditLogger.Log(audit.ActionProviderLogin, profileName,
					map[string]string{"provider": string(providerpkg.GitLabID), "method": "pat"}, err)
				return fmt.Errorf("could not save the GitLab token securely\n\n  This might be a file permission issue. Run: gcm doctor")
			}

			setProfileProviderAccount(profileConfig, providerpkg.GitLabID, user.Username, providerpkg.AuthMethodPAT)
			if err := ctr.ProfileManager.Update(profileConfig); err != nil {
				ui.Warning("GitLab token was saved, but profile metadata could not be updated: %v", err)
			}

			ctr.AuditLogger.Log(audit.ActionProviderLogin, profileName,
				map[string]string{"provider": string(providerpkg.GitLabID), "user": user.Username, "method": "pat"}, nil)

			ui.Blank()
			if user.Name != "" {
				ui.Success("Logged in to GitLab as %s (%s)", ui.Bold(user.Username), user.Name)
			} else {
				ui.Success("Logged in to GitLab as %s", ui.Bold(user.Username))
			}
			if user.WebURL != "" {
				ui.Detail("GitLab Profile", user.WebURL)
			}

			if isActiveProfile(profileName) {
				configureGitCredentialsForProvider(profileName, profileConfig, def, tokenSet)
				ui.Print("  Git credentials updated — git push/pull will use this GitLab account.")
			} else {
				ui.Blank()
				ui.Print("  Note: This is not the active profile.")
				ui.Print("  Git credentials will be updated when you switch to it:")
				ui.Print("    gcm use %s", profileName)
			}

			activateAsGlobalIfFirst(profileName)
			if !stdinPiped {
				setupUploadKeysForGitLab(cmd.Context(), profileName)
			}
			return nil
		},
	}
}

func newGitLabLogoutCmd() *cobra.Command {
	var clearGitCreds bool
	var forceLogout bool

	cmd := &cobra.Command{
		Use:   "logout <profile>",
		Short: "Remove stored GitLab token for a profile",
		Args:  requireArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			profileName := args[0]
			profileConfig, err := ctr.ProfileManager.Get(profileName)
			if err != nil {
				return fmt.Errorf("profile %q not found", profileName)
			}

			if !isActiveProfile(profileName) && !forceLogout {
				ui.Warning("Profile %q is not the active profile.", profileName)
				confirm, askErr := ui.AskConfirm(fmt.Sprintf("Remove the GitLab token for %q?", profileName), false)
				if askErr != nil || !confirm {
					ui.Info("Cancelled.")
					return nil
				}
			}

			def, err := gitLabProviderDefinition()
			if err != nil {
				return err
			}

			if err := deleteProviderToken(profileName, def, profileConfig); err != nil {
				ctr.AuditLogger.Log(audit.ActionProviderLogout, profileName,
					map[string]string{"provider": string(providerpkg.GitLabID)}, err)
				return fmt.Errorf("could not remove GitLab token for profile %q", profileName)
			}

			ctr.AuditLogger.Log(audit.ActionProviderLogout, profileName,
				map[string]string{"provider": string(providerpkg.GitLabID)}, nil)
			ui.Success("GitLab token removed for profile %q", profileName)

			if clearGitCreds && isActiveProfile(profileName) {
				if err := ctr.GitHubClient.ClearGitCredentials(def.CredentialServer()); err != nil {
					ui.Warning("Git credentials could not be cleared automatically.")
				} else {
					ui.Success("GitLab git credentials cleared.")
				}
			}

			ui.Blank()
			ui.Print("To re-authenticate later: gcm gitlab login %s", profileName)
			return nil
		},
	}

	cmd.Flags().BoolVar(&clearGitCreds, "clear-credentials", true, "Also clear cached git credentials")
	cmd.Flags().BoolVarP(&forceLogout, "force", "f", false, "Skip confirmation when logging out a non-active profile")
	return cmd
}

func newGitLabVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify <profile>",
		Short: "Verify that the stored GitLab token is still valid",
		Args:  requireArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]
			profileConfig, err := ctr.ProfileManager.Get(profileName)
			if err != nil {
				return fmt.Errorf("profile %q not found", profileName)
			}
			def, err := gitLabProviderDefinition()
			if err != nil {
				return err
			}
			token, err := loadProviderToken(profileName, def, profileConfig)
			if err != nil {
				ui.Print("Profile %q is not authenticated with GitLab yet.", profileName)
				ui.Print("To authenticate: gcm gitlab login %s", profileName)
				return fmt.Errorf("profile %q is not authenticated with GitLab", profileName)
			}
			ctr.GitLabClient.SetTokenSet(token)
			user, err := ctr.GitLabClient.VerifyToken(cmd.Context())
			if err != nil {
				ui.Print("The stored GitLab token for profile %q is no longer valid.", profileName)
				ui.Print("To fix, re-authenticate: gcm gitlab login %s", profileName)
				return fmt.Errorf("GitLab token expired or revoked for profile %q", profileName)
			}
			ui.Success("Authenticated with GitLab as %s", ui.Bold(user.Username))
			return nil
		},
	}
}

func newGitLabUserCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "user <profile>",
		Short: "Show GitLab user information for a profile",
		Args:  requireArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]
			profileConfig, err := ctr.ProfileManager.Get(profileName)
			if err != nil {
				return fmt.Errorf("profile %q not found", profileName)
			}
			def, err := gitLabProviderDefinition()
			if err != nil {
				return err
			}
			token, err := loadProviderToken(profileName, def, profileConfig)
			if err != nil {
				ui.Print("Profile %q is not authenticated with GitLab yet.", profileName)
				ui.Print("To authenticate: gcm gitlab login %s", profileName)
				return fmt.Errorf("profile %q is not authenticated with GitLab", profileName)
			}
			ctr.GitLabClient.SetTokenSet(token)
			user, err := ctr.GitLabClient.GetUser(cmd.Context())
			if err != nil {
				return fmt.Errorf("could not fetch GitLab user info for %q", profileName)
			}
			ui.Header("GitLab User: %s", user.Username)
			ui.Detail("Name", user.Name)
			ui.Detail("Email", user.Email)
			ui.Detail("Public Email", user.PublicEmail)
			ui.Detail("URL", user.WebURL)
			return nil
		},
	}
}

func newGitLabStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show GitLab authentication status for all profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			profiles, err := ctr.ProfileManager.List()
			if err != nil {
				return err
			}
			def, err := gitLabProviderDefinition()
			if err != nil {
				return err
			}

			ui.Header("%s GitLab Authentication Status", ui.IconGlobe)
			ui.Blank()

			headers := []string{"Profile", "Status", "Username", "Method"}
			var rows [][]string
			for _, profileConfig := range profiles {
				status := ui.Red("not authenticated")
				username := "-"
				method := "-"

				token, err := loadProviderToken(profileConfig.Name, def, profileConfig)
				if err == nil && token.AccessToken != "" {
					ctr.GitLabClient.SetTokenSet(token)
					if user, verr := ctr.GitLabClient.GetUser(cmd.Context()); verr == nil {
						status = ui.Green("authenticated")
						username = user.Username
						method = token.AuthMethod
					} else {
						status = ui.Yellow("token expired")
					}
				}
				account := providerAccountForProfile(profileConfig, providerpkg.GitLabID)
				if account.Username != "" && username == "-" {
					username = account.Username + " (cached)"
				}

				rows = append(rows, []string{profileConfig.Name, status, username, method})
			}

			ui.SimpleTable(headers, rows)
			return nil
		},
	}
}

func setupUploadKeysForGitLab(ctx context.Context, profileName string) {
	def, err := gitLabProviderDefinition()
	if err != nil || !def.UploadKeys {
		return
	}

	profileConfig, err := ctr.ProfileManager.Get(profileName)
	if err != nil || profileConfig == nil {
		return
	}
	token, err := loadProviderToken(profileName, def, profileConfig)
	if err != nil || token.AccessToken == "" {
		return
	}
	ctr.GitLabClient.SetTokenSet(token)

	if profileConfig.SSH != nil && profileConfig.SSH.KeyPath != "" {
		pubKey, pubErr := ctr.SSHManager.GetPublicKey(profileConfig.SSH.KeyPath)
		if pubErr == nil && pubKey != "" {
			exists, checkErr := ctr.GitLabClient.SSHKeyExists(ctx, pubKey)
			if checkErr == nil && !exists {
				ui.Blank()
				upload, askErr := ui.AskConfirm("Upload SSH key to GitLab?", true)
				if askErr == nil && upload {
					title := fmt.Sprintf("gcm-%s", profileName)
					if uploadErr := ctr.GitLabClient.UploadSSHKey(ctx, title, pubKey); uploadErr != nil {
						ui.Warning("Could not upload SSH key to GitLab: %v", uploadErr)
					} else {
						ui.Success("SSH key uploaded to GitLab")
					}
				}
			} else if checkErr == nil && exists {
				ui.Blank()
				ui.Success("SSH key already on GitLab")
			}
		}
	}

	if profileConfig.GPG != nil && profileConfig.GPG.KeyID != "" {
		exists, checkErr := ctr.GitLabClient.GPGKeyExists(ctx, profileConfig.GPG.KeyID)
		if checkErr == nil && !exists {
			upload, askErr := ui.AskConfirm("Upload GPG key to GitLab?", true)
			if askErr == nil && upload {
				pubKey, gpgErr := ctr.GPGManager.GetPublicKey(profileConfig.GPG.KeyID)
				if gpgErr != nil {
					ui.Warning("Could not read GPG public key: %v", gpgErr)
				} else if uploadErr := ctr.GitLabClient.UploadGPGKey(ctx, pubKey); uploadErr != nil {
					ui.Warning("Could not upload GPG key to GitLab: %v", uploadErr)
				} else {
					ui.Success("GPG key uploaded to GitLab")
				}
			}
		} else if checkErr == nil && exists {
			ui.Success("GPG key already on GitLab")
		}
	}
}
