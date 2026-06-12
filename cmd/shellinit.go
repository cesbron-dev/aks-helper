package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newShellInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell-init [bash|zsh|fish|powershell|pwsh]",
		Short: "Print shell integration to enable 'aks use' to set KUBECONFIG",
		Long: `Prints a shell function named 'aks' that wraps aks-helper.

The wrapper intercepts 'aks use' (and its aliases) and evaluates the printed
statement that sets KUBECONFIG, which a child process cannot do on its own.
Every other subcommand is passed straight through.

Add to your shell startup file, e.g.:

  # ~/.bashrc
  eval "$(aks-helper shell-init bash)"

  # ~/.zshrc
  eval "$(aks-helper shell-init zsh)"

  # ~/.config/fish/config.fish
  aks-helper shell-init fish | source

  # PowerShell  ($PROFILE)
  aks-helper shell-init powershell | Out-String | Invoke-Expression`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell", "pwsh"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash", "zsh", "sh":
				fmt.Print(posixInit)
			case "fish":
				fmt.Print(fishInit)
			case "powershell", "pwsh":
				fmt.Print(powershellInit)
			default:
				return fmt.Errorf("unsupported shell %q (supported: bash, zsh, fish, powershell, pwsh)", args[0])
			}
			return nil
		},
	}
	return cmd
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
`
