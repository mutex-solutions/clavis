package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mutex-solutions/clavis/internal/profile"
	"github.com/mutex-solutions/clavis/internal/provider"
)

// maxPickerModels caps the rows handed to Claude Code. An aggregator like
// OpenRouter lists hundreds of ids, and past a few dozen the picker stops
// being navigable.
const maxPickerModels = 50

// pickerSettings is the slice of Claude Code's settings file this needs.
type pickerSettings struct {
	ModelPicker pickerLineup `json:"modelPicker"`
}

type pickerLineup struct {
	Options []pickerRow `json:"options"`
}

type pickerRow struct {
	Model       string `json:"model"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

// WriteModelPickerSettings writes a settings file listing the provider's own
// models and returns its path. An empty path means "nothing to add".
//
// Claude Code's gateway discovery keeps only ids containing "claude" or
// "anthropic", which no third-party gateway satisfies, so a clavis session
// otherwise offers just the model it launched with. --settings is read in
// addition to the user's settings, so this never touches ~/.claude.
//
// The picker is a convenience: a provider that is unreachable or lists
// nothing must not stop the launch, hence the silent empty return.
func WriteModelPickerSettings(p *profile.Profile, apiKey string) string {
	if p.AuthMode == profile.AuthOAuth {
		// Claude Code's own picker already knows the account's models; there is
		// nothing to add and its listing needs OAuth headers, not an API key.
		return ""
	}
	models, err := provider.FetchModels(p.BaseURL, p.BaseURL, apiKey)
	if err != nil || len(models) == 0 {
		return ""
	}
	if len(models) > maxPickerModels {
		models = models[:maxPickerModels]
	}

	rows := make([]pickerRow, 0, len(models))
	for _, m := range models {
		label := m.DisplayName
		if label == "" {
			label = m.ID
		}
		row := pickerRow{Model: m.ID, Label: label}
		if w := m.ContextWindow(); w > 0 {
			row.Description = fmt.Sprintf("%d tokens of context", w)
		}
		rows = append(rows, row)
	}

	body, err := json.Marshal(pickerSettings{ModelPicker: pickerLineup{Options: rows}})
	if err != nil {
		return ""
	}

	dir := profile.ProfileDir(p.Name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	path := filepath.Join(dir, "claude-settings.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return ""
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return ""
	}
	return path
}

// hasSettingsFlag reports whether the caller is already passing --settings,
// in which case ours is dropped rather than stacked beside theirs.
func hasSettingsFlag(args []string) bool {
	for _, a := range args {
		if a == "--settings" || strings.HasPrefix(a, "--settings=") {
			return true
		}
	}
	return false
}
