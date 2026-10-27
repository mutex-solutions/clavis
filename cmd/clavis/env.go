package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/mutex-solutions/clavis/internal/claude"
	"github.com/mutex-solutions/clavis/internal/profile"
	"github.com/mutex-solutions/clavis/internal/ui"
)

// runEnv implements `clavis env <profile>`: print the environment variables
// clavis would set, so an agent runner (Hermes/OpenClaw) can own the claude
// process itself. Shell format by default, JSON with --json.
//
//	eval "$(clavis env deepseek)"   # then run your own `claude`
//
// The API key is redacted unless --reveal-key is passed. As a safety measure,
// the real key is never written to a terminal unless explicitly forced.
func runEnv(args []string) {
	var (
		name      string
		asJSON    bool
		reveal    bool
		exportKey = true
	)
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		case "--reveal-key":
			reveal = true
		case "--no-export":
			exportKey = false
		case "-h", "--help":
			fmt.Println(`clavis env <profile> — print the env vars clavis sets for a profile

Usage:
  clavis env <name>              Shell export lines (key redacted)
  clavis env <name> --json       JSON object of the vars
  clavis env <name> --reveal-key Include the real API key
  clavis env <name> --no-export  Bare KEY=VALUE lines (no 'export')

Examples:
  eval "$(clavis env deepseek --reveal-key)"   # then run your own claude`)
			return
		default:
			if name == "" && !strings.HasPrefix(a, "-") {
				name = a
			}
		}
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, "Usage: clavis env <name> [--json] [--reveal-key] [--no-export]")
		os.Exit(exitUsage)
	}

	p, err := profile.Load(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(exitNotFound)
	}
	if p.PicksModelAtLaunch() {
		fmt.Fprintln(os.Stderr, ui.Error("profile \""+name+"\" picks its model at every launch, so there is no fixed model to export; bind one with: clavis edit "+name))
		os.Exit(exitUsage)
	}
	apiKey, err := profile.LoadKey(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(exitNotFound)
	}

	// Safety: don't spill the real key onto a screen unless forced.
	if reveal && ui.StdoutIsTTY() {
		fmt.Fprintln(os.Stderr, ui.Warn("refusing to print the API key to a terminal; pipe it, e.g. eval \"$(clavis env "+name+" --reveal-key)\""))
		os.Exit(exitUsage)
	}
	vars := claude.EnvVars(p, apiKey)
	if !reveal {
		if _, ok := vars["ANTHROPIC_AUTH_TOKEN"]; ok {
			vars["ANTHROPIC_AUTH_TOKEN"] = "***redacted*** (use --reveal-key)"
		}
	}

	if asJSON {
		out, _ := json.Marshal(vars)
		fmt.Println(string(out))
		return
	}

	// Deterministic order for stable diffs / eval.
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	prefix := "export "
	if !exportKey {
		prefix = ""
	}
	for _, k := range keys {
		fmt.Printf("%s%s=%s\n", prefix, k, strconv.Quote(vars[k]))
	}
}
