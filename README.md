# aks-helper

A small CLI to manage your connections to **Azure Kubernetes Service (AKS)**
clusters.

It wraps the workflow you probably already script by hand — `az`, `kubelogin`
and `fzf` — into a single static Go binary:

- Lists your Azure **subscriptions** and **AKS clusters** with a built-in fuzzy
  finder (no external `fzf` required).
- Imports each cluster's credentials and stores a standalone kubeconfig under
  `~/.kube/aks/<name>.yaml`, converting Azure AD auth with `kubelogin`.
- Switches the active cluster for your shell by pointing `KUBECONFIG` at the
  selected file, so `kubectl`, `k9s`, `helm`, … just work.
- Has fully **non-interactive** flags so scripts and coding agents can use it
  too.

## Install

Requires Go 1.25+, the [Azure CLI](https://learn.microsoft.com/cli/azure/)
(`az`) and [`kubelogin`](https://azure.github.io/kubelogin/) on your `PATH`.

**Prebuilt binaries** for Linux, macOS and Windows (amd64/arm64, plus linux
armv7) are attached to each [GitHub release](../../releases). Download the
archive for your platform, extract `aks-helper` and put it on your `PATH`.

**From source:**

```sh
git clone https://github.com/cesbron-dev/aks-helper
cd aks-helper
make install          # builds to $(go env GOPATH)/bin/aks-helper
# or: go install github.com/cesbron-dev/aks-helper@latest
```

### Shell integration (one time)

A child process can't change its parent shell's environment, so `use` prints an
`export` that a tiny shell function evaluates for you. Add to your shell rc:

```sh
# ~/.bashrc
eval "$(aks-helper shell-init bash)"

# ~/.zshrc
eval "$(aks-helper shell-init zsh)"

# ~/.config/fish/config.fish
aks-helper shell-init fish | source

# PowerShell / pwsh  ($PROFILE)
aks-helper shell-init powershell | Out-String | Invoke-Expression
```

This defines an `aks` function that wraps the binary; every command works the
same, but `aks use` also updates `KUBECONFIG` in your current shell. The
`bash`, `zsh`, `fish`, `powershell` and `pwsh` shells are supported.

## Usage

```sh
az login                 # once, the tool reuses your az session

aks sync                 # fuzzy-pick subscriptions, then clusters, to import
aks use                  # fuzzy-pick a cluster for this shell
kubectl get nodes        # talks to the selected cluster
k9s                      # …so does everything else

aks list                 # show stored clusters (* = current)
aks current              # show the active cluster
aks use prod             # switch by name
aks remove old-cluster   # forget a stored cluster (Azure is untouched)
```

### Commands

| Command       | Description                                                        |
| ------------- | ----------------------------------------------------------------- |
| `sync`        | Import AKS credentials from Azure (interactive or via flags).      |
| `use [name]`  | Select a cluster for the current shell (sets `KUBECONFIG`).        |
| `list`        | List stored clusters (`--plain`, `--json`).                        |
| `current`     | Show the active cluster (`--quiet` for prompts).                   |
| `exec`        | Run a command against a cluster without touching the shell env.    |
| `path [name]` | Print a cluster's kubeconfig path (for `--kubeconfig` flags).      |
| `remove`      | Delete stored cluster(s).                                          |
| `shell-init`  | Print the bash/zsh/fish integration function.                     |

Most commands have aliases: `use` → `select`, `switch`; `sync` → `get-cred`,
`get-credentials`, `creds`, `import`; `list` → `ls`; `current` → `cur`;
`remove` → `rm`, `delete`. The shell integration also defines the classic
hyphenated shortcuts **`aks-select`** (= `aks use`) and **`aks-get-cred`**
(= `aks sync`).

### Non-interactive use (CI & coding agents)

Every interactive step has a flag-driven equivalent:

```sh
# Import without prompting
aks sync --subscription "Prod" --all
aks sync -s 00000000-0000-0000-0000-000000000000 -c my-cluster

# Run kubectl against a specific cluster, no shell changes
aks exec --cluster prod -- kubectl get pods -A
aks exec -- kubectl get nodes          # uses the current selection

aks list --plain                       # one name per line
aks list --json                        # full metadata
```

By default clusters are converted with `kubelogin --login azurecli`, which
reuses the existing `az login` session and never prompts — ideal for
automation. Use `--login devicecode` for headless interactive login, or
`--admin` for certificate-based local admin credentials.

## Layout on disk

```
~/.kube/aks/
├── <name>.yaml     # one standalone kubeconfig per cluster
├── index.json      # subscription / resource-group metadata
└── .current        # name of the currently selected cluster
```

Set `AKS_HELPER_DIR` to use a different directory.

## For coding agents

Guidance for AI coding agents is declared agent-agnostically in
[`AGENTS.md`](AGENTS.md) (the [agents.md](https://agents.md) convention, read by
GitHub Copilot, Google Antigravity, Cursor, Codex, Gemini CLI, …). Tool-specific
entry points point at it:

- GitHub Copilot — [`.github/copilot-instructions.md`](.github/copilot-instructions.md)
- Claude Code — [`.claude/skills/aks-access`](.claude/skills/aks-access/SKILL.md)

They all teach agents to reach AKS clusters non-interactively via
`aks-helper exec`.
