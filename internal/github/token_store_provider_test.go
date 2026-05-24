package github

import (
	"testing"

	"git-config-manager/internal/provider"
)

func TestTokenStore_ProviderTokenSet_SaveLoadDelete(t *testing.T) {
	ts := newPlainStore(t)
	key := provider.TokenKey{Profile: "work", Provider: provider.GitLabID, Host: "gitlab.com"}
	token := provider.TokenSet{AccessToken: "glpat-secret", AuthMethod: provider.AuthMethodPAT, Scopes: []string{"api"}}

	if err := ts.SaveTokenSet(key, token); err != nil {
		t.Fatalf("SaveTokenSet: %v", err)
	}
	loaded, err := ts.LoadTokenSet(key)
	if err != nil {
		t.Fatalf("LoadTokenSet: %v", err)
	}
	if loaded.AccessToken != token.AccessToken {
		t.Fatalf("AccessToken = %q, want %q", loaded.AccessToken, token.AccessToken)
	}
	if loaded.AuthMethod != provider.AuthMethodPAT {
		t.Fatalf("AuthMethod = %q", loaded.AuthMethod)
	}
	if len(loaded.Scopes) != 1 || loaded.Scopes[0] != "api" {
		t.Fatalf("Scopes = %#v", loaded.Scopes)
	}

	if err := ts.DeleteTokenSet(key); err != nil {
		t.Fatalf("DeleteTokenSet: %v", err)
	}
	if _, err := ts.LoadTokenSet(key); err == nil {
		t.Fatal("expected error after DeleteTokenSet")
	}
}

func TestTokenStore_ProviderTokenSet_DoesNotCollideAcrossProviders(t *testing.T) {
	ts := newPlainStore(t)
	githubKey := provider.TokenKey{Profile: "work", Provider: provider.GitHubID, Host: "github.com"}
	gitlabKey := provider.TokenKey{Profile: "work", Provider: provider.GitLabID, Host: "gitlab.com"}

	if err := ts.SaveTokenSet(githubKey, provider.TokenSet{AccessToken: "gh-token", AuthMethod: provider.AuthMethodPAT}); err != nil {
		t.Fatalf("SaveTokenSet github: %v", err)
	}
	if err := ts.SaveTokenSet(gitlabKey, provider.TokenSet{AccessToken: "gl-token", AuthMethod: provider.AuthMethodPAT}); err != nil {
		t.Fatalf("SaveTokenSet gitlab: %v", err)
	}

	githubToken, err := ts.LoadTokenSet(githubKey)
	if err != nil {
		t.Fatalf("LoadTokenSet github: %v", err)
	}
	gitlabToken, err := ts.LoadTokenSet(gitlabKey)
	if err != nil {
		t.Fatalf("LoadTokenSet gitlab: %v", err)
	}
	if githubToken.AccessToken != "gh-token" || gitlabToken.AccessToken != "gl-token" {
		t.Fatalf("tokens collided: github=%q gitlab=%q", githubToken.AccessToken, gitlabToken.AccessToken)
	}
}

func TestTokenStore_ProviderTokenSet_FallsBackToLegacyGitHubToken(t *testing.T) {
	ts := newPlainStore(t)
	if err := ts.Save("work", "legacy-gh-token"); err != nil {
		t.Fatalf("Save legacy token: %v", err)
	}

	loaded, err := ts.LoadTokenSet(provider.TokenKey{Profile: "work", Provider: provider.GitHubID, Host: "github.com"})
	if err != nil {
		t.Fatalf("LoadTokenSet fallback: %v", err)
	}
	if loaded.AccessToken != "legacy-gh-token" {
		t.Fatalf("AccessToken = %q", loaded.AccessToken)
	}
	if loaded.AuthMethod != provider.AuthMethodLegacy {
		t.Fatalf("AuthMethod = %q", loaded.AuthMethod)
	}
}

func TestTokenStore_ProviderTokenSet_RequiresProvider(t *testing.T) {
	ts := newPlainStore(t)
	err := ts.SaveTokenSet(provider.TokenKey{Profile: "work"}, provider.TokenSet{AccessToken: "tok"})
	if err == nil {
		t.Fatal("expected error for empty provider")
	}
}
