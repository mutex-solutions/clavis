package main

import (
	"fmt"
	"os"

	"github.com/mutex-solutions/clavis/internal/profile"
	"github.com/mutex-solutions/clavis/internal/provider"
	"github.com/mutex-solutions/clavis/internal/ui"
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
		os.Exit(exitNotFound)
	}

	apiKey, err := profile.LoadKey(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(exitNotFound)
	}

	if p.PicksModelAtLaunch() {
		fmt.Printf("  Listing models from %s (%s)... ", p.ProviderID, p.BaseURL)
		models, err := modelCatalogue(p, apiKey)
		if err != nil {
			fmt.Println(ui.Error(err.Error()))
			os.Exit(exitValidation)
		}
		if len(models) == 0 {
			fmt.Println(ui.Error("the provider reported no models to pick from"))
			os.Exit(exitValidation)
		}
		fmt.Println(ui.Success(fmt.Sprintf("%d models", len(models))))
		return
	}

	fmt.Printf("  Pinging %s (%s)... ", p.ProviderID, p.BaseURL)
	if err := provider.Ping(p.BaseURL, apiKey, p.SonnetModel); err != nil {
		fmt.Println(ui.Error(err.Error()))
		os.Exit(exitValidation)
	}
	fmt.Println(ui.Success("OK"))
}
