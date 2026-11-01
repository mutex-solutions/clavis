package claude

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/mutex-solutions/clavis/internal/profile"
)

func TestWriteModelPickerSettings(t *testing.T) {
	t.Setenv("CLAVIS_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, `{"data":[
			{"id":"deepseek-flash","display_name":"DeepSeek Flash","max_input_tokens":1048576},
			{"id":"glm-5.3"}
		]}`)
	}))
	defer srv.Close()

	p := &profile.Profile{Name: "default", ProviderID: "custom", BaseURL: srv.URL}
	path := WriteModelPickerSettings(p, "sk-key")
	if path == "" {
		t.Fatal("WriteModelPickerSettings returned no path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var got struct {
		ModelPicker struct {
			Options []struct {
				Model       string `json:"model"`
				Label       string `json:"label"`
				Description string `json:"description"`
			} `json:"options"`
		} `json:"modelPicker"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("settings are not valid JSON: %v", err)
	}
	opts := got.ModelPicker.Options
	if len(opts) != 2 {
		t.Fatalf("options = %d, want 2", len(opts))
	}
	if opts[0].Model != "deepseek-flash" || opts[0].Label != "DeepSeek Flash" {
		t.Errorf("first row = %+v", opts[0])
	}
	if !strings.Contains(opts[0].Description, "1048576") {
		t.Errorf("description = %q, want the reported window", opts[0].Description)
	}
	// A model without a display_name is labelled by its id.
	if opts[1].Label != "glm-5.3" {
		t.Errorf("second label = %q, want the id", opts[1].Label)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 600", perm)
	}
}

func TestWriteModelPickerSettingsUnreachable(t *testing.T) {
	t.Setenv("CLAVIS_HOME", t.TempDir())

	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close() // now nothing is listening

	p := &profile.Profile{Name: "default", ProviderID: "custom", BaseURL: srv.URL}
	if path := WriteModelPickerSettings(p, "sk-key"); path != "" {
		t.Errorf("path = %q, want empty when the provider cannot be reached", path)
	}
	if _, err := os.Stat(profile.ProfileDir(p.Name) + "/claude-settings.json"); err == nil {
		t.Error("a settings file was written despite the failed fetch")
	}
}

func TestWriteModelPickerSettingsSkipsOAuth(t *testing.T) {
	t.Setenv("CLAVIS_HOME", t.TempDir())
	p := &profile.Profile{Name: "acct", ProviderID: "claude", BaseURL: "https://api.anthropic.com", AuthMode: profile.AuthOAuth}
	if path := WriteModelPickerSettings(p, "tok"); path != "" {
		t.Errorf("path = %q, want empty for a setup-token profile", path)
	}
}

func TestHasSettingsFlag(t *testing.T) {
	for _, tt := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"-p", "hi"}, false},
		{[]string{"--settings", "x.json"}, true},
		{[]string{"--settings=x.json"}, true},
		{[]string{"--settings-other"}, false},
	} {
		if got := hasSettingsFlag(tt.args); got != tt.want {
			t.Errorf("hasSettingsFlag(%v) = %v, want %v", tt.args, got, tt.want)
		}
	}
}
