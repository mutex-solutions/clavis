package main

import (
	"fmt"
	"os"

	"github.com/dinhlongviolin1/clavis/internal/profile"
	"github.com/dinhlongviolin1/clavis/internal/provider"
	"github.com/dinhlongviolin1/clavis/internal/ui"
)

// runValidate pings a profile's provider.
func runValidate(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: clavis validate <name>")
		os.Exit(1)
	}

	name := args[0]
	p, err := profile.Load(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}

	apiKey, err := profile.LoadKey(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}

	fmt.Printf("  Pinging %s (%s)... ", p.ProviderID, p.BaseURL)
	if err := provider.Ping(p.BaseURL, apiKey, p.SonnetModel); err != nil {
		fmt.Println(ui.Error(err.Error()))
		os.Exit(1)
	}
	fmt.Println(ui.Success("OK"))
}
