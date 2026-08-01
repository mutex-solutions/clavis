package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dinhlongviolin1/clavis/internal/profile"
	"github.com/dinhlongviolin1/clavis/internal/provider"
)

func runListJSON() {
	profiles, err := profile.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, `{"ok":false,"error":"%s"}`+"\n", err)
		os.Exit(1)
	}
	out, _ := json.Marshal(profiles)
	fmt.Println(string(out))
}

func runProvidersJSON() {
	providers := provider.All()
	data, err := json.MarshalIndent(providers, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, `{"ok":false,"error":"%s"}`+"\n", err)
		os.Exit(1)
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

