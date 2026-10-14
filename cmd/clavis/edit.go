package main

import (
	"fmt"
	"os"

	"github.com/mutex-solutions/clavis/internal/profile"
	"github.com/mutex-solutions/clavis/internal/provider"
	"github.com/mutex-solutions/clavis/internal/ui"
)

// runEdit implements `clavis edit [name]`: change an existing profile's settings
// in place.
//
// The stored API key is never required — `profile.Save` only rewrites .env when
// given a non-empty key, so an edit that touches the context window leaves the
// secret byte-for-byte identical. Re-entering it is asked for only when the user
// chooses to, and then it is validated before anything is written.
func runEdit(args []string) {
	// The TTY check happens as late as possible, so that --help and a bad
	// profile name still report properly from a script or an agent.
	name := ""
	for _, a := range args {
		switch a {
		case "-h", "--help", "-help":
			fmt.Println(`clavis edit — change an existing profile

Usage:
  clavis edit            Pick a profile, then edit it
  clavis edit <name>     Edit that profile

Opens a menu (context window, models, effort, thinking, auto-compaction,
config isolation, minimal mode, API key). Changes are held in memory and
written only when you pick "Done — save", so esc discards them.

The stored API key is left untouched unless you replace it.`)
			return
		default:
			if name == "" {
				name = a
			}
		}
	}

	if name == "" {
		profiles, err := profile.List()
		if err != nil {
			fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
			os.Exit(exitNotFound)
		}
		if len(profiles) == 0 {
			fmt.Fprintln(os.Stderr, ui.Error("no profiles to edit"))
			fmt.Println(ui.NextStep("Create one first:", "clavis add"))
			os.Exit(exitNotFound)
		}
		requireTTY("clavis add --provider <id> --model <id> --key <k> --context-window <n> --overwrite --json")
		name, err = ui.SelectProfileForEdit(profiles)
		if err != nil {
			return // cancelled
		}
	}

	p, err := profile.Load(name)
	if err != nil {
		// profile.Load already names the profile in its message.
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(exitNotFound)
	}
	requireTTY("clavis add --provider <id> --model <id> --key <k> --context-window <n> --overwrite --json")

	// A missing .env isn't fatal here: the profile is still editable, and the
	// key is only needed to look up the provider's model list.
	apiKey, _ := profile.LoadKey(name)

	result, err := ui.EditProfile(p, apiKey)
	if err != nil {
		fmt.Println(ui.DimStyle.Render("  cancelled — nothing saved"))
		return
	}

	// Validate a replacement key before it can displace a working one. On
	// failure the key change is dropped but the rest of the edits still save,
	// so a typo can't cost the user their other work.
	if result.APIKey != "" {
		if err := ui.Spinner("Validating key…", func() error {
			return provider.Ping(result.Profile.BaseURL, result.APIKey, result.Profile.SonnetModel)
		}); err != nil {
			fmt.Println(ui.Warn(fmt.Sprintf("Key validation failed: %v", err)))
			fmt.Println(ui.Info("Keeping the previously stored key; your other changes are still saved."))
			result.APIKey = ""
		}
	}

	// An empty key preserves .env exactly as it is on disk.
	if err := profile.Save(result.Profile, result.APIKey); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("failed to save: %v", err)))
		os.Exit(exitSave)
	}

	fmt.Println(ui.Success(fmt.Sprintf("Profile %q updated", name)))
	if result.Profile.MaxContextTokens != nil {
		fmt.Println(ui.Info("Context window: " + ui.FormatTokens(*result.Profile.MaxContextTokens)))
	}
	fmt.Println(ui.NextStep("Launch it with:", "clavis "+name))
}
