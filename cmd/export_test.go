package cmd

import "testing"

func TestExportStatement(t *testing.T) {
	cases := []struct {
		shell string
		path  string
		want  string
	}{
		{"posix", "/home/u/.kube/aks/prod.yaml", `export KUBECONFIG='/home/u/.kube/aks/prod.yaml'`},
		{"", "/p", `export KUBECONFIG='/p'`},
		{"bash", "/p", `export KUBECONFIG='/p'`},
		{"fish", "/p", `set -gx KUBECONFIG '/p'`},
		{"powershell", `C:\Users\u\.kube\aks\prod.yaml`, `$env:KUBECONFIG = 'C:\Users\u\.kube\aks\prod.yaml'`},
		{"pwsh", "/p", `$env:KUBECONFIG = '/p'`},
	}
	for _, c := range cases {
		got, err := exportStatement(c.shell, c.path)
		if err != nil {
			t.Errorf("exportStatement(%q): %v", c.shell, err)
			continue
		}
		if got != c.want {
			t.Errorf("exportStatement(%q, %q) = %q, want %q", c.shell, c.path, got, c.want)
		}
	}

	if _, err := exportStatement("tcsh", "/p"); err == nil {
		t.Error("expected error for unsupported shell")
	}
}

func TestQuotingEscapes(t *testing.T) {
	if got := posixQuote(`a'b`); got != `'a'\''b'` {
		t.Errorf("posixQuote = %q", got)
	}
	if got := psQuote(`a'b`); got != `'a''b'` {
		t.Errorf("psQuote = %q", got)
	}
}
