package tui

import (
	"runtime"
	"strings"
	"testing"
)

func TestBannerCommandWrapsBinary(t *testing.T) {
	c := bannerCommand("/usr/local/bin/k9s", "Launching k9s for prod …")
	if c == nil {
		t.Fatal("nil command")
	}
	joined := strings.Join(c.Args, " ")
	if !strings.Contains(joined, "/usr/local/bin/k9s") {
		t.Errorf("command does not reference the binary: %v", c.Args)
	}
	if runtime.GOOS != "windows" {
		// On Unix the banner is passed as an argument to the sh wrapper.
		if !strings.Contains(joined, "Launching k9s for prod") {
			t.Errorf("banner not passed to wrapper: %v", c.Args)
		}
		if !strings.HasSuffix(c.Path, "sh") && !strings.Contains(c.Path, "/sh") {
			t.Errorf("expected an sh wrapper, got %q", c.Path)
		}
	}
}

func TestPSQuote(t *testing.T) {
	if got := psQuote(`C:\Program Files\k9s.exe`); got != `'C:\Program Files\k9s.exe'` {
		t.Errorf("psQuote = %q", got)
	}
	if got := psQuote(`it's`); got != `'it''s'` {
		t.Errorf("psQuote escape = %q", got)
	}
}
