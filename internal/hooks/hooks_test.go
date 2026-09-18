package hooks

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// clearHookEnv isolates a test from hook-related variables in the outer
// environment.
func clearHookEnv(t *testing.T) {
	t.Helper()
	t.Setenv(disableEnv, "")
	t.Setenv(OverrideEnv(PostImport), "")
}

// writeHook drops an executable post-import hook into the store's hooks dir.
func writeHook(t *testing.T, storeDir, body string) string {
	t.Helper()
	clearHookEnv(t)
	dir := Dir(storeDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, PostImport)
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("hook execution tests use sh scripts")
	}
}

func TestRunNoHookConfigured(t *testing.T) {
	clearHookEnv(t)
	ran, err := Run(context.Background(), t.TempDir(), Event{Name: PostImport, KubeconfigPath: "x"})
	if ran || err != nil {
		t.Fatalf("ran=%v err=%v", ran, err)
	}
}

func TestRunReceivesEnvAndArg(t *testing.T) {
	skipOnWindows(t)
	store := t.TempDir()
	out := filepath.Join(store, "seen.txt")
	writeHook(t, store, `printf '%s|%s|%s|%s' "$AKS_HELPER_EVENT" "$AKS_HELPER_KUBECONFIG" "$AKS_HELPER_CLUSTER" "$1" > `+out)

	kc := filepath.Join(store, "prod.yaml")
	ran, err := Run(context.Background(), store, Event{
		Name:           PostImport,
		KubeconfigPath: kc,
		Env:            map[string]string{"AKS_HELPER_CLUSTER": "prod"},
	})
	if !ran || err != nil {
		t.Fatalf("ran=%v err=%v", ran, err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want := "post-import|" + kc + "|prod|" + kc
	if string(got) != want {
		t.Errorf("hook saw %q, want %q", got, want)
	}
}

func TestRunFailureSurfacesOutput(t *testing.T) {
	skipOnWindows(t)
	store := t.TempDir()
	writeHook(t, store, `echo "proxy rewrite failed" >&2; exit 3`)

	ran, err := Run(context.Background(), store, Event{Name: PostImport, KubeconfigPath: "x"})
	if !ran || err == nil {
		t.Fatalf("ran=%v err=%v", ran, err)
	}
	if !strings.Contains(err.Error(), "proxy rewrite failed") {
		t.Errorf("error does not carry hook output: %v", err)
	}
}

func TestRunDisabledByEnv(t *testing.T) {
	skipOnWindows(t)
	store := t.TempDir()
	writeHook(t, store, "exit 1")
	t.Setenv(disableEnv, "1")

	ran, err := Run(context.Background(), store, Event{Name: PostImport, KubeconfigPath: "x"})
	if ran || err != nil {
		t.Fatalf("ran=%v err=%v", ran, err)
	}
}

func TestRunEnvOverridePath(t *testing.T) {
	skipOnWindows(t)
	store := t.TempDir()
	// A hook elsewhere, pointed at by the override variable, wins over the dir.
	other := filepath.Join(t.TempDir(), "custom.sh")
	out := filepath.Join(store, "out.txt")
	if err := os.WriteFile(other, []byte("#!/bin/sh\necho custom > "+out+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeHook(t, store, "echo dir > "+out)
	t.Setenv(OverrideEnv(PostImport), other)

	if _, err := Run(context.Background(), store, Event{Name: PostImport, KubeconfigPath: "x"}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(out)
	if strings.TrimSpace(string(got)) != "custom" {
		t.Errorf("override ignored: %q", got)
	}
}

func TestOverrideEnvName(t *testing.T) {
	if got := OverrideEnv(PostImport); got != "AKS_HELPER_HOOK_POST_IMPORT" {
		t.Errorf("OverrideEnv = %q", got)
	}
}
