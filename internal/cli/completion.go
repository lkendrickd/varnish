// completion.go implements the "varnish completion" command.
//
// This file is used by:
//   - cli/root.go: dispatches "completion" command here
//
// Generates shell completion scripts for bash, zsh, and fish.
// Users source the output to enable tab completion.
//
// Usage:
//
//	varnish completion bash   # Output bash completion script
//	varnish completion zsh    # Output zsh completion script
//	varnish completion fish   # Output fish completion script
package cli

import (
	"fmt"
	"io"
)

func runCompletion(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printCompletionUsage(stdout)
		return nil
	}

	shell := args[0]

	switch shell {
	case "bash":
		fmt.Fprint(stdout, bashCompletion)
	case "zsh":
		fmt.Fprint(stdout, zshCompletion)
	case "fish":
		fmt.Fprint(stdout, fishCompletion)
	case "help", "-h", "--help":
		printCompletionUsage(stdout)
	default:
		fmt.Fprintf(stderr, "unknown shell: %s\n\n", shell)
		printCompletionUsage(stderr)
		return fmt.Errorf("unknown shell: %s (supported: bash, zsh, fish)", shell)
	}

	return nil
}

func printCompletionUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage: varnish completion <shell>

Generate shell completion scripts.

Supported shells:
  bash    Bash completion script
  zsh     Zsh completion script
  fish    Fish completion script

Setup:

  # Bash (add to ~/.bashrc)
  source <(varnish completion bash)

  # Zsh (add to ~/.zshrc)
  source <(varnish completion zsh)

  # Fish (add to ~/.config/fish/config.fish)
  varnish completion fish | source`)
}

const bashCompletion = `# varnish bash completion
_varnish_completions() {
    local cur prev words cword
    _init_completion || return

    local commands="init store env list project completion version help"
    local store_commands="set get list ls delete rm import"
    local project_commands="name list delete"

    case "${cword}" in
        1)
            COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))
            ;;
        2)
            case "${prev}" in
                store)
                    COMPREPLY=($(compgen -W "${store_commands}" -- "${cur}"))
                    ;;
                project)
                    COMPREPLY=($(compgen -W "${project_commands}" -- "${cur}"))
                    ;;
                completion)
                    COMPREPLY=($(compgen -W "bash zsh fish" -- "${cur}"))
                    ;;
                init)
                    COMPREPLY=($(compgen -W "--project -p --from -f --no-import --sync -s --force" -- "${cur}"))
                    ;;
                env)
                    COMPREPLY=($(compgen -W "--dry-run --force --output" -- "${cur}"))
                    ;;
                list)
                    COMPREPLY=($(compgen -W "--missing --json" -- "${cur}"))
                    ;;
                *)
                    ;;
            esac
            ;;
        3)
            case "${words[1]}" in
                store)
                    case "${prev}" in
                        set|get|delete|rm)
                            # Could complete with keys from store
                            COMPREPLY=($(compgen -W "--project -p --global -g" -- "${cur}"))
                            ;;
                        list|ls)
                            COMPREPLY=($(compgen -W "--pattern --project -p --global -g --json" -- "${cur}"))
                            ;;
                        import)
                            COMPREPLY=($(compgen -f -- "${cur}"))
                            ;;
                    esac
                    ;;
                project)
                    case "${prev}" in
                        delete)
                            COMPREPLY=($(compgen -W "--dry-run" -- "${cur}"))
                            ;;
                    esac
                    ;;
            esac
            ;;
    esac
}

complete -F _varnish_completions varnish
`

const zshCompletion = `#compdef varnish

_varnish() {
    local -a commands store_commands project_commands

    commands=(
        'init:Initialize project with .varnish.yaml'
        'store:Manage central variable store'
        'env:Generate .env file'
        'list:Show resolved variables'
        'project:Show/manage project info'
        'completion:Generate shell completion'
        'version:Show version'
        'help:Show help'
    )

    store_commands=(
        'set:Add or update a variable'
        'get:Retrieve a variable'
        'list:List variables'
        'ls:List variables (alias)'
        'delete:Remove a variable'
        'rm:Remove a variable (alias)'
        'import:Import from .env file'
    )

    project_commands=(
        'name:Show current project name'
        'list:List all projects'
        'delete:Delete project variables'
    )

    case "${words[2]}" in
        store)
            if (( CURRENT == 3 )); then
                _describe -t commands 'store commands' store_commands
            else
                case "${words[3]}" in
                    set|get|delete|rm)
                        _arguments \
                            '-p[Project namespace]:project:' \
                            '--project[Project namespace]:project:' \
                            '-g[Bypass project auto-detection]' \
                            '--global[Bypass project auto-detection]'
                        ;;
                    list|ls)
                        _arguments \
                            '--pattern[Glob pattern]:pattern:' \
                            '-p[Project namespace]:project:' \
                            '--project[Project namespace]:project:' \
                            '-g[Show all variables]' \
                            '--global[Show all variables]' \
                            '--json[Output as JSON]'
                        ;;
                    import)
                        _arguments \
                            '-p[Project namespace]:project:' \
                            '--project[Project namespace]:project:' \
                            '*:file:_files'
                        ;;
                esac
            fi
            ;;
        project)
            if (( CURRENT == 3 )); then
                _describe -t commands 'project commands' project_commands
            else
                case "${words[3]}" in
                    delete)
                        _arguments '--dry-run[Preview deletions]'
                        ;;
                esac
            fi
            ;;
        init)
            _arguments \
                '-p[Project name]:project:' \
                '--project[Project name]:project:' \
                '-f[Path to .env file]:file:_files' \
                '--from[Path to .env file]:file:_files' \
                '--no-import[Skip importing defaults]' \
                '-s[Sync store with .env]' \
                '--sync[Sync store with .env]' \
                '--force[Overwrite existing config]'
            ;;
        env)
            _arguments \
                '--dry-run[Preview without writing]' \
                '--force[Overwrite existing .env]' \
                '--output[Output path]:file:_files'
            ;;
        list)
            _arguments \
                '--missing[Show missing variables]' \
                '--json[Output as JSON]'
            ;;
        completion)
            if (( CURRENT == 3 )); then
                _values 'shell' bash zsh fish
            fi
            ;;
        *)
            if (( CURRENT == 2 )); then
                _describe -t commands 'varnish commands' commands
            fi
            ;;
    esac
}

_varnish "$@"
`

const fishCompletion = `# varnish fish completion

# Disable file completion by default
complete -c varnish -f

# Main commands
complete -c varnish -n "__fish_use_subcommand" -a "init" -d "Initialize project"
complete -c varnish -n "__fish_use_subcommand" -a "store" -d "Manage variable store"
complete -c varnish -n "__fish_use_subcommand" -a "env" -d "Generate .env file"
complete -c varnish -n "__fish_use_subcommand" -a "list" -d "Show resolved variables"
complete -c varnish -n "__fish_use_subcommand" -a "project" -d "Project info"
complete -c varnish -n "__fish_use_subcommand" -a "completion" -d "Generate completions"
complete -c varnish -n "__fish_use_subcommand" -a "version" -d "Show version"
complete -c varnish -n "__fish_use_subcommand" -a "help" -d "Show help"

# store subcommands
complete -c varnish -n "__fish_seen_subcommand_from store" -a "set" -d "Add/update variable"
complete -c varnish -n "__fish_seen_subcommand_from store" -a "get" -d "Get variable"
complete -c varnish -n "__fish_seen_subcommand_from store" -a "list ls" -d "List variables"
complete -c varnish -n "__fish_seen_subcommand_from store" -a "delete rm" -d "Delete variable"
complete -c varnish -n "__fish_seen_subcommand_from store" -a "import" -d "Import from file"

# store flags
complete -c varnish -n "__fish_seen_subcommand_from store" -s p -l project -d "Project namespace"
complete -c varnish -n "__fish_seen_subcommand_from store" -s g -l global -d "Bypass project detection"

# project subcommands
complete -c varnish -n "__fish_seen_subcommand_from project" -a "name" -d "Show project name"
complete -c varnish -n "__fish_seen_subcommand_from project" -a "list" -d "List all projects"
complete -c varnish -n "__fish_seen_subcommand_from project" -a "delete" -d "Delete project vars"

# completion shells
complete -c varnish -n "__fish_seen_subcommand_from completion" -a "bash zsh fish"

# init flags
complete -c varnish -n "__fish_seen_subcommand_from init" -s p -l project -d "Project name"
complete -c varnish -n "__fish_seen_subcommand_from init" -s f -l from -d "Path to .env file"
complete -c varnish -n "__fish_seen_subcommand_from init" -l no-import -d "Skip importing"
complete -c varnish -n "__fish_seen_subcommand_from init" -s s -l sync -d "Sync store"
complete -c varnish -n "__fish_seen_subcommand_from init" -l force -d "Overwrite config"

# env flags
complete -c varnish -n "__fish_seen_subcommand_from env" -l dry-run -d "Preview only"
complete -c varnish -n "__fish_seen_subcommand_from env" -l force -d "Overwrite .env"
complete -c varnish -n "__fish_seen_subcommand_from env" -l output -d "Output path"

# list flags
complete -c varnish -n "__fish_seen_subcommand_from list" -l missing -d "Show missing vars"
complete -c varnish -n "__fish_seen_subcommand_from list" -l json -d "JSON output"
`
