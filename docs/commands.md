# Commands Reference

Complete reference for every GCM command, subcommand, flag, and alias.

---

## Global Behavior

Every command supports these global flags:

| Flag             | Description                         |
| ---------------- | ----------------------------------- |
| `-h, --help`     | Help for the command                |
| `-v, --verbose`  | Enable verbose (debug) output       |
| `-q, --quiet`    | Suppress non-essential output       |
| `--no-color`     | Disable colored output              |

```bash
gcm --help
gcm profile --help
gcm ssh generate --help
gcm -v use work         # see debug logs during activation
gcm --no-color profile list   # plain text, no ANSI colors
```

GCM silences Cobra's default usage/error output — errors are displayed as clean messages without the full help text.

---

## `gcm setup`

Guided first-time setup wizard. Walks you through the complete GCM configuration.

```bash
gcm setup
gcm quickstart   # alias
```

**Aliases:** `gcm quickstart`

**What it does:**
1. Shell integration (auto-switching, prompt)
2. Creating your first profile (name, email)
3. SSH key generation
4. GPG signing (optional)
5. GitHub authentication
6. Activating your profile

Perfect for first-time users. Run this once and you're fully set up.

---

## `gcm status`

Show a quick overview of your GCM setup.

```bash
gcm status
gcm st           # alias
```

**Aliases:** `gcm st`

Shows: active profile, all profiles summary, GitHub auth status, SSH keys, and any issues that need attention.

---

## `gcm init`

Set up shell integration and credential helper.

```bash
gcm init
```

**What it does:**
1. Detects your shell (`bash`, `zsh`, `fish`, `powershell`)
2. Appends a marked hook block to your shell config file
3. Registers GCM as git's credential helper for github.com
4. Reports the shell and config file path

**Output:** The config file path. Restart your shell afterward.

**Notes:**
- Fails if integration is already installed. Remove the old block first (see [Shell Integration](shell-integration.md#uninstalling)).
- If auto-detection fails, it suggests specifying manually.
- The credential helper makes git authentication immune to external credential store changes (VS Code logout, Keychain edits, etc.)

**See also:** [Shell Integration](shell-integration.md)

---

## `gcm profile`

Manage Git profiles. Running `gcm profile` with no subcommand defaults to `gcm profile list`.

**Aliases:** `gcm p`

### `gcm profile create <name>`

Create a new Git profile.

```bash
gcm profile create work -i
gcm profile create work --name "Jane Doe" --email "jane@acme.example"
gcm profile create work --name "Jane" --email "jane@acme.example" --ssh-key ~/.ssh/id_ed25519_work --editor code
```

| Flag               | Short | Default | Description                        |
| ------------------ | ----- | ------- | ---------------------------------- |
| `--interactive`    | `-i`  | false   | Launch the 4-step interactive wizard |
| `--name`           | `-n`  |         | Git `user.name`                    |
| `--email`          | `-e`  |         | Git `user.email`                   |
| `--editor`         |       |         | Git `core.editor`                  |
| `--ssh-key`        |       |         | Path to an existing SSH private key |
| `--from-template`  | `-t`  |         | Apply settings from a template     |

**Interactive wizard steps:**
1. **Basic info** — name, email, editor
2. **SSH key** — generate new key (Ed25519/RSA/ECDSA) or skip
3. **GPG signing** — generate new GPG key or skip
4. **GitHub** — set username or skip

**Validation:**
- `--name` and `--email` are required unless `--interactive` is set
- Profile name must not contain `/`, `\`, `..`, or control characters
- Profile name must not be empty

### `gcm profile list`

List all profiles with status, email, signing, and last used.

```bash
gcm profile list
gcm profile ls        # alias
gcm profile           # also runs list
```

**Output columns:** Profile, Status (`active`, `default`, `active default`), Email, Signing (✓/✗), Last Used.

### `gcm profile show <name>`

Display detailed information about a profile.

```bash
gcm profile show work
```

**Shows:** Git user/email/editor, SSH key path/type/fingerprint, GPG key ID, GitHub username, metadata (created, usage count).

### `gcm profile edit <name>`

Edit a profile's name or email.

```bash
gcm profile edit work -n "Jane Smith"
gcm profile edit work -e "jane.smith@acme.example"
gcm profile edit work -n "Jane Smith" -e "jane.smith@acme.example"
```

| Flag      | Short | Description         |
| --------- | ----- | ------------------- |
| `--name`  | `-n`  | Update `user.name`  |
| `--email` | `-e`  | Update `user.email` |

### `gcm profile delete <name>`

Delete a profile. Prompts for confirmation unless `--yes` is set.

```bash
gcm profile delete old-work
gcm profile delete old-work -y     # skip confirmation
gcm profile rm old-work            # alias
```

| Flag    | Short | Default | Description          |
| ------- | ----- | ------- | -------------------- |
| `--yes` | `-y`  | false   | Skip confirmation    |

**Aliases:** `rm`

### `gcm profile export <name>`

Export a profile as YAML to stdout.

```bash
gcm profile export work > work-profile.yaml
```

### `gcm profile import <file>`

Import a profile from a YAML file.

```bash
gcm profile import work-profile.yaml
```

### `gcm profile diff <profile1> <profile2>`

Compare two profiles side by side.

```bash
gcm profile diff work personal
```

**Compares:** Name, Email, SSH key path. Only fields that differ are shown.

---

## `gcm use <name>`

Activate a profile.

```bash
gcm use work                 # smart: session (git repo) or local (elsewhere)
gcm use work --global        # set as machine-wide default
gcm use work --local         # pin to current directory
gcm use work --dry-run       # preview changes, apply nothing
```

| Flag        | Short | Default | Description                              |
| ----------- | ----- | ------- | ---------------------------------------- |
| `--global`  | `-g`  | false   | Persist as the default profile           |
| `--local`   | `-l`  | false   | Write `.gcm-profile` in current directory |
| `--dry-run` |       | false   | Show what would change without applying  |

**What it does (non-dry-run):**
1. Loads the profile from `~/.gcm/profiles/<name>.yaml`
2. Writes Git config (`user.name`, `user.email`, `core.editor`, `commit.gpgsign`, `user.signingkey`)
3. Loads the SSH key into the ssh-agent via `ssh-add` (if configured and key file exists)
4. **Pins `credential.https://github.com.username`** so git only uses credentials belonging to this profile
5. If GCM is the credential helper: git will ask GCM dynamically for credentials (no system keychain involved)
6. If GCM is NOT the credential helper (legacy): clears old credentials via `git credential reject` and stores the new profile's token via `git credential approve`
7. Increments the profile's `usage_count` and `last_used`
8. Logs the activation to the audit log
9. Verifies the GitHub token validity (best-effort, warns if expired)

> **Credential Isolation:** After switching, git clone/push/pull will only work with the active profile's GitHub account. Other profiles' credentials cannot bleed through.

**Scopes:**
- **Session** (default, in a git repo) — writes a `.git/gcm-session` marker file for reliable detection, plus local git config
- **Local** (default, outside a git repo) — creates a `.gcm-profile` file in the current directory
- **Global** (`--global`) — saves as `default_profile` in `config.yaml`, persists across shell restarts; also clears any local/session overrides in the current directory

**Smart scope fallback (no explicit flag):**
- Inside a git repo → session scope
- Outside a git repo → local scope (writes `.gcm-profile`)
- This means `gcm use <name>` **always works** regardless of whether you're in a git repository.

---

## `gcm current`

Show the currently active profile.

```bash
gcm current              # full details (user, scope, SSH, GPG)
gcm current --short      # just the profile name (for scripts/prompts)
gcm current --short --hide-default  # nothing if default is active (shell prompts)
```

| Flag             | Default | Description                                              |
| ---------------- | ------- | -------------------------------------------------------- |
| `--short`        | false   | Print only the profile name                              |
| `--hide-default` | false   | Output nothing if the active profile is the default one  |

> **Shell prompt tip:** Use `gcm current --short --hide-default` in your prompt so it only shows a profile indicator when you've explicitly switched away from your default.

**Detection order:**
1. `.git/gcm-session` marker file (written by `gcm use`) → `session`
2. `git config --local user.email` matched against known profiles → `session` (fallback)
3. `.gcm-profile` in the current directory → `local`
4. `default_profile` in `config.yaml` → `global`

> **Why session first?** `gcm use <name>` is an explicit user action and should always take effect immediately. The `.gcm-profile` file is a directory-level pin that serves as a default for that project, but can be overridden by an explicit switch.

---

## `gcm refresh`

Re-evaluate the current directory and activate the appropriate profile.

```bash
gcm refresh              # normal output
gcm refresh --silent     # suppress all output (used by shell hooks)
```

| Flag       | Default | Description              |
| ---------- | ------- | ------------------------ |
| `--silent` | false   | Suppress all output      |

This is the command that shell hooks call on every `cd`.

---

## `gcm ssh`

Manage SSH keys. Running `gcm ssh` with no subcommand defaults to `gcm ssh list`.

### `gcm ssh generate <profile>`

Generate a new SSH key pair and associate it with a profile.

```bash
gcm ssh generate work                          # Ed25519, no passphrase
gcm ssh generate work -t rsa -b 4096           # RSA 4096
gcm ssh generate work -t ecdsa                 # ECDSA P-256
gcm ssh generate work -c "work laptop"         # custom comment
gcm ssh generate work -p "my-passphrase"       # with passphrase
```

| Flag             | Short | Default    | Description                          |
| ---------------- | ----- | ---------- | ------------------------------------ |
| `--type`         | `-t`  | `ed25519`  | Key type: `ed25519`, `rsa`, `ecdsa`  |
| `--bits`         | `-b`  | `4096`     | Key size (RSA only: 2048/3072/4096)  |
| `--comment`      | `-c`  |            | Comment baked into the key           |
| `--passphrase`   | `-p`  |            | Key passphrase (encrypted at rest)   |

**What it does:**
1. Generates the key pair using Go's native `crypto` library (no subprocess)
2. Writes private key with `0600` permissions, public key with `0644`
3. If a passphrase is given, encrypts the private key at rest using OpenSSH native format (bcrypt-KDF + AES-256-CTR)
4. Updates the profile's SSH configuration automatically
5. Prints the public key for easy copying
6. If a GitHub token is stored for this profile, offers to upload the key to GitHub automatically

### `gcm ssh list`

List SSH keys across all profiles.

```bash
gcm ssh list
gcm ssh ls          # alias
gcm ssh             # also runs list
```

**Output columns:** Key path, Type, Fingerprint, Agent (✓/✗).

### `gcm ssh test <profile>`

Test the SSH connection to GitHub using the profile's key.

```bash
gcm ssh test work
```

Runs `ssh -T git@github.com` with the profile's key.

### `gcm ssh copy <profile>`

Print the public key to stdout.

```bash
gcm ssh copy work
gcm ssh copy work | pbcopy    # macOS: copy to clipboard
```

### `gcm ssh upload <profile>`

Upload the profile's SSH public key to GitHub. Checks for duplicates before uploading.

```bash
gcm ssh upload work
gcm ssh upload work --force    # skip duplicate check
```

| Flag      | Short | Default | Description            |
| --------- | ----- | ------- | ---------------------- |
| `--force` | `-f`  | `false` | Skip duplicate check   |

**What it does:**
1. Loads the stored GitHub token for the profile
2. Lists existing SSH keys on GitHub and compares by key material
3. If the key already exists, reports "already uploaded" and exits
4. Otherwise, uploads with title `gcm-<profile>-<type>`

---

## `gcm gpg`

Manage GPG keys. Running `gcm gpg` with no subcommand defaults to `gcm gpg list`.

### `gcm gpg generate <profile>`

Generate a new GPG key for a profile (uses the profile's name and email).

```bash
gcm gpg generate work
```

The profile must already have `user.name` and `user.email` set.

After generation, the profile is automatically updated with the GPG key ID and `commit.gpgsign = true`.
If a GitHub token is stored for this profile, GCM will offer to upload the GPG public key to GitHub so commits show as "Verified".

### `gcm gpg list`

List GPG keys known to the system.

```bash
gcm gpg list
gcm gpg ls          # alias
gcm gpg             # also runs list
```

**Output columns:** Key ID, Name, Email, Created, Trust.

If GPG is not installed, shows a warning with installation instructions.

### `gcm gpg sign enable <profile>`

Enable commit signing for a profile.

```bash
gcm gpg sign enable work
```

Requires a GPG key to be configured on the profile. Sets `commit.gpgsign = true` and `user.signingkey = <key-id>`.

### `gcm gpg sign disable <profile>`

Disable commit signing for a profile.

```bash
gcm gpg sign disable work
```

### `gcm gpg test <profile>`

Test GPG signing capability.

```bash
gcm gpg test work
```

Performs a test signature using the profile's GPG key ID.

### `gcm gpg upload <profile>`

Upload the profile's GPG public key to GitHub for commit verification. Checks for duplicates before uploading.

```bash
gcm gpg upload work
gcm gpg upload work --force    # skip duplicate check
```

| Flag      | Short | Default | Description            |
| --------- | ----- | ------- | ---------------------- |
| `--force` | `-f`  | `false` | Skip duplicate check   |

**What it does:**
1. Loads the stored GitHub token for the profile
2. Lists existing GPG keys on GitHub and compares by key ID
3. If the key already exists, reports "already uploaded" and exits
4. Otherwise, exports the armored public key and uploads to GitHub

---

## `gcm github`

Manage GitHub integration. Supports multiple authentication methods.

**Aliases:** `gcm gh`

### `gcm github login <profile>`

Authenticate with a Personal Access Token (PAT). Useful for CI/CD, headless environments, or fine-grained token control.

```bash
gcm github login work                          # interactive (masked input)
echo "$GH_TOKEN" | gcm github login work       # piped from env/script
```

**Requirements:** Generate a token at https://github.com/settings/tokens with scopes: `repo`, `admin:public_key`, `admin:gpg_key`.

**What it does:**
1. Reads token from interactive input or stdin pipe
2. Verifies the token against the GitHub API
3. Encrypts and stores the token
4. Updates the profile with your GitHub username
5. **If this is the active profile**, git credentials are stored for clone/push/pull

> **Note:** Git credentials are only updated if the profile being logged in is the currently active one. Logging into a non-active profile saves the token but does not affect git operations until you switch to that profile with `gcm use`.

### `gcm github login-oauth <profile>`

Authenticate with GitHub using the OAuth device flow (interactive, browser-based).

```bash
gcm github login-oauth work
```

**Flow:**
1. Initiates the OAuth device flow
2. Displays a URL and a user code
3. You open the URL in your browser and enter the code
4. GCM polls until you approve (up to 15 minutes)
5. Token is encrypted and stored in `~/.gcm/tokens/`
6. Profile is updated with your GitHub username
7. **If this is the active profile**, git credentials are stored for clone/push/pull

### `gcm github login-gh <profile>`

Import authentication from the GitHub CLI (`gh`). Requires `gh` to be installed and authenticated.

```bash
gcm github login-gh work
```

**What it does:**
1. Runs `gh auth token` to retrieve your existing token
2. Verifies the token against the GitHub API
3. Encrypts and stores it in GCM
4. Updates the profile with your GitHub username

**Note:** Requires the GitHub CLI (https://cli.github.com) to be installed and logged in (`gh auth login`).

### `gcm github status`

Show authentication status for all profiles.

```bash
gcm github status
```

**Output columns:** Profile, Status (`authenticated`, `not authenticated`, `token expired`), Username, Method.

### `gcm github logout <profile>`

Remove the stored GitHub token for a profile and clear git credentials.

```bash
gcm github logout work                         # remove token + clear git credentials
gcm github logout work --clear-credentials=false  # only remove GCM token
```

| Flag                  | Short | Default | Description                                                 |
| --------------------- | ----- | ------- | ----------------------------------------------------------- |
| `--clear-credentials` |       | true    | Also clear cached git credentials via `git credential reject` |
| `--force`             | `-f`  | false   | Skip confirmation when logging out a non-active profile     |

**What it does:**
1. If logging out a non-active profile, prompts for confirmation (skip with `--force`)
2. Deletes the stored token from the keychain/encrypted file
3. (If `--clear-credentials` AND the profile is currently active) Clears git credentials for GitHub from the system credential store (macOS Keychain, Windows Credential Manager, Linux secret-service)

> **Note:** Git credentials are only cleared if the profile being logged out is the currently active one. This prevents accidentally breaking the active profile's authentication.

### `gcm github verify <profile>`

Verify that the stored token is still valid.

```bash
gcm github verify work
```

### `gcm github user <profile>`

Show GitHub user information for the authenticated profile.

```bash
gcm github user work
```

**Shows:** Login, Name, Email, Company, Location, Public repos, Profile URL.

---

## `gcm template`

Manage configuration templates. Running `gcm template` with no subcommand defaults to `gcm template list`.

**Aliases:** `gcm tpl`

### `gcm template create <name>`

Create a new configuration template. Templates store reusable git settings (editor, pull strategy, signing, aliases).

```bash
gcm template create company-standard -i
gcm template create company-standard --from-profile work
gcm template create minimal --editor "code --wait" --rebase true
gcm template create team --editor vim --gpg-sign true --alias "co=checkout,st=status"
```

| Flag              | Short | Default | Description                              |
| ----------------- | ----- | ------- | ---------------------------------------- |
| `--interactive`   | `-i`  | false   | Interactive creation wizard               |
| `--description`   | `-d`  |         | Template description                     |
| `--from-profile`  |       |         | Extract settings from an existing profile |
| `--editor`        |       |         | Git editor (e.g. `vim`, `code --wait`)   |
| `--rebase`        |       |         | Pull rebase strategy (true/false/merges) |
| `--gpg-sign`      |       |         | Enable commit signing (true/false)       |
| `--alias`         |       |         | Git aliases (format: key=value)          |

### `gcm template list`

List all templates.

```bash
gcm template list
gcm template ls         # alias
gcm template            # also runs list
```

**Output columns:** Template name, Description, Version, Created.

### `gcm template show <name>`

Show template details (prints the full YAML).

```bash
gcm template show company-standard
```

### `gcm template import <file>`

Import a template from a YAML file.

```bash
gcm template import company-standard.yaml
```

### `gcm template export <name>`

Export a template as YAML to stdout.

```bash
gcm template export company-standard > company.yaml
```

### `gcm template apply <template> <profile>`

Apply a template's git settings to an existing profile. Merges settings (editor, pull strategy, commit signing, push settings, aliases) into the target profile. Identity fields (name, email, SSH, GPG keys) are never modified.

```bash
gcm template apply company-standard work
gcm template apply company-standard work --force   # skip confirmation
```

| Flag      | Short | Default | Description         |
| --------- | ----- | ------- | ------------------- |
| `--force` |       | false   | Skip confirmation   |

### `gcm template delete <name>`

Delete a template.

```bash
gcm template delete company-standard
gcm template rm company-standard     # alias
```

---

## `gcm backup`

Backup and restore GCM data.

### `gcm backup create`

Create a timestamped `.tar.gz` backup of `~/.gcm/`.

```bash
gcm backup create
```

**Includes:** `config.yaml`, all profiles, all templates. Backup is stored in `~/.gcm/backups/` with `0600` permissions.

### `gcm backup list`

List all backups.

```bash
gcm backup list
gcm backup ls          # alias
```

**Output columns:** Date, Size, Path.

### `gcm backup restore <file>`

Restore from a backup file. Prompts for confirmation before overwriting.

```bash
gcm backup restore ~/.gcm/backups/gcm-backup-2026-05-18-143000.tar.gz
```

Restoration is guarded against path-traversal (zip-slip) attacks.

### `gcm backup prune`

Remove old backups, keeping the most recent ones.

```bash
gcm backup prune              # keep last 5 (default)
gcm backup prune --keep 10    # keep last 10
```

| Flag     | Default | Description                      |
| -------- | ------- | -------------------------------- |
| `--keep` | `5`     | Number of recent backups to keep |

---

## `gcm validate`

Validate profile configurations.

```bash
gcm validate            # validate all profiles
gcm validate work       # validate a specific profile
```

**Checks:**
- YAML schema and required fields
- SSH key paths exist and have correct permissions
- GPG key IDs exist in the keyring
- Email format is valid

**Output:** Per-profile results with ✓ (info), ⚠ (warning), ✗ (error) indicators, plus suggestions for fixing issues.

---

## `gcm doctor`

Run a full system health check.

```bash
gcm doctor
```

**Checks:**
- **System:** OS, architecture, Go version
- **Dependencies:** Git, SSH, GPG (version and availability)
- **Services:** SSH agent status
- **Shell:** Shell integration status
- **Credential Helper:** Whether GCM is registered as git's credential helper for github.com
- **Configuration:** Config file location, profile count, template count

---

## `gcm clean`

Clean cache and temporary files.

```bash
gcm clean            # remove cache directory
gcm clean --all      # also remove logs directory
```

| Flag    | Default | Description                                |
| ------- | ------- | ------------------------------------------ |
| `--all` | false   | Also clean the logs directory              |

**Note:** This does *not* remove profiles, tokens, or backups. For a full reset, see [Troubleshooting](troubleshooting.md#full-reset).

---

## `gcm version`

Show version information.

```bash
gcm version              # full output
gcm version --short      # just the version number
```

| Flag      | Default | Description               |
| --------- | ------- | ------------------------- |
| `--short` | false   | Short version output only |

**Full output shows:** Version, Commit hash, Build date, Go version, OS/Architecture.

---

## `gcm credential-helper` (Internal)

Git credential helper implementation. This command is hidden from normal help output and is called by git itself when performing authentication.

```bash
# Not called directly — git invokes it automatically when configured:
# credential.https://github.com.helper = !/path/to/gcm credential-helper
```

**Subcommands:**
- `get` — Reads the active profile's token from GCM's encrypted store and returns it to git in credential protocol format
- `store` — No-op (GCM manages its own token storage)
- `erase` — No-op (use `gcm github logout` to remove tokens)

**Registration:**
- Automatically registered by `gcm init` and `gcm setup`
- Verified by `gcm doctor`

**How it works:**
1. Git calls `gcm credential-helper get` when it needs credentials for github.com
2. GCM determines the active profile via session/local/global scope
3. GCM loads the token from its encrypted store (`~/.gcm/tokens/<profile>`)
4. GCM returns `protocol`, `host`, `username`, and `password` to git

This makes git authentication independent of the system keychain (macOS Keychain, Windows Credential Manager, etc.), so external credential changes (VS Code logout, browser session clear) cannot break git operations.

---

## Exit Codes

| Code | Meaning                                  |
| ---- | ---------------------------------------- |
| `0`  | Success                                  |
| `1`  | General error (invalid args, not found)  |

---

## Environment Variables

| Variable              | Description                                      |
| --------------------- | ------------------------------------------------ |
| `_GCM_PROMPT`         | Shell variable set by precmd hook with active profile name (used in prompt) |
| `SHELL`               | Used by `gcm init` to detect your shell          |
| `HOME`                | Used to locate `~/.gcm/`                         |

---

## Command Aliases

| Full Command      | Alias              |
| ----------------- | ------------------ |
| `gcm setup`       | `gcm quickstart`   |
| `gcm status`      | `gcm st`           |
| `gcm profile`     | `gcm p`            |
| `gcm github`      | `gcm gh`           |
| `gcm template`    | `gcm tpl`          |
| `gcm profile list`| `gcm profile ls`   |
| `gcm profile delete`| `gcm profile rm` |
| `gcm ssh list`    | `gcm ssh ls`       |
| `gcm gpg list`    | `gcm gpg ls`       |
| `gcm backup list` | `gcm backup ls`    |
| `gcm template list`| `gcm template ls` |
| `gcm template delete`| `gcm template rm`|

---

## Command Cheatsheet

```text
Getting Started
  gcm setup                                    First-time setup wizard
  gcm status                                   Overview of current state
  gcm init                                     Install shell hooks
  gcm doctor                                   System health check
  gcm version [--short]                        Show version info

Profiles
  gcm profile create <name> [-i] [--name] [--email] [--ssh-key] [--editor] [--from-template]
  gcm profile list                             List all profiles
  gcm profile show <name>                      Show profile details
  gcm profile edit <name> [-n] [-e]            Edit name/email
  gcm profile delete <name> [-y]               Delete profile
  gcm profile export <name>                    Export as YAML
  gcm profile import <file>                    Import from YAML
  gcm profile diff <a> <b>                     Compare two profiles

Activation
  gcm use <name> [-g | -l | --dry-run]         Activate profile
  gcm current [--short]                         Show active profile
  gcm refresh [--silent]                        Re-evaluate directory

SSH
  gcm ssh generate <name> [-t] [-b] [-c] [-p]  Generate key
  gcm ssh upload <name> [--force]               Upload key to GitHub
  gcm ssh list                                  List all keys
  gcm ssh test <name>                           Test GitHub connection
  gcm ssh copy <name>                           Print public key

GPG
  gcm gpg generate <name>                       Generate GPG key
  gcm gpg upload <name> [--force]               Upload key to GitHub
  gcm gpg list                                  List GPG keys
  gcm gpg sign enable|disable <name>            Toggle signing
  gcm gpg test <name>                           Test signing

GitHub
  gcm github login <name>                       Personal Access Token
  gcm github login-oauth <name>                 OAuth device flow
  gcm github login-gh <name>                    Import from gh CLI
  gcm github status                             Auth status for all profiles
  gcm github logout <name>                      Remove token
  gcm github verify <name>                      Verify token
  gcm github user <name>                        Show user info

Templates
  gcm template create <name> [-i] [--from-profile]  Create template
  gcm template list                             List templates
  gcm template show <name>                      Show details
  gcm template apply <tpl> <profile> [--force]  Apply to profile
  gcm template import <file>                    Import template
  gcm template export <name>                    Export template
  gcm template delete <name>                    Delete template

Backup
  gcm backup create                             Create backup
  gcm backup list                               List backups
  gcm backup restore <file>                     Restore backup
  gcm backup prune [--keep N]                   Remove old backups

Maintenance
  gcm validate [name]                           Validate profiles
  gcm clean [--all]                             Clean cache/logs

Internal (used by git)
  gcm credential-helper get                     Provide credentials to git
  gcm credential-helper store                   No-op (GCM manages own store)
  gcm credential-helper erase                   No-op (GCM manages own store)
```
