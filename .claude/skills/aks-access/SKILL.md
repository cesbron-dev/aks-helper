---
name: aks-access
description: Access Azure Kubernetes Service (AKS) clusters non-interactively using the aks-helper CLI. Use when the user asks to inspect, query, debug or operate an AKS / Kubernetes cluster — running kubectl/helm against a cluster, listing pods/nodes/deployments, checking logs, importing cluster credentials from Azure, or switching between AKS clusters. Triggers include "aks", "kubectl on the cluster", "k8s cluster", "kubeconfig", "az aks", "kubelogin".
---

# Accessing AKS clusters with aks-helper

The full, agent-agnostic guidance is the single source of truth in
[`AGENTS.md`](../../../AGENTS.md) at the repository root — read it for the
complete workflow, guidance and command reference. The essentials:

- You have **no TTY**: never trigger the interactive fuzzy finder, use the
  flag-driven paths only.
- **Run cluster commands with `exec`, not `use`** (a child process can't change
  the parent shell's `KUBECONFIG`):
  - `aks-helper exec --cluster <name> -- kubectl get nodes`
  - `aks-helper exec -- kubectl get pods -A` (uses the current selection)
- **Discover first**: `aks-helper list --plain` / `--json`,
  `aks-helper current --quiet`.
- **Import only if needed** (requires `az login` + `kubelogin`):
  `aks-helper sync --subscription <sub> --cluster <cluster>` — defaults to the
  non-interactive `azurecli` login; do not use `devicecode`/`interactive`.
- Prefer **read-only** operations; `delete`/`apply`/`scale`/`rollout`/
  `helm upgrade` are mutating and need explicit authorization.
- On `az login` / `AADSTS` / "not logged in" errors, ask the user to run
  `az login`.
