package provider

import (
	"testing"

	"git-config-manager/internal/config"
)

func TestRegistry_DefaultProvidersResolveHosts(t *testing.T) {
	cfg := config.DefaultConfig()
	registry := NewRegistry(cfg)

	githubDef, ok := registry.ResolveHost("github.com")
	if !ok {
		t.Fatal("expected github.com to resolve")
	}
	if githubDef.ID != GitHubID {
		t.Fatalf("github.com resolved to %q, want %q", githubDef.ID, GitHubID)
	}

	gitlabDef, ok := registry.ResolveHost("https://gitlab.com")
	if !ok {
		t.Fatal("expected gitlab.com to resolve")
	}
	if gitlabDef.ID != GitLabID {
		t.Fatalf("gitlab.com resolved to %q, want %q", gitlabDef.ID, GitLabID)
	}
}

func TestRegistry_LegacyGitHubCustomAPIURLOverridesDefaultProviderHost(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GitHub.APIURL = "https://github.company.test/api/v3"

	registry := NewRegistry(cfg)
	def, ok := registry.Get(GitHubID)
	if !ok {
		t.Fatal("expected GitHub provider")
	}
	if def.WebURL != "https://github.company.test" {
		t.Fatalf("WebURL = %q, want custom enterprise web URL", def.WebURL)
	}
	if def.CredentialServer() != "https://github.company.test" {
		t.Fatalf("CredentialServer = %q", def.CredentialServer())
	}
	resolved, ok := registry.ResolveHost("github.company.test")
	if !ok || resolved.ID != GitHubID {
		t.Fatalf("custom host did not resolve to GitHub")
	}
	if _, ok := registry.ResolveHost("github.com"); ok {
		t.Fatal("default github.com host should not resolve after legacy custom API override")
	}
}

func TestRegistry_AllIsDeterministic(t *testing.T) {
	registry := &Registry{
		providers: make(map[ProviderID]Definition),
		hostIndex: make(map[string]ProviderID),
	}
	registry.Register(Definition{ID: GitLabID})
	registry.Register(Definition{ID: GitHubID})

	defs := registry.All()
	if len(defs) != 2 {
		t.Fatalf("len(All()) = %d, want 2", len(defs))
	}
	if defs[0].ID != GitHubID || defs[1].ID != GitLabID {
		t.Fatalf("All() order = %q, %q", defs[0].ID, defs[1].ID)
	}
}

func TestDefinition_CredentialUsernameStrategies(t *testing.T) {
	gitlab := Definition{ID: GitLabID}
	if got := gitlab.CredentialUsername("work", "jane", TokenSet{AuthMethod: AuthMethodPAT}); got != "jane" {
		t.Fatalf("GitLab PAT username = %q", got)
	}
	if got := gitlab.CredentialUsername("work", "jane", TokenSet{AuthMethod: AuthMethodOAuthDevice}); got != "oauth2" {
		t.Fatalf("GitLab OAuth username = %q", got)
	}

	github := Definition{ID: GitHubID}
	if got := github.CredentialUsername("work", "", TokenSet{}); got != "work" {
		t.Fatalf("GitHub fallback username = %q", got)
	}
}
