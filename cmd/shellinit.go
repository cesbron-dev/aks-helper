package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newShellInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell-init [bash|zsh|fish]",
		Short: "Print shell integration to enable 'aks use' to set KUBECONFIG",
		Long: `Prints a shell function named 'aks' that wraps aks-helper.

The wrapper intercepts 'aks use' (and its aliases) and evaluates the printed
'export KUBECONFIG=...', which a child process cannot do on its own. Every other
subcommand is passed straight through.

Add to your shell rc file, e.g.:

  # ~/.bashrc
  eval "$(aks-helper shell-init bash)"

  # ~/.zshrc
  eval "$(aks-helper shell-init zsh)"

  # ~/.config/fish/config.fish
  aks-helper shell-init fish | source`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash", "zsh":
				fmt.Print(posixInit)
			case "fish":
				fmt.Print(fishInit)
			default:
				return fmt.Errorf("unsupported shell %q (supported: bash, zsh, fish)", args[0])
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
      _out="$(command aks-helper "$@" --export)" || return $?
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
            set -l _out (command aks-helper $argv --export)
            or return $status
            eval $_out
            set -l _name (command aks-helper current --quiet 2>/dev/null)
            test -n "$_name"; and echo "switched to $_name (KUBECONFIG set)" >&2
        case '*'
            command aks-helper $argv
    end
end
`
