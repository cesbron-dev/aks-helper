package cmd

import (
	"fmt"
	"strings"
)

// exportStatement renders the shell statement that sets KUBECONFIG to path,
// using the syntax of the requested shell. The output is meant to be eval'd
// (POSIX/fish) or Invoke-Expression'd (PowerShell) by the wrapper function from
// 'shell-init'.
func exportStatement(shell, path string) (string, error) {
	switch shell {
	case "", "posix", "bash", "zsh", "sh":
		return "export KUBECONFIG=" + posixQuote(path), nil
	case "fish":
		// fish has no 'export'; use a global exported variable instead.
		return "set -gx KUBECONFIG " + posixQuote(path), nil
	case "powershell", "pwsh":
		return "$env:KUBECONFIG = " + psQuote(path), nil
	default:
		return "", fmt.Errorf("unsupported shell %q (supported: posix, fish, powershell)", shell)
	}
}

// manualExportHint returns a copy-pasteable command a user can run by hand when
// no shell integration is installed.
func manualExportHint(shell, path string) string {
	stmt, err := exportStatement(shell, path)
	if err != nil {
		stmt, _ = exportStatement("posix", path)
	}
	return stmt
}

// posixQuote wraps s in single quotes, which disables every form of expansion,
// escaping any embedded single quote as the usual '\” sequence.
func posixQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// psQuote wraps s in PowerShell single quotes (literal string), doubling any
// embedded single quote.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
