package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	initMarkerStart = "# >>> aks-helper shell-init >>>"
	initMarkerEnd   = "# <<< aks-helper shell-init <<<"
)

func newShellInitCmd() *cobra.Command {
	var (
		install bool
		file    string
	)

	cmd := &cobra.Command{
		Use:   "shell-init [bash|zsh|fish|powershell|pwsh]",
		Short: "Print (or install) shell integration to enable 'aks use'",
		Long: `Prints a shell function named 'aks' that wraps aks-helper.

The wrapper intercepts 'aks use' (and its aliases) and evaluates the printed
statement that sets KUBECONFIG, which a child process cannot do on its own.
Every other subcommand is passed straight through.

It also defines two hyphenated shortcuts that mirror the classic script names:
'aks-select' (= aks use) and 'aks-get-cred' (= aks sync).

Print it and add it yourself:

  eval "$(aks-helper shell-init bash)"                              # ~/.bashrc
  aks-helper shell-init fish | source                              # fish
  aks-helper shell-init powershell | Out-String | Invoke-Expression # $PROFILE

…or let aks-helper write the block into your startup file (idempotent — it
updates the block in place on re-run):

  aks-helper shell-init bash --install
  aks-helper shell-init powershell --install --file $PROFILE`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell", "pwsh"},
		RunE: func(cmd *cobra.Command, args []string) error {
			block, err := initBlock(args[0])
			if err != nil {
				return err
			}
			if !install {
				fmt.Print(block)
				return nil
			}
			return installInitBlock(cmd, args[0], file, block)
		},
	}
	cmd.Flags().BoolVar(&install, "install", false, "write the block into your shell startup file instead of printing it")
	cmd.Flags().StringVar(&file, "file", "", "startup file to write to (default: auto-detected per shell)")
	return cmd
}

// initBlock returns the shell-init snippet for a shell.
func initBlock(shell string) (string, error) {
	switch shell {
	case "bash", "zsh", "sh":
		return posixInit, nil
	case "fish":
		return fishInit, nil
	case "powershell", "pwsh":
		return powershellInit, nil
	default:
		return "", fmt.Errorf("unsupported shell %q (supported: bash, zsh, fish, powershell, pwsh)", shell)
	}
}

// installInitBlock writes (or updates) the marked shell-init block in the shell
// startup file, leaving everything else untouched.
func installInitBlock(cmd *cobra.Command, shell, file, block string) error {
	if file == "" {
		var err error
		file, err = defaultInitFile(shell)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}

	existing, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	marked := initMarkerStart + "\n" + strings.TrimRight(block, "\n") + "\n" + initMarkerEnd + "\n"
	updated, replaced := replaceBlock(string(existing), marked)
	if err := os.WriteFile(file, []byte(updated), 0o644); err != nil {
		return err
	}

	action := "added aks-helper shell-init to"
	if replaced {
		action = "updated aks-helper shell-init in"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", action, file)
	fmt.Fprintf(cmd.ErrOrStderr(), "Restart your shell or reload it to pick up the change.\n")
	return nil
}

// replaceBlock swaps the existing marked block for marked, or appends it. It
// returns the new content and whether an existing block was replaced.
func replaceBlock(content, marked string) (string, bool) {
	start := strings.Index(content, initMarkerStart)
	end := strings.Index(content, initMarkerEnd)
	if start >= 0 && end > start {
		// Replace from the start marker through the end marker (and its newline).
		tail := end + len(initMarkerEnd)
		if tail < len(content) && content[tail] == '\n' {
			tail++
		}
		return content[:start] + marked + content[tail:], true
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + marked, false
}

// defaultInitFile returns the conventional startup file for a shell.
func defaultInitFile(shell string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch shell {
	case "bash":
		return filepath.Join(home, ".bashrc"), nil
	case "zsh":
		return filepath.Join(home, ".zshrc"), nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish"), nil
	case "powershell", "pwsh":
		return "", fmt.Errorf("cannot auto-detect the PowerShell profile path — pass --file \"$PROFILE\"")
	default:
		return "", fmt.Errorf("no default startup file known for %q — pass --file", shell)
	}
}

const posixInit = `# aks-helper shell integration
aks() {
  case "$1" in
    use|select|switch)
      local _out
      _out="$(command aks-helper "$@" --export --shell posix)" || return $?
      eval "$_out"
      local _name
      _name="$(command aks-helper current --quiet 2>/dev/null)"
      [ -n "$_name" ] && echo "switched to $_name (KUBECONFIG set)" >&2
      ;;
    *)
      command aks-helper "$@"
      ;;
  esac
}

# Hyphenated shortcuts mirroring the classic script names.
aks-select()   { aks use "$@"; }
aks-get-cred() { aks sync "$@"; }
`

const fishInit = `# aks-helper shell integration
function aks
    switch $argv[1]
        case use select switch
            set -l _out (command aks-helper $argv --export --shell fish)
            or return $status
            eval $_out
            set -l _name (command aks-helper current --quiet 2>/dev/null)
            test -n "$_name"; and echo "switched to $_name (KUBECONFIG set)" >&2
        case '*'
            command aks-helper $argv
    end
end

# Hyphenated shortcuts mirroring the classic script names.
function aks-select
    aks use $argv
end
function aks-get-cred
    aks sync $argv
end
`

const powershellInit = `# aks-helper shell integration
function aks {
    if ($args.Count -gt 0 -and @('use', 'select', 'switch') -contains $args[0]) {
        $out = & aks-helper @args --export --shell powershell
        if ($LASTEXITCODE -ne 0) { return }
        $out | Invoke-Expression
        $name = & aks-helper current --quiet 2>$null
        if ($name) { Write-Host "switched to $name (KUBECONFIG set)" }
    }
    else {
        & aks-helper @args
    }
}

# Hyphenated shortcuts mirroring the classic script names.
function aks-select { aks use @args }
function aks-get-cred { aks sync @args }
`
