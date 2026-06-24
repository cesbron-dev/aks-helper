package tui

import (
	"os/exec"
	"runtime"
	"strings"
)

// bannerCommand builds a command that clears the screen and prints banner before
// running bin, so the brief gap before an external full-screen program (k9s)
// paints shows a purposeful message instead of the underlying shell prompt.
//
// It wraps bin in a small shell (sh on Unix, PowerShell on Windows) and falls
// back to running bin directly when no suitable wrapper is available, so the
// launch never breaks.
func bannerCommand(bin, banner string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		for _, ps := range []string{"pwsh", "powershell"} {
			if p, err := exec.LookPath(ps); err == nil {
				script := "Clear-Host; Write-Host " + psQuote("  "+banner) + "; & " + psQuote(bin)
				return exec.Command(p, "-NoProfile", "-NoLogo", "-Command", script)
			}
		}
		return exec.Command(bin)
	}
	if sh, err := exec.LookPath("sh"); err == nil {
		// $1 is the banner; the rest is the program to exec. clear falls back to
		// an ANSI clear when the `clear` binary is missing.
		const script = `clear 2>/dev/null || printf '\033[2J\033[H'; printf '  %s\n' "$1"; shift; exec "$@"`
		return exec.Command(sh, "-c", script, "sh", banner, bin)
	}
	return exec.Command(bin)
}

// psQuote wraps s as a PowerShell single-quoted literal.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
