package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// InstallCompletion writes the completion script to the appropriate location.
func runInstallCompletion(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: clavis completion install bash|zsh|fish")
		os.Exit(1)
	}

	var script, target string
	home, _ := os.UserHomeDir()

	switch args[0] {
	case "bash":
		script = bashCompletion
		target = filepath.Join(home, ".bash_completion.d", "clavis")
	case "zsh":
		script = zshCompletion
		target = filepath.Join(home, ".zfunc", "_clavis")
	case "fish":
		script = fishCompletion
		target = filepath.Join(home, ".config", "fish", "completions", "clavis.fish")
	default:
		fmt.Fprintf(os.Stderr, "Unknown shell: %s. Supported: bash, zsh, fish\n", args[0])
		os.Exit(1)
	}

	os.MkdirAll(filepath.Dir(target), 0755)
	if err := os.WriteFile(target, []byte(script), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write completion script: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Wrote completion script to %s\n", target)
	fmt.Println()
	fmt.Println("To enable immediately:")
	switch args[0] {
	case "zsh":
		fmt.Printf("  autoload -Uz compinit && compinit")
		if !strings.Contains(os.Getenv("fpath"), filepath.Dir(target)) {
			fmt.Printf("\n  fpath=(%s $fpath)", filepath.Dir(target))
		}
		fmt.Println()
	case "bash":
		fmt.Printf("  source %s\n", target)
	case "fish":
		fmt.Println("  Restart your shell or run: exec fish")
	}
}
