package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const skillName = "aks-access"

// SkillFS is the embedded skill tree, injected from package main. SkillEmbedRoot
// is the path of the skill directory within it.
var (
	SkillFS        fs.FS
	SkillEmbedRoot = ".claude/skills/" + skillName
)

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage the bundled coding-agent skill (aks-access)",
		Long: `The 'aks-access' skill teaches coding agents (Claude Code, GitHub Copilot, …)
how to reach AKS clusters through aks-helper. It is embedded in this binary, so
it can be installed without a checkout of the repository.`,
	}
	cmd.AddCommand(newSkillInstallCmd(), newSkillPrintCmd())
	return cmd
}

func newSkillInstallCmd() *cobra.Command {
	var (
		scope string
		agent string
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the agent skill into your skills directory",
		Long: `Writes the embedded skill into the skills directory that coding agents read.
Idempotent — re-running overwrites the files.

Locations (Agent Skills spec):
  global  claude=~/.claude/skills  copilot=~/.copilot/skills  agents=~/.agents/skills
  local   claude=.claude/skills    copilot=.github/skills     agents=.agents/skills`,
		Example: `  aks-helper skill install                  # global, all agents
  aks-helper skill install --agent copilot  # only ~/.copilot/skills
  aks-helper skill install --scope local    # into the current project`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if SkillFS == nil {
				return fmt.Errorf("this build does not embed the skill")
			}
			targets, err := skillTargets(scope, agent)
			if err != nil {
				return err
			}
			installed := 0
			for _, t := range targets {
				dest := filepath.Join(t.dir, skillName)
				if err := writeSkill(dest); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %v\n", t.agent, err)
					continue
				}
				fmt.Printf("installed skill %q (%s, %s) -> %s\n", skillName, t.agent, scope, dest)
				installed++
			}
			if installed == 0 {
				return fmt.Errorf("nothing installed")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "global", "global (into ~/) or local (into the current project)")
	cmd.Flags().StringVar(&agent, "agent", "all", "claude|copilot|agents|all")
	return cmd
}

func newSkillPrintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "print",
		Short: "Print the embedded SKILL.md to stdout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if SkillFS == nil {
				return fmt.Errorf("this build does not embed the skill")
			}
			data, err := fs.ReadFile(SkillFS, SkillEmbedRoot+"/SKILL.md")
			if err != nil {
				return err
			}
			fmt.Print(string(data))
			return nil
		},
	}
}

type skillTarget struct{ agent, dir string }

// skillTargets resolves the skills directories to write to for the given scope
// and agent selection.
func skillTargets(scope, agent string) ([]skillTarget, error) {
	var agents []string
	switch agent {
	case "all":
		agents = []string{"claude", "copilot", "agents"}
	case "claude", "copilot", "agents":
		agents = []string{agent}
	default:
		return nil, fmt.Errorf("invalid --agent %q (claude|copilot|agents|all)", agent)
	}

	var root string
	switch scope {
	case "global":
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		root = home
	case "local":
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		root = cwd
	default:
		return nil, fmt.Errorf("invalid --scope %q (global|local)", scope)
	}

	out := make([]skillTarget, 0, len(agents))
	for _, a := range agents {
		out = append(out, skillTarget{agent: a, dir: filepath.Join(root, skillBaseDir(scope, a))})
	}
	return out, nil
}

// skillBaseDir returns the per-agent skills directory, relative to the home (for
// global) or project root (for local).
func skillBaseDir(scope, agent string) string {
	switch agent {
	case "copilot":
		if scope == "local" {
			return filepath.Join(".github", "skills")
		}
		return filepath.Join(".copilot", "skills")
	case "agents":
		return filepath.Join(".agents", "skills")
	default: // claude
		return filepath.Join(".claude", "skills")
	}
}

// writeSkill copies the embedded skill tree into dest, preserving any nested
// files.
func writeSkill(dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return fs.WalkDir(SkillFS, SkillEmbedRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(SkillFS, p)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, SkillEmbedRoot+"/")
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
