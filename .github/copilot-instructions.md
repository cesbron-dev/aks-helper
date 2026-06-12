# GitHub Copilot instructions

The full, agent-agnostic guidance for this repository lives in
[`AGENTS.md`](../AGENTS.md). Read it before working here or before accessing an
AKS cluster — the essentials are repeated below.

## Accessing AKS clusters

This repo provides `aks-helper`, a CLI for AKS cluster connections. As an agent
you have no TTY, so use the non-interactive paths only:

- **Run cluster commands with `exec`, never `use`** (a child process can't change
  the parent shell's `KUBECONFIG`):
  - `aks-helper exec --cluster <name> -- kubectl get nodes`
  - `aks-helper exec -- kubectl get pods -A` (uses the current selection)
- **Discover** what is available first: `aks-helper list --plain` /
  `aks-helper list --json` / `aks-helper current --quiet`.
- **Import** only if needed (requires `az login` + `kubelogin`):
  `aks-helper sync --subscription <sub> --cluster <cluster>` (defaults to the
  non-interactive `azurecli` login — do not use `devicecode`/`interactive`).
- Prefer **read-only** kubectl/helm operations; treat `delete`/`apply`/`scale`/
  `rollout`/`helm upgrade` as mutating actions needing explicit authorization.
- On `az login` / `AADSTS` / "not logged in" errors, ask the user to run
  `az login`; you cannot complete it non-interactively.

## Installing the skill globally

If the user asks to "install the skill from this repo", run it for them rather
than only explaining: `./scripts/install.sh --skill-only` (or
`pwsh scripts/install.ps1 -SkillOnly` on Windows), or the full
`./scripts/install.sh` to also build the binary. See `AGENTS.md` for the manual
fallback. Note that a *global* skill is a Claude Code concept; for Copilot, the
guidance is the repo-scoped `AGENTS.md` / this file.

## Working on the codebase

Go project. Keep it building and clean:

```sh
go build ./... && go test ./... && gofmt -l . && go vet ./...
```

See [`AGENTS.md`](../AGENTS.md) for the full reference.
