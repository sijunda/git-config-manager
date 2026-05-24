# Provider Integrations

GCM supports Git hosting providers through a provider-aware architecture. GitHub remains backward compatible, and GitLab is available as the first non-GitHub provider. Bitbucket support should be added by implementing the same provider contracts instead of adding provider-specific branching in CLI commands.

---

## Current Providers

| Provider | Status | Auth | Credential Helper | SSH Keys | GPG Keys |
| -------- | ------ | ---- | ----------------- | -------- | -------- |
| GitHub | Stable | PAT, OAuth device flow, GitHub CLI import | Yes | Yes | Yes |
| GitLab | MVP | PAT | Yes | Yes | Yes |
| Bitbucket | Planned | Not implemented | Planned | Planned | Planned |

GitLab MVP targets the production-critical profile workflow first: PAT login, token verification, Git credential resolution, status reporting, and SSH/GPG key upload.

---

## Configuration

Provider definitions live under `providers` in `~/.gcm/config.yaml`.

```yaml
providers:
  github:
    type: github
    api_url: https://api.github.com
    web_url: https://github.com
    git_hosts:
      - github.com
    ssh_host: github.com
    upload_keys: true
    auth:
      default_method: pat
      scopes:
        - repo
        - admin:public_key
        - admin:gpg_key

  gitlab:
    type: gitlab
    api_url: https://gitlab.com/api/v4
    web_url: https://gitlab.com
    git_hosts:
      - gitlab.com
    ssh_host: gitlab.com
    upload_keys: true
    auth:
      default_method: pat
      scopes:
        - api
        - read_user
        - read_repository
        - write_repository
```

For self-managed GitLab, configure all three URL concepts explicitly:

```yaml
providers:
  gitlab:
    type: gitlab
    api_url: https://gitlab.company.example/api/v4
    web_url: https://gitlab.company.example
    git_hosts:
      - gitlab.company.example
    ssh_host: gitlab.company.example
    ssh_port: 22
```

`api_url` is used for REST calls. `web_url` is shown to users and used for web links. `git_hosts` is used by the credential helper to decide whether GCM should answer a Git credential request.

---

## Profile Accounts

Profiles can keep provider-specific account metadata under `providers`.

```yaml
name: work
git:
  user:
    name: Jane Doe
    email: jane@company.example

providers:
  github:
    username: jane-gh
    auth_method: pat
    upload_keys: true
  gitlab:
    username: jane-gl
    auth_method: pat
    upload_keys: true
```

The legacy `github:` profile block is still supported and is mapped into the GitHub provider path where needed.

---

## Token Storage

Provider-aware token keys include profile, provider, host, and optional account. This prevents a `work` GitHub token and a `work` GitLab token from colliding.

The structured token payload can hold PATs today and OAuth refresh-token metadata later:

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "token_type": "bearer",
  "auth_method": "oauth_device",
  "scopes": ["api"],
  "expires_at": "2026-05-24T12:00:00Z"
}
```

Backward compatibility: existing GitHub tokens stored under the old profile-only key still load through the GitHub provider path.

---

## Credential Helper Flow

1. Git invokes `gcm credential-helper get` with `protocol`, `host`, and optional `path`.
2. GCM resolves `host` through the provider registry.
3. GCM reads the active profile.
4. GCM loads the provider-aware token for profile/provider/host.
5. GCM returns provider-specific Git credentials.

Provider username strategy:

| Provider | PAT Username | OAuth Username |
| -------- | ------------ | -------------- |
| GitHub | configured GitHub username, fallback profile name | configured GitHub username |
| GitLab | configured GitLab username, fallback `oauth2` | `oauth2` |
| Bitbucket planned | account/email depending on auth method | `x-token-auth` |

---

## Implementation Tradeoff

The provider-aware token APIs are currently implemented on the existing `internal/github.TokenStore` concrete type to preserve compatibility with the current container and tests. The public behavior is provider-aware, but the package name is historical.

Recommended follow-up when Bitbucket work starts: move the token store concrete implementation to a neutral package such as `internal/tokenstore`, then keep a type alias in `internal/github` for one release if API compatibility is needed.
