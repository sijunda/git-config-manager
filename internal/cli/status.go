package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	providerpkg "git-config-manager/internal/provider"
	"git-config-manager/pkg/ui"

	"github.com/spf13/cobra"
)

// quickVerifyToken checks token validity with a short deadline without
// mutating the shared GitHubClient. This avoids a data race when verifying
// multiple profiles in sequence with goroutine-based timeouts.
func quickVerifyToken(token, apiURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL+"/user", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	// Drain body before close to allow connection reuse.
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096)) //nolint:errcheck
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func quickVerifyGitLabToken(token providerpkg.TokenSet, apiURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(apiURL, "/")+"/user", nil)
	if err != nil {
		return err
	}
	if token.Bearer() {
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	} else {
		req.Header.Set("PRIVATE-TOKEN", token.AccessToken)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096)) //nolint:errcheck
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// padRight pads a string to the given visible width with spaces.
func padRight(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show a quick overview of your GCM setup",
		Long: `Display a dashboard of your current GCM state at a glance.

Shows: active profile, all profiles summary, GitHub auth status,
SSH keys, and any issues that need attention.`,
		Aliases: []string{"st"},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runStatus()
		},
	}
}

func runStatus() error {
	ui.Header("%s GCM Status", ui.IconRocket)
	ui.Blank()

	profiles, _ := ctr.ProfileManager.List()
	currentName, scope, _ := ctr.ProfileSwitcher.Current()

	// Calculate max profile name length for alignment
	maxNameLen := 0
	for _, p := range profiles {
		if len(p.Name) > maxNameLen {
			maxNameLen = len(p.Name)
		}
	}
	if maxNameLen < 6 {
		maxNameLen = 6
	}
	// Add 1 for breathing room
	maxNameLen++

	// ─── Active Profile ───
	ui.Print("  %s", ui.Bold("Active Profile"))
	if currentName == "" {
		ui.Print("    %s No active profile", ui.Red(ui.IconError))
		ui.Print("      Activate one: %s", ui.Cyan("gcm use <profile>"))
	} else {
		p, _ := ctr.ProfileManager.Get(currentName)
		if p != nil {
			ui.Print("    %s %s (%s)", ui.Green(ui.IconSuccess), ui.Bold(currentName), scope.String())
			ui.Print("      %s <%s>", p.Git.User.Name, p.Git.User.Email)
		} else {
			ui.Print("    %s %s (%s)", ui.Green(ui.IconSuccess), ui.Bold(currentName), scope.String())
		}
	}

	ui.Blank()
	ui.Divider()

	// ─── Profiles ───
	ui.Blank()
	ui.Print("  %s %s", ui.Bold("Profiles"), ui.Dim(fmt.Sprintf("(%d total)", len(profiles))))

	if len(profiles) == 0 {
		ui.Print("    %s No profiles yet", ui.Yellow(ui.IconWarning))
		ui.Print("      Create one: %s", ui.Cyan("gcm profile create work -i"))
	} else {
		for _, p := range profiles {
			marker := ui.Dim("•")
			if p.Name == currentName {
				marker = ui.Green("●")
			}
			extras := ""
			if p.SSH != nil {
				extras += " " + ui.IconKey
			}
			if p.GPG != nil {
				extras += " 🔏"
			}
			ui.Print("    %s %-*s %s%s", marker, maxNameLen, p.Name, ui.Dim(p.Git.User.Email), extras)
		}
	}

	ui.Blank()
	ui.Divider()

	// ─── GitHub Auth ───
	ui.Blank()
	ui.Print("  %s", ui.Bold("GitHub Auth"))

	var issues []string

	// Calculate max username length for alignment
	maxUserLen := 0
	if len(profiles) == 0 {
		ui.Print("    %s No profiles configured", ui.Dim("—"))
	} else {
		// Pre-compute usernames for alignment
		type ghEntry struct {
			icon     string
			name     string
			username string
			status   string
			hint     string
		}
		entries := make([]ghEntry, 0, len(profiles))

		for _, p := range profiles {
			token, loadErr := ctr.GitHubClient.LoadToken(p.Name)
			if loadErr != nil || token == "" {
				e := ghEntry{
					icon:   ui.Red(ui.IconError),
					name:   p.Name,
					status: ui.Dim("not configured"),
					hint:   fmt.Sprintf("gcm github login %s", p.Name),
				}
				entries = append(entries, e)
				continue
			}

			// Quick verification (context-bounded, no shared state mutation)
			username := ""
			if p.GitHub != nil && p.GitHub.Username != "" {
				username = "@" + p.GitHub.Username
			}
			if len(username) > maxUserLen {
				maxUserLen = len(username)
			}

			status := ui.Green("valid")
			icon := ui.Green(ui.IconSuccess)
			if verifyErr := quickVerifyToken(token, ctr.Config.GitHub.APIURL); verifyErr != nil {
				if strings.Contains(verifyErr.Error(), "context deadline exceeded") ||
					strings.Contains(verifyErr.Error(), "timeout") {
					status = ui.Yellow("timeout")
					icon = ui.Yellow(ui.IconWarning)
				} else {
					status = ui.Red("expired/invalid")
					icon = ui.Red(ui.IconError)
					issues = append(issues, fmt.Sprintf("Token for %q expired — run: gcm github login %s", p.Name, p.Name))
				}
			}

			entries = append(entries, ghEntry{
				icon:     icon,
				name:     p.Name,
				username: username,
				status:   status,
			})
		}

		if maxUserLen < 12 {
			maxUserLen = 12
		}

		for _, e := range entries {
			if e.hint != "" {
				ui.Print("    %s %-*s %s %s", e.icon, maxNameLen, e.name, ui.Dim(padRight("—", maxUserLen)), e.status)
				ui.Print("      %s %s", ui.Dim("└─"), ui.Cyan(e.hint))
			} else {
				ui.Print("    %s %-*s %s %s", e.icon, maxNameLen, e.name, ui.Dim(padRight(e.username, maxUserLen)), e.status)
			}
		}
	}

	ui.Blank()
	ui.Divider()

	// ─── GitLab Auth ───
	ui.Blank()
	ui.Print("  %s", ui.Bold("GitLab Auth"))

	gitlabDef, gitlabConfigured := ctr.ProviderRegistry.Get(providerpkg.GitLabID)
	if !gitlabConfigured {
		ui.Print("    %s GitLab provider not configured", ui.Dim("—"))
	} else if len(profiles) == 0 {
		ui.Print("    %s No profiles configured", ui.Dim("—"))
	} else {
		type glEntry struct {
			icon     string
			name     string
			username string
			status   string
			hint     string
		}
		entries := make([]glEntry, 0, len(profiles))
		maxGitLabUserLen := 0

		for _, p := range profiles {
			token, loadErr := loadProviderToken(p.Name, gitlabDef, p)
			if loadErr != nil || token.AccessToken == "" {
				entries = append(entries, glEntry{
					icon:   ui.Red(ui.IconError),
					name:   p.Name,
					status: ui.Dim("not configured"),
					hint:   fmt.Sprintf("gcm gitlab login %s", p.Name),
				})
				continue
			}

			account := providerAccountForProfile(p, providerpkg.GitLabID)
			username := ""
			if account.Username != "" {
				username = "@" + account.Username
			}
			if len(username) > maxGitLabUserLen {
				maxGitLabUserLen = len(username)
			}

			status := ui.Green("valid")
			icon := ui.Green(ui.IconSuccess)
			if verifyErr := quickVerifyGitLabToken(token, gitlabDef.APIURL); verifyErr != nil {
				if strings.Contains(verifyErr.Error(), "context deadline exceeded") || strings.Contains(verifyErr.Error(), "timeout") {
					status = ui.Yellow("timeout")
					icon = ui.Yellow(ui.IconWarning)
				} else {
					status = ui.Red("expired/invalid")
					icon = ui.Red(ui.IconError)
					issues = append(issues, fmt.Sprintf("GitLab token for %q expired — run: gcm gitlab login %s", p.Name, p.Name))
				}
			}

			entries = append(entries, glEntry{icon: icon, name: p.Name, username: username, status: status})
		}

		if maxGitLabUserLen < 12 {
			maxGitLabUserLen = 12
		}
		for _, e := range entries {
			if e.hint != "" {
				ui.Print("    %s %-*s %s %s", e.icon, maxNameLen, e.name, ui.Dim(padRight("—", maxGitLabUserLen)), e.status)
				ui.Print("      %s %s", ui.Dim("└─"), ui.Cyan(e.hint))
			} else {
				ui.Print("    %s %-*s %s %s", e.icon, maxNameLen, e.name, ui.Dim(padRight(e.username, maxGitLabUserLen)), e.status)
			}
		}
	}

	ui.Blank()
	ui.Divider()

	// ─── SSH Keys ───
	ui.Blank()
	ui.Print("  %s", ui.Bold("SSH Keys"))

	// Calculate max key filename length
	maxKeyLen := 0
	for _, p := range profiles {
		if p.SSH != nil && p.SSH.KeyPath != "" {
			kl := len(filepath.Base(p.SSH.KeyPath))
			if kl > maxKeyLen {
				maxKeyLen = kl
			}
		}
	}
	if maxKeyLen < 10 {
		maxKeyLen = 10
	}
	maxKeyLen += 2

	hasKeys := false
	for _, p := range profiles {
		if p.SSH != nil && p.SSH.KeyPath != "" {
			hasKeys = true
			icon := ui.Green(ui.IconSuccess)
			if _, statErr := os.Stat(p.SSH.KeyPath); statErr != nil {
				icon = ui.Red(ui.IconError)
				issues = append(issues, fmt.Sprintf("SSH key for %q missing at %s", p.Name, p.SSH.KeyPath))
			}
			keyName := padRight(filepath.Base(p.SSH.KeyPath), maxKeyLen)
			ui.Print("    %s %-*s %s %s", icon, maxNameLen, p.Name, keyName, ui.Dim(string(p.SSH.KeyType)))
		} else {
			ui.Print("    %s %-*s %s", ui.Dim("—"), maxNameLen, p.Name, ui.Dim("not configured"))
		}
	}

	if !hasKeys && len(profiles) == 0 {
		ui.Print("    %s No SSH keys configured", ui.Dim("—"))
		ui.Print("      Generate: %s", ui.Cyan("gcm ssh generate <profile>"))
	}

	// ─── Issues / Suggestions ───
	if len(issues) > 0 {
		ui.Blank()
		ui.Divider()
		ui.Blank()
		ui.Print("  %s %s", ui.Bold("Issues"), ui.Red(fmt.Sprintf("(%d)", len(issues))))
		for _, issue := range issues {
			ui.Print("    %s %s", ui.Red(ui.IconArrow), issue)
		}
	}

	// ─── Quick Tips (if new user) ───
	if len(profiles) == 0 {
		ui.Blank()
		ui.Divider()
		ui.Blank()
		ui.Print("  %s", ui.Bold("Quick Start"))
		ui.Print("    Run %s for a guided setup wizard", ui.Cyan("gcm setup"))
	}

	ui.Blank()
	return nil
}
