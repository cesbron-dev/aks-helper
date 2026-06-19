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

**One-shot install (binary + agent skill):**

```sh
./scripts/install.sh                  # macOS / Linux — global, all agents
./scripts/install.sh --agent copilot  # only one agent (claude|copilot|agents)
./scripts/install.sh --scope local    # into the current project instead of ~/
pwsh scripts/install.ps1              # Windows
```

This builds `aks-helper` onto your PATH and installs the
[agent skill](#for-coding-agents) in the open `SKILL.md` format, which Claude
Code and GitHub Copilot both load:

| Scope             | Claude Code        | GitHub Copilot      | Neutral            |
| ----------------- | ------------------ | ------------------- | ------------------ |
| global (per-user) | `~/.claude/skills` | `~/.copilot/skills` | `~/.agents/skills` |
| local (project)   | `.claude/skills`   | `.github/skills`    | `.agents/skills`   |

Add `--skill-only` / `-SkillOnly` to install just the skill. You can also simply
ask your coding agent: *"install the skill from this repo"* — it will run this
for you (see [`AGENTS.md`](AGENTS.md)).

### Switching clusters (per terminal)

Each shell keeps its **own** `KUBECONFIG`, so different terminals can work on
different clusters at the same time. A child process can't change its parent
shell's environment, so pick one of:

#### Subshell — no setup, most reliable on Windows

`aks-helper shell <name>` opens a subshell scoped to a cluster. It works in every
shell (PowerShell, git-bash, cmd, bash, zsh, fish) with no profile changes and no
`eval`:

```sh
aks-helper shell prod      # opens a shell pinned to 'prod'
kubectl get nodes          # …talks to prod
exit                       # back to your previous context
```

Open a second terminal with `aks-helper shell staging` to work on both at once.

#### In-place shell function

To switch the current shell in place (no nesting), load the wrapper function once
in your profile — it sets `KUBECONFIG` to the per-cluster file for that shell
only:

```sh
eval "$(aks-helper shell-init bash)"                               # ~/.bashrc
eval "$(aks-helper shell-init zsh)"                                # ~/.zshrc
aks-helper shell-init fish | source                               # fish
aks-helper shell-init powershell | Out-String | Invoke-Expression # $PROFILE
```

Then `aks use my-cluster` updates `KUBECONFIG` in that terminal. Supported:
`bash`, `zsh`, `fish`, `powershell`, `pwsh`. On Windows, if the function path is
awkward (calling the binary directly bypasses it), prefer `aks-helper shell`.

Let aks-helper write the block into your startup file for you (idempotent — it
updates the block in place on re-run):

```sh
aks-helper shell-init bash --install                  # writes ~/.bashrc
aks-helper shell-init zsh  --install                  # writes ~/.zshrc
aks-helper shell-init fish --install                  # writes ~/.config/fish/config.fish
aks-helper shell-init powershell --install --file $PROFILE
```

## Interactive UI

Prefer something less austere? `aks-helper ui` (alias `tui`) opens a k9s-style
terminal UI listing your clusters, with a live state icon (● running, ○ stopped,
✖ gone) and Kubernetes version pulled from Azure, live filtering, and one-key
actions:

```sh
aks-helper ui
```

| Key         | Action                                              |
| ----------- | --------------------------------------------------- |
| `↑`/`↓`     | move                                                |
| `enter`/`k` | launch **k9s** on the highlighted cluster           |
| `s`         | open a subshell scoped to the highlighted cluster   |
| `d`         | delete the cluster (with confirmation)              |
| `i`         | import from Azure (runs `sync`)                     |
| `c`         | check/clean stale clusters (runs `cleanup`)         |
| `/`         | filter by name / subscription / resource group      |
| `r`         | reload                                              |
| `q`         | quit                                                |

## Usage

```sh
az login                 # once, the tool reuses your az session

aks sync                 # fuzzy-pick subscriptions, then clusters, to import
aks use                  # fuzzy-pick a cluster for this shell (needs shell-init)
aks-helper shell prod    # …or open a subshell pinned to a cluster (no setup)
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
| `ui`           | Browse and manage clusters in an interactive k9s-style TUI.        |
| `sync`         | Import AKS credentials from Azure (interactive or via flags).     |
| `use [name]`   | Select a cluster for the current shell (via the wrapper function). |
| `shell [name]` | Open a subshell scoped to a cluster (per-terminal, no setup).      |
| `list`         | List stored clusters (`--plain`, `--json`).                       |
| `current`      | Show the cluster active in this shell (`--quiet` for prompts).     |
| `cleanup`      | Check stored clusters vs Azure; `--prune` deleted, `--refresh` stale. |
| `exec`         | Run a command against a cluster without touching the shell env.    |
| `path [name]`  | Print a cluster's kubeconfig path (for `--kubeconfig` flags).      |
| `remove`       | Delete stored cluster(s).                                          |
| `shell-init`   | Print (or `--install`) the bash/zsh/fish/powershell function.     |

### Housekeeping

`aks-helper cleanup` checks every stored cluster against Azure and flags those
that are **gone** (deleted) or **stale** — a cluster deleted and recreated under
the same name gets a new API server, so the stored credentials no longer work.

```sh
aks-helper cleanup            # report only
aks-helper cleanup --prune    # remove clusters deleted in Azure
aks-helper cleanup --refresh  # re-fetch credentials for recreated/rotated ones
aks-helper cleanup --prune --refresh --yes
```

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
