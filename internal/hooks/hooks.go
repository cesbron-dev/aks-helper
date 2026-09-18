// Package hooks runs user-provided extension scripts at defined points, so
// site-specific steps — rewriting the API server URL behind a corporate proxy,
// injecting proxy-url, patching TLS settings — can plug into aks-helper without
// forking it.
//
// A hook is resolved in this order:
//
//  1. the AKS_HELPER_HOOK_<EVENT> environment variable (path to any program),
//  2. an executable at <store>/hooks/<event> (Unix: no extension or .sh;
//     Windows: .ps1, .cmd, .bat or .exe).
//
// Setting AKS_HELPER_NO_HOOKS to any non-empty value disables all hooks.
package hooks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// PostImport fires after a cluster's kubeconfig has been fetched, converted and
// written, and before the cluster is recorded in the index. The hook receives
// the kubeconfig path as its first argument and may edit the file in place.
const PostImport = "post-import"

const (
	disableEnv  = "AKS_HELPER_NO_HOOKS"
	hookTimeout = 60 * time.Second
)

// Event describes one hook invocation. Env entries are exported to the hook
// process in addition to AKS_HELPER_EVENT and AKS_HELPER_KUBECONFIG.
type Event struct {
	Name           string
	KubeconfigPath string
	Env            map[string]string
}

// Run executes the hook configured for the event, if any. It reports whether a
// hook ran; a non-zero exit fails with the hook's output in the error so the
// caller can surface why the site-specific step rejected the kubeconfig.
func Run(ctx context.Context, storeDir string, ev Event) (bool, error) {
	if os.Getenv(disableEnv) != "" {
		return false, nil
	}
	argv := resolve(storeDir, ev.Name)
	if len(argv) == 0 {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(ctx, hookTimeout)
	defer cancel()

	args := append(argv[1:], ev.KubeconfigPath)
	cmd := exec.CommandContext(ctx, argv[0], args...)
	cmd.Env = append(os.Environ(),
		"AKS_HELPER_EVENT="+ev.Name,
		"AKS_HELPER_KUBECONFIG="+ev.KubeconfigPath,
	)
	for k, v := range ev.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return true, fmt.Errorf("%s hook: %s: %w", ev.Name, firstLines(msg, 5), err)
		}
		return true, fmt.Errorf("%s hook: %w", ev.Name, err)
	}
	return true, nil
}

// OverrideEnv returns the environment variable that overrides the hook location
// for an event, e.g. AKS_HELPER_HOOK_POST_IMPORT.
func OverrideEnv(event string) string {
	return "AKS_HELPER_HOOK_" + strings.ToUpper(strings.ReplaceAll(event, "-", "_"))
}

// Dir returns the directory scanned for hook scripts.
func Dir(storeDir string) string {
	return filepath.Join(storeDir, "hooks")
}

// resolve returns the command (program plus leading interpreter args) for the
// event's hook, or nil when none is configured.
func resolve(storeDir, event string) []string {
	if p := os.Getenv(OverrideEnv(event)); p != "" {
		return runnerFor(p)
	}
	base := filepath.Join(Dir(storeDir), event)
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{base + ".ps1", base + ".cmd", base + ".bat", base + ".exe"}
	} else {
		candidates = []string{base, base + ".sh"}
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return runnerFor(c)
		}
	}
	return nil
}

// runnerFor picks the interpreter needed to execute path, based on extension.
func runnerFor(path string) []string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ps1":
		for _, ps := range []string{"pwsh", "powershell"} {
			if p, err := exec.LookPath(ps); err == nil {
				return []string{p, "-NoProfile", "-File", path}
			}
		}
		return nil
	case ".cmd", ".bat":
		if c := os.Getenv("ComSpec"); c != "" {
			return []string{c, "/c", path}
		}
		return []string{"cmd", "/c", path}
	case ".sh":
		if sh, err := exec.LookPath("sh"); err == nil {
			return []string{sh, path}
		}
		return []string{path}
	default:
		return []string{path}
	}
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + " …"
}
