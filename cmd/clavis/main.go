package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/dinhlongviolin1/clavis/internal/claude"
	"github.com/dinhlongviolin1/clavis/internal/profile"
	"github.com/dinhlongviolin1/clavis/internal/provider"
	"github.com/dinhlongviolin1/clavis/internal/ui"
)

const version = "0.1.0"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		runSelector(nil)
		return
	}

	switch args[0] {
	case "--help", "-h", "help":
		printUsage()
	case "--version", "-v", "version":
		fmt.Printf("clavis %s\n", version)
	case "add":
		runAdd()
	case "list", "ls":
		runList()
	case "remove", "rm":
		runRemove(args[1:])
	case "providers":
		runProviders()
	case "validate":
		runValidate(args[1:])
	case "start":
		runSelector(args[1:])
	default:
		// First arg might be a profile name → quick launch
		if err := profile.ValidName(args[0]); err == nil && profile.Exists(args[0]) {
			runQuickLaunch(args[0], args[1:])
		} else if err == nil {
			fmt.Fprintf(os.Stderr, "unknown command %q — run clavis --help\n", args[0])
			os.Exit(1)
		} else {
			fmt.Fprintf(os.Stderr, "invalid profile name: %v\n", err)
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Print(`clavis — Claude Code provider manager

Usage:
  clavis                      Open profile selector
  clavis <profile> [args...]  Quick launch a profile
  clavis start [args...]      Open profile selector
  clavis add                  Create a new profile
  clavis list                 List all profiles
  clavis remove <name>        Delete a profile
  clavis providers            Show provider catalog
  clavis validate <name>      Ping a profile's provider
  clavis version              Print version

Examples:
  clavis                                # TUI picker
  clavis deepseek                       # launch with deepseek profile
  clavis deepseek "fix the bug"         # forward prompt
  clavis deepseek -- --model opus       # forward flags after --
`)
}

// runSelector shows the inline profile picker, then launches.
func runSelector(extraArgs []string) {
	profiles, err := profile.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("failed to list profiles: %v", err)))
		os.Exit(1)
	}

	if len(profiles) == 0 {
		fmt.Println(ui.DimStyle.Render("No profiles yet. Let's create one."))
		runAdd()
		return
	}

	lastUsed := profile.LoadLastUsed()
	result, err := ui.SelectProfile(profiles, lastUsed)
	if err != nil {
		os.Exit(1)
	}

	switch result.Action {
	case "launch":
		launchProfile(result.Profile, extraArgs)
	case "add":
		runAdd()
	case "quit":
		// do nothing
	}
}

// runQuickLaunch launches a profile by name, skipping the selector.
func runQuickLaunch(name string, args []string) {
	p, err := profile.Load(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}
	launchProfile(p, args)
}

// launchProfile loads the key and spawns claude.
func launchProfile(p *profile.Profile, extraArgs []string) {
	apiKey, err := profile.LoadKey(p.Name)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}

	// Strip leading "--" if present
	if len(extraArgs) > 0 && extraArgs[0] == "--" {
		extraArgs = extraArgs[1:]
	}

	fmt.Println(ui.DimStyle.Render(fmt.Sprintf("  → launching claude via %s (%s)", p.ProviderID, p.SonnetModel)))

	code, err := claude.Launch(p, apiKey, extraArgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}
	os.Exit(code)
}

// runAdd runs the add-profile wizard.
func runAdd() {
	result, err := ui.AddProfile()
	if err != nil {
		// User cancelled (ctrl+c / esc)
		return
	}

	// Validate key with a ping
	fmt.Print(ui.DimStyle.Render("  Validating key..."))
	if err := validateKey(result.Profile.BaseURL, result.APIKey, result.Profile.SonnetModel); err != nil {
		fmt.Print("\r" + strings.Repeat(" ", 60) + "\r")
		fmt.Println(ui.Error(fmt.Sprintf("key validation failed: %v", err)))
		fmt.Println(ui.DimStyle.Render("  Saving profile anyway. Fix the key later with clavis add."))
	} else {
		fmt.Print("\r" + strings.Repeat(" ", 60) + "\r")
		fmt.Println(ui.Success("Key valid"))
	}

	if err := profile.Save(result.Profile, result.APIKey); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("failed to save: %v", err)))
		os.Exit(1)
	}

	fmt.Println(ui.Success(fmt.Sprintf("Profile %q saved", result.Profile.Name)))
}

// runList prints all profiles as a styled table.
func runList() {
	profiles, err := profile.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}

	if len(profiles) == 0 {
		fmt.Println(ui.DimStyle.Render("No profiles. Run: clavis add"))
		return
	}

	lastUsed := profile.LoadLastUsed()

	fmt.Println()
	fmt.Printf("  %-14s %-16s %-22s %-10s\n", "NAME", "PROVIDER", "SONNET MODEL", "LAST USED")
	for _, p := range profiles {
		marker := " "
		if p.Name == lastUsed {
			marker = "*"
		}
		fmt.Printf(" %s%-13s %-16s %-22s\n", marker, p.Name, p.ProviderID, p.SonnetModel)
	}
	fmt.Println()
	fmt.Printf("  %d profiles in %s\n", len(profiles), profile.Dir())
	fmt.Println()
}

// runRemove deletes a profile.
func runRemove(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: clavis remove <name>")
		os.Exit(1)
	}

	name := args[0]
	if !profile.Exists(name) {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("profile %q not found", name)))
		os.Exit(1)
	}

	if !ui.ConfirmDelete(name) {
		fmt.Println("Cancelled.")
		return
	}

	if err := profile.Delete(name); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}
	fmt.Println(ui.Success(fmt.Sprintf("Profile %q deleted", name)))
}

// runProviders prints the built-in provider catalog.
func runProviders() {
	fmt.Println()
	fmt.Printf("  %-14s %-30s %-8s %s\n", "ID", "NAME", "TIER", "BASE URL")
	for _, p := range provider.All() {
		url := p.BaseURL
		if url == "" {
			url = "(user-provided)"
		}
		fmt.Printf("  %-14s %-30s %-8s %s\n", p.ID, p.DisplayName, p.Tier, url)
	}
	fmt.Println()

	fmt.Println(ui.DimStyle.Render("  Models:"))
	for _, p := range provider.All() {
		if len(p.Models) == 0 {
			continue
		}
		for _, m := range p.Models {
			tiers := strings.Join(m.TierHints, ",")
			free := ""
			if m.Free {
				free = " [free]"
			}
			fmt.Printf("    %-12s %-28s %-18s %d ctx%s\n", p.ID, m.ID, tiers, m.Context, free)
		}
	}
	fmt.Println()
}

// runValidate pings a profile's provider.
func runValidate(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: clavis validate <name>")
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
	if err := validateKey(p.BaseURL, apiKey, p.SonnetModel); err != nil {
		fmt.Println(ui.Error(err.Error()))
		os.Exit(1)
	}
	fmt.Println(ui.Success("OK"))
}
