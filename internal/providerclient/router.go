package providerclient

import (
	"context"
	"fmt"
	"sync"

	"git-config-manager/internal/github"
	"git-config-manager/internal/gitlab"
	providerpkg "git-config-manager/internal/provider"
)

// AuthenticatedUser is the provider-neutral identity returned after token verification.
type AuthenticatedUser struct {
	Username string
	Name     string
}

// Router dispatches provider operations to the concrete provider clients.
type Router struct {
	mu     sync.Mutex
	github *github.Client
	gitlab *gitlab.Client
}

// NewRouter creates a provider operation router for configured clients.
func NewRouter(githubClient *github.Client, gitlabClient *gitlab.Client) *Router {
	return &Router{github: githubClient, gitlab: gitlabClient}
}

// SetToken configures the concrete provider client for subsequent API calls.
func (r *Router) SetToken(def providerpkg.Definition, token providerpkg.TokenSet) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.setTokenLocked(def, token)
}

func (r *Router) setTokenLocked(def providerpkg.Definition, token providerpkg.TokenSet) error {
	if token.AccessToken == "" {
		return fmt.Errorf("%s token is empty", def.DisplayName)
	}
	switch def.ID {
	case providerpkg.GitHubID:
		if r.github == nil {
			return fmt.Errorf("GitHub client is not configured")
		}
		r.github.SetToken(token.AccessToken)
		return nil
	case providerpkg.GitLabID:
		if r.gitlab == nil {
			return fmt.Errorf("GitLab client is not configured")
		}
		r.gitlab.SetTokenSet(token)
		return nil
	default:
		return fmt.Errorf("provider %q is not implemented", def.ID)
	}
}

// VerifyPAT validates a Personal Access Token and returns the authenticated user.
func (r *Router) VerifyPAT(ctx context.Context, def providerpkg.Definition, token string) (AuthenticatedUser, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tokenSet := providerpkg.TokenSet{AccessToken: token, AuthMethod: providerpkg.AuthMethodPAT, TokenType: "pat"}
	switch def.ID {
	case providerpkg.GitHubID:
		if err := r.setTokenLocked(def, tokenSet); err != nil {
			return AuthenticatedUser{}, err
		}
		user, err := r.github.GetUser(ctx)
		if err != nil {
			return AuthenticatedUser{}, fmt.Errorf("GitHub token verification failed: %w", err)
		}
		return AuthenticatedUser{Username: user.Login, Name: user.Name}, nil
	case providerpkg.GitLabID:
		if err := r.setTokenLocked(def, tokenSet); err != nil {
			return AuthenticatedUser{}, err
		}
		user, err := r.gitlab.GetUser(ctx)
		if err != nil {
			return AuthenticatedUser{}, fmt.Errorf("GitLab token verification failed: %w", err)
		}
		return AuthenticatedUser{Username: user.Username, Name: user.Name}, nil
	default:
		return AuthenticatedUser{}, fmt.Errorf("provider %q is not implemented yet", def.ID)
	}
}

// SSHKeyExists reports whether a provider already has the given public key.
func (r *Router) SSHKeyExists(ctx context.Context, def providerpkg.Definition, token providerpkg.TokenSet, publicKey string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch def.ID {
	case providerpkg.GitHubID:
		if err := r.setTokenLocked(def, token); err != nil {
			return false, err
		}
		return r.github.SSHKeyExists(ctx, publicKey)
	case providerpkg.GitLabID:
		if err := r.setTokenLocked(def, token); err != nil {
			return false, err
		}
		return r.gitlab.SSHKeyExists(ctx, publicKey)
	default:
		return false, fmt.Errorf("provider %q does not support SSH key upload yet", def.ID)
	}
}

// UploadSSHKey uploads a public SSH key to the provider account.
func (r *Router) UploadSSHKey(ctx context.Context, def providerpkg.Definition, token providerpkg.TokenSet, title, publicKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch def.ID {
	case providerpkg.GitHubID:
		if err := r.setTokenLocked(def, token); err != nil {
			return err
		}
		return r.github.UploadSSHKey(ctx, title, publicKey)
	case providerpkg.GitLabID:
		if err := r.setTokenLocked(def, token); err != nil {
			return err
		}
		return r.gitlab.UploadSSHKey(ctx, title, publicKey)
	default:
		return fmt.Errorf("provider %q does not support SSH key upload yet", def.ID)
	}
}

// DeleteSSHKey removes a public SSH key from the provider account when found.
func (r *Router) DeleteSSHKey(ctx context.Context, def providerpkg.Definition, token providerpkg.TokenSet, publicKey string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch def.ID {
	case providerpkg.GitHubID:
		if err := r.setTokenLocked(def, token); err != nil {
			return false, err
		}
		return r.github.DeleteSSHKey(ctx, publicKey)
	case providerpkg.GitLabID:
		if err := r.setTokenLocked(def, token); err != nil {
			return false, err
		}
		return r.gitlab.DeleteSSHKey(ctx, publicKey)
	default:
		return false, fmt.Errorf("provider %q does not support SSH key deletion yet", def.ID)
	}
}

// GPGKeyExists reports whether a provider already has the given GPG key.
func (r *Router) GPGKeyExists(ctx context.Context, def providerpkg.Definition, token providerpkg.TokenSet, keyID string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch def.ID {
	case providerpkg.GitHubID:
		if err := r.setTokenLocked(def, token); err != nil {
			return false, err
		}
		return r.github.GPGKeyExists(ctx, keyID)
	case providerpkg.GitLabID:
		if err := r.setTokenLocked(def, token); err != nil {
			return false, err
		}
		return r.gitlab.GPGKeyExists(ctx, keyID)
	default:
		return false, fmt.Errorf("provider %q does not support GPG key upload yet", def.ID)
	}
}

// UploadGPGKey uploads an armored GPG public key to the provider account.
func (r *Router) UploadGPGKey(ctx context.Context, def providerpkg.Definition, token providerpkg.TokenSet, armoredKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch def.ID {
	case providerpkg.GitHubID:
		if err := r.setTokenLocked(def, token); err != nil {
			return err
		}
		return r.github.UploadGPGKey(ctx, armoredKey)
	case providerpkg.GitLabID:
		if err := r.setTokenLocked(def, token); err != nil {
			return err
		}
		return r.gitlab.UploadGPGKey(ctx, armoredKey)
	default:
		return fmt.Errorf("provider %q does not support GPG key upload yet", def.ID)
	}
}

// DeleteGPGKey removes a GPG key from the provider account when found.
func (r *Router) DeleteGPGKey(ctx context.Context, def providerpkg.Definition, token providerpkg.TokenSet, keyID string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch def.ID {
	case providerpkg.GitHubID:
		if err := r.setTokenLocked(def, token); err != nil {
			return false, err
		}
		return r.github.DeleteGPGKey(ctx, keyID)
	case providerpkg.GitLabID:
		if err := r.setTokenLocked(def, token); err != nil {
			return false, err
		}
		return r.gitlab.DeleteGPGKey(ctx, keyID)
	default:
		return false, fmt.Errorf("provider %q does not support GPG key deletion yet", def.ID)
	}
}
