package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dinhlongviolin1/clavis/internal/claude"
	"github.com/dinhlongviolin1/clavis/internal/profile"
	"github.com/dinhlongviolin1/clavis/internal/provider"
	"github.com/dinhlongviolin1/clavis/internal/ui"
)

var Version = "dev" // set by ldflags -X at build time

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
		fmt.Printf("clavis %s\n", Version)
	case "add":
		runAddOrHeadless(args[1:])
	case "list", "ls":
		if len(args) > 1 && args[1] == "--json" { runListJSON(); break }
		runList()
	case "remove", "rm":
		runRemove(args[1:])
	case "completion":
		if len(args) > 1 && args[1] == "install" {
			runInstallCompletion(args[2:])
		} else {
			runCompletion(args[1:])
		}
	case "providers":
		runProvidersCmd(args[1:])
	case "validate":
		runValidate(args[1:])
	case "start":
		runSelector(args[1:])
	default:
		// First arg might be a profile name → quick launch
		if err := profile.ValidName(args[0]); err == nil && profile.Exists(args[0]) {
			runQuickLaunch(args[0], args[1:])
		} else if err == nil {
			fmt.Fprintf(os.Stderr, "unknown command %q — try %s or %s\n",
				args[0],
				ui.CommandStyle.Render("clavis --help"),
				ui.CommandStyle.Render("clavis list"))
			os.Exit(1)
		} else {
			fmt.Fprintf(os.Stderr, "%v\n", err)
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
  clavis list, ls             List all profiles
  clavis remove, rm <name>    Delete a profile
  clavis providers            Show provider catalog
  clavis validate <name>      Ping a profile's provider
  clavis version              Print version
	clavis completion <shell>    Generate shell completion script

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
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("Failed to list profiles: %v", err)))
		os.Exit(1)
	}

	if len(profiles) == 0 {
		fmt.Print(ui.DimStyle.Render("No profiles yet. ") +
			"Create one now with " + ui.CommandStyle.Render("clavis add") +
			", or run " + ui.CommandStyle.Render("clavis providers") + " to see supported backends.")
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
	if err := provider.Ping(result.Profile.BaseURL, result.APIKey, result.Profile.SonnetModel); err != nil {
		fmt.Print("\r" + strings.Repeat(" ", 60) + "\r")
		fmt.Println(ui.Warn(fmt.Sprintf("Key validation failed: %v", err)))
		fmt.Println(ui.NextStep("Edit the key, then retry:", "clavis validate "+result.Profile.Name))
	} else {
		fmt.Print("\r" + strings.Repeat(" ", 60) + "\r")
		fmt.Println(ui.Success("Key valid"))
	}

	if err := profile.Save(result.Profile, result.APIKey); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("Failed to save profile: %v", err)))
		os.Exit(1)
	}

	fmt.Println(ui.Success(fmt.Sprintf("Profile %q saved", result.Profile.Name)))
	fmt.Println(ui.NextStep("Launch it with:", "clavis "+result.Profile.Name))
}

// runList prints all profiles as a styled table.
func runList() {
	grouped, err := profile.ListByProvider()
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}

	total := 0
	for _, g := range grouped {
		total += len(g)
	}
	if total == 0 {
		fmt.Println(ui.DimStyle.Render("No profiles. Run: clavis add"))
		return
	}

	lastUsed := profile.LoadLastUsed()

	fmt.Println()
	// Group by provider
	providers := provider.All()
	for _, prov := range providers {
		profiles, ok := grouped[prov.ID]
		if !ok || len(profiles) == 0 {
			continue
		}

		fmt.Printf("  %s\n", ui.TitleStyle.Render(prov.DisplayName))

		for _, p := range profiles {
			marker := " "
			if p.Name == lastUsed {
				marker = "*"
			}
			updated := ""
			if p.UpdatedAt != "" {
				t, err := time.Parse(time.RFC3339, p.UpdatedAt)
				if err == nil {
					updated = formatRelativeTime(time.Since(t))
				}
			}
			sonnet := p.SonnetModel
			if sonnet == "" {
				sonnet = "(unset)"
			}
			fmt.Printf("  %s %-20s  %-24s  %s\n", marker, p.Name, sonnet, ui.DimStyle.Render(updated))
		}
	}
	fmt.Println()
	fmt.Printf("  %d profiles in %s\n", total, profile.Dir())
	fmt.Println()
}

// runRemove deletes a profile.
func runRemove(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: clavis remove <name> [--force]")
		os.Exit(1)
	}

	name := args[0]
	force := len(args) > 1 && (args[1] == "--force" || args[1] == "-f")

	if !profile.Exists(name) {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("Profile %q not found. Run %s to see all profiles.", name, ui.CommandStyle.Render("clavis list"))))
		os.Exit(1)
	}

	if !force && !ui.ConfirmDelete(name) {
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
	fmt.Printf("  %-14s %-30s %s\n", "ID", "NAME", "BASE URL")
	for _, p := range provider.All() {
		url := p.BaseURL
		if url == "" {
			url = "(user-provided)"
		}
		fmt.Printf("  %-14s %-30s %s\n", p.ID, p.DisplayName, url)
	}
	fmt.Println()
	fmt.Println(ui.DimStyle.Render("  Models are fetched live from each provider during clavis add."))
	fmt.Println()
}

// formatRelativeTime returns a human-readable relative time string.
func formatRelativeTime(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	case d < 7*24*time.Hour:
		ds := int(d.Hours() / 24)
		if ds == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", ds)
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
