package profile

import (
	"strings"
	"testing"
)

func TestConfigDir_SharedByDefault(t *testing.T) {
	p := &Profile{Name: "x"}
	dir, isolated := p.ConfigDir()
	if isolated {
		t.Error("default profile should be shared, not isolated")
	}
	if dir != "" {
		t.Errorf("shared ConfigDir = %q, want empty (inherit default)", dir)
	}
}

func TestConfigDir_Isolated(t *testing.T) {
	p := &Profile{Name: "myprof", IsolatedConfig: true}
	dir, isolated := p.ConfigDir()
	if !isolated {
		t.Error("IsolatedConfig=true should report isolated")
	}
	if !strings.HasSuffix(dir, ".clavis/profiles/myprof") {
		t.Errorf("isolated ConfigDir = %q, want suffix .clavis/profiles/myprof", dir)
	}
}

func TestDir_ClavisHomeOverride(t *testing.T) {
	t.Setenv("CLAVIS_HOME", "/custom/clavis/root")
	if got := Dir(); got != "/custom/clavis/root" {
		t.Errorf("Dir() = %q, want the CLAVIS_HOME override", got)
	}
}

func TestDir_DefaultWhenNoOverride(t *testing.T) {
	t.Setenv("CLAVIS_HOME", "")
	if got := Dir(); got == "/custom/clavis/root" {
		t.Error("Dir() should fall back to ~/.clavis when CLAVIS_HOME is unset")
	}
}
