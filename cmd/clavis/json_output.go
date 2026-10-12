package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mutex-solutions/clavis/internal/profile"
	"github.com/mutex-solutions/clavis/internal/provider"
)

// jsonError prints a properly escaped JSON error object to stderr and exits.
// Hand-formatting was a bug: an error string containing a quote or newline
// produced invalid JSON that broke agent pipelines.
func jsonError(err error) {
	out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
	fmt.Fprintln(os.Stderr, string(out))
	os.Exit(1)
}

func runListJSON() {
	profiles, err := profile.List()
	if err != nil {
		jsonError(err)
	}
	// Never emit `null` for an empty list — agents do `jq '.[]'` over it.
	if profiles == nil {
		profiles = []*profile.Profile{}
	}
	out, _ := json.Marshal(profiles)
	fmt.Println(string(out))
}

func runProvidersJSON() {
	providers := provider.All()
	data, err := json.MarshalIndent(providers, "", "  ")
	if err != nil {
		jsonError(err)
	}
	fmt.Println(string(data))
}

func runProvidersCmd(args []string) {
	if len(args) > 0 && args[0] == "--json" {
		runProvidersJSON()
		return
	}
	runProviders()
}
