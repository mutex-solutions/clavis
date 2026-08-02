package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func runCompletion(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: clavis completion bash|zsh|fish")
		os.Exit(1)
	}
	switch args[0] {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		fmt.Fprintf(os.Stderr, "Unknown shell: %s. Supported: bash, zsh, fish\n", args[0])
		os.Exit(1)
	}
}

const bashCompletion = `_clavis() {
    local cur prev words cword
    _init_completion || return

    local commands="add list ls remove rm providers validate test update version start help completion"
    local profiles=$(ls -1 "$HOME/.clavis/profiles/" 2>/dev/null)

    case $prev in
        remove|rm|validate|test|start)
            COMPREPLY=($(compgen -W "$profiles" -- "$cur"))
            return
            ;;
        clavis)
            if [[ $cur == -* ]]; then
                COMPREPLY=($(compgen -W "--help --version -h -v" -- "$cur"))
            elif [[ -z $cur || $cur == [a-zA-Z]* ]]; then
                COMPREPLY=($(compgen -W "$commands $profiles" -- "$cur"))
            fi
            return
            ;;
    esac

    if [[ $cword -eq 1 ]]; then
        COMPREPLY=($(compgen -W "$commands $profiles" -- "$cur"))
    fi
}
complete -F _clavis clavis
`

const zshCompletion = `#compdef clavis

_clavis() {
    local -a commands profiles
    commands=(add list ls remove rm providers validate test update version start help completion)
    profiles=(${(f)"$(ls -1 $HOME/.clavis/profiles/ 2>/dev/null)"})

    local context state line
    typeset -A opt_args

    _arguments -C \
        '1: :->first' \
        '2: :->second' \
        '*:: :->args'

    case $state in
        first)
            _describe 'command' commands
            _describe 'profile' profiles
            ;;
        second)
            case $words[2] in
                remove|rm|validate)
                    _describe 'profile' profiles
                    ;;
                *)
                    _describe 'profile' profiles
                    ;;
            esac
            ;;
    esac
}

_clavis "$@"
`

const fishCompletion = `function __fish_clavis_profiles
    ls -1 "$HOME/.clavis/profiles/" 2>/dev/null
end

complete -c clavis -f

# Subcommands
complete -c clavis -n "not __fish_seen_subcommand_from add list ls remove rm providers validate test update version start help completion" \
    -a "add list ls remove rm providers validate test update version start help completion"

# Profile names for launch
complete -c clavis -n "not __fish_seen_subcommand_from add list ls remove rm providers validate test update version start help completion" \
    -a "(__fish_clavis_profiles)" -d "Profile"

# Profile names for remove/validate/test
complete -c clavis -n "__fish_seen_subcommand_from remove rm validate test start" \
    -a "(__fish_clavis_profiles)" -d "Profile"
`

// completionSpec is the single source of truth for where a shell's completion
// file lives and what goes in it. Install, uninstall, and the update-time
// refresh all resolve through here so they can never drift apart.
type completionSpec struct {
	shell  string
	path   string // absolute target path (OS-correct via filepath.Join)
	script string
}

// supportedShells lists the shells clavis can install completion for. These are
// the same on Linux, macOS, and Windows (under Git Bash / WSL / MSYS).
func supportedShells() []string { return []string{"bash", "zsh", "fish"} }

// completionSpecFor resolves the install target for a shell. The home dir is
// resolved with os.UserHomeDir(), so paths are correct on every OS.
func completionSpecFor(shell string) (completionSpec, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return completionSpec{}, fmt.Errorf("cannot resolve home directory: %w", err)
	}
	switch shell {
	case "bash":
		return completionSpec{shell, filepath.Join(home, ".bash_completion.d", "clavis"), bashCompletion}, nil
	case "zsh":
		return completionSpec{shell, filepath.Join(home, ".zfunc", "_clavis"), zshCompletion}, nil
	case "fish":
		return completionSpec{shell, filepath.Join(home, ".config", "fish", "completions", "clavis.fish"), fishCompletion}, nil
	default:
		return completionSpec{}, fmt.Errorf("unknown shell %q (supported: bash, zsh, fish)", shell)
	}
}

// runInstallCompletion handles `clavis completion install <shell>`.
func runInstallCompletion(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: clavis completion install bash|zsh|fish")
		os.Exit(1)
	}
	spec, err := completionSpecFor(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := writeCompletion(spec); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write completion script: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Wrote %s completion to %s\n", spec.shell, spec.path)
	fmt.Println()
	fmt.Println("To enable immediately:")
	switch spec.shell {
	case "zsh":
		// If clavis was previously installed and the file changed, zsh's cache
		// (~/.zcompdump) can still point at the old definition — clear it.
		fmt.Printf("  fpath=(%s $fpath)\n", filepath.Dir(spec.path))
		fmt.Println("  rm -f ~/.zcompdump* && autoload -Uz compinit && compinit")
	case "bash":
		fmt.Printf("  source %s\n", spec.path)
	case "fish":
		fmt.Println("  Restart your shell or run: exec fish")
	}
}

// runUninstallCompletion handles `clavis completion uninstall [shell]`. With no
// shell it removes every installed completion file it can find.
func runUninstallCompletion(args []string) {
	shells := args
	if len(shells) == 0 {
		shells = supportedShells()
	}

	removedZsh := false
	removedAny := false
	for _, sh := range shells {
		spec, err := completionSpecFor(sh)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if _, statErr := os.Stat(spec.path); statErr != nil {
			continue // nothing installed for this shell
		}
		if err := os.Remove(spec.path); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to remove %s: %v\n", spec.path, err)
			os.Exit(1)
		}
		fmt.Printf("Removed %s completion (%s)\n", spec.shell, spec.path)
		removedAny = true
		if spec.shell == "zsh" {
			removedZsh = true
		}
	}

	if !removedAny {
		fmt.Println("No installed completions found.")
		return
	}
	if removedZsh {
		// This is the exact gotcha: a deleted _clavis with a stale zcompdump
		// makes zsh print "function definition file not found" on <Tab>.
		fmt.Println()
		fmt.Println("zsh caches completions — clear it so the old entry doesn't linger:")
		fmt.Println("  rm -f ~/.zcompdump* && exec zsh")
	}
}

// refreshInstalledCompletions rewrites any completion files that are already
// installed, so new subcommands show up after an update. It never installs a
// file that wasn't there and never errors the caller — it's best-effort polish.
// Returns the shells whose completion was refreshed.
func refreshInstalledCompletions() []string {
	var refreshed []string
	for _, sh := range supportedShells() {
		spec, err := completionSpecFor(sh)
		if err != nil {
			continue
		}
		if _, err := os.Stat(spec.path); err != nil {
			continue // not installed — leave it alone
		}
		if err := writeCompletion(spec); err != nil {
			continue // unwritable — skip silently
		}
		refreshed = append(refreshed, spec.shell)
	}
	return refreshed
}

func writeCompletion(spec completionSpec) error {
	if err := os.MkdirAll(filepath.Dir(spec.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(spec.path, []byte(spec.script), 0o644)
}
