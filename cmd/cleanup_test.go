package cmd

import "testing"

func TestIsStale(t *testing.T) {
	const fqdn = "myaks-abc123.hcp.eastus.azmk8s.io"
	cases := []struct {
		name        string
		server      string
		fqdn        string
		privateFqdn string
		want        bool
	}{
		{"matches current fqdn", "https://" + fqdn + ":443", fqdn, "", false},
		{"recreated -> new fqdn", "https://myaks-OLD999.hcp.eastus.azmk8s.io:443", fqdn, "", true},
		{"matches private fqdn", "https://myaks-priv.privatelink.eastus.azmk8s.io:443", "", "myaks-priv.privatelink.eastus.azmk8s.io", false},
		{"no stored server", "", fqdn, "", false},
		{"no fqdn known -> not stale", "https://" + fqdn + ":443", "", "", false},
		{"case-insensitive", "https://" + "MYAKS-ABC123.HCP.EASTUS.AZMK8S.IO" + ":443", fqdn, "", false},
	}
	for _, c := range cases {
		if got := isStale(c.server, c.fqdn, c.privateFqdn); got != c.want {
			t.Errorf("%s: isStale(%q,%q,%q)=%v want %v", c.name, c.server, c.fqdn, c.privateFqdn, got, c.want)
		}
	}
}

func TestReplaceBlock(t *testing.T) {
	marked := initMarkerStart + "\nBODY\n" + initMarkerEnd + "\n"

	// Append to a file with no existing block.
	got, replaced := replaceBlock("# rc\nalias k=kubectl\n", marked)
	if replaced {
		t.Error("should not report replaced when appending")
	}
	want := "# rc\nalias k=kubectl\n" + marked
	if got != want {
		t.Errorf("append:\n got %q\nwant %q", got, want)
	}

	// Re-running replaces in place (no duplication).
	got2, replaced2 := replaceBlock(got, initMarkerStart+"\nNEWBODY\n"+initMarkerEnd+"\n")
	if !replaced2 {
		t.Error("should report replaced when a block exists")
	}
	wantBody := "# rc\nalias k=kubectl\n" + initMarkerStart + "\nNEWBODY\n" + initMarkerEnd + "\n"
	if got2 != wantBody {
		t.Errorf("replace:\n got %q\nwant %q", got2, wantBody)
	}

	// Content preserved around the block.
	surrounded := "before\n" + marked + "after\n"
	got3, _ := replaceBlock(surrounded, initMarkerStart+"\nX\n"+initMarkerEnd+"\n")
	want3 := "before\n" + initMarkerStart + "\nX\n" + initMarkerEnd + "\n" + "after\n"
	if got3 != want3 {
		t.Errorf("surrounded:\n got %q\nwant %q", got3, want3)
	}

	// Appends a trailing newline when the file doesn't end with one.
	got4, _ := replaceBlock("no newline", marked)
	if got4 != "no newline\n"+marked {
		t.Errorf("missing newline: got %q", got4)
	}
}
