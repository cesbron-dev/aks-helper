# AGENTS.md

Guidance for AI coding agents — Claude Code, GitHub Copilot, Google Antigravity,
Cursor, Codex, Gemini CLI and any tool that reads the cross-agent
[`AGENTS.md`](https://agents.md) convention — working in this repository **or**
using the `aks-helper` tool it builds.

This file is the single source of truth. The agent-specific entry points just
point here:

- Claude Code: `.claude/skills/aks-access/SKILL.md`
- GitHub Copilot: `.github/copilot-instructions.md`

## Project

`aks-helper` is a Go CLI that manages connections to Azure Kubernetes Service
(AKS) clusters. Layout: `main.go`, commands in `cmd/`, internals in
`internal/{azure,config,kubeconfig,ui}`.

### Build, test, lint

```sh
go build ./...      # or: make build
go test ./...       # or: make test
gofmt -l .          # must print nothing
go vet ./...
```

Keep `gofmt` and `go vet` clean and add tests for new behaviour. Never commit
secrets or real kubeconfigs.

## Accessing AKS clusters with aks-helper

`aks-helper` manages AKS kubeconfigs under `~/.kube/aks` and runs commands
against a chosen cluster. As an agent you have **no TTY**, so never use the
interactive fuzzy finder — always use the flag-driven, non-interactive paths
below.

### Golden rule: use `exec`, not `use`

`aks use` sets `KUBECONFIG` in the *parent shell* via a shell function. That
mechanism does not work for you, because each command you run is a fresh
process. Instead, run every cluster command through `exec`, which sets
`KUBECONFIG` only for that child process:

```sh
aks-helper exec --cluster <name> -- kubectl get nodes
aks-helper exec --cluster <name> -- kubectl get pods -A
aks-helper exec --cluster <name> -- helm list -A
```

If a cluster is already selected, omit `--cluster` to use the current one:

```sh
aks-helper exec -- kubectl get nodes
```

### Workflow

1. **Check what is already available** before importing anything:

   ```sh
   aks-helper list --plain          # one cluster name per line; empty = none
   aks-helper list --json           # full metadata (subscription, rg, login)
   aks-helper current --quiet       # currently selected cluster, if any
   ```

2. **Import a cluster** only if the one you need is not listed. This requires a
   valid Azure session (`az login` already done by the user) and `kubelogin`:

   ```sh
   # By subscription + cluster name (both substring matches):
   aks-helper sync --subscription "<sub name or id>" --cluster "<cluster>"

   # All clusters in a subscription:
   aks-helper sync --subscription "<sub>" --all
   ```

   `sync` (alias `get-cred`) defaults to `--login azurecli`, which reuses the
   existing `az` session and never prompts. Do **not** switch to
   `devicecode`/`interactive` login modes — they block waiting for a human.

3. **Run cluster commands** with `exec` as shown above.

### Guidance

- Prefer **read-only** operations (`get`, `describe`, `logs`, `top`) unless the
  user explicitly asked you to change cluster state. Treat `delete`, `apply`,
  `scale`, `drain`, `rollout` and `helm upgrade/install` as mutating actions
  that need clear authorization.
- If `aks-helper list` is empty and you cannot import (no Azure login,
  `kubelogin` missing, or you lack the subscription/cluster name), stop and ask
  the user rather than guessing.
- Resolve a cluster's kubeconfig path with `aks-helper path <name>` when a tool
  needs an explicit `--kubeconfig` flag.
- Errors mentioning `az login`, `AADSTS`, or `not logged in` mean the Azure
  session expired — tell the user to run `az login`; you cannot complete it
  non-interactively.
- The tool never modifies clusters in Azure; `remove` only forgets a stored
  kubeconfig locally.

### Quick reference

| Goal                            | Command                                          |
| ------------------------------- | ------------------------------------------------ |
| List stored clusters            | `aks-helper list --plain`                        |
| Cluster metadata as JSON        | `aks-helper list --json`                         |
| Currently selected cluster      | `aks-helper current --quiet`                     |
| Import one cluster              | `aks-helper sync -s <sub> -c <cluster>`          |
| Import all in a subscription    | `aks-helper sync -s <sub> --all`                 |
| Run kubectl against a cluster   | `aks-helper exec -c <name> -- kubectl get nodes` |
| Run against the current cluster | `aks-helper exec -- kubectl get pods -A`         |
| Get a cluster's kubeconfig path | `aks-helper path <name>`                         |
