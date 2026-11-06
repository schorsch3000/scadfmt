// Package completion generates shell completion scripts for a command-line
// program from a description of its flags.
package completion

import (
	"fmt"
	"strings"
)

// Flag describes one command-line flag.
type Flag struct {
	Name   string   // without leading dashes
	Usage  string   // one-line description
	Bool   bool     // takes no value
	Values []string // the only valid values, if the flag has a fixed set
	File   bool     // the value is a file name
}

// Shells lists the supported shells.
func Shells() []string { return []string{"bash", "zsh", "fish"} }

// Generate returns the completion script of shell for the program prog. Besides
// the flags, the program takes .scad files and directories as arguments.
func Generate(shell, prog string, flags []Flag) (string, error) {
	switch shell {
	case "bash":
		return bash(prog, flags), nil
	case "zsh":
		return zsh(prog, flags), nil
	case "fish":
		return fish(prog, flags), nil
	}
	return "", fmt.Errorf("unknown shell %q (have: %s)", shell, strings.Join(Shells(), ", "))
}

// spelling is how a flag is offered: -w for one letter, --name otherwise.
func spelling(name string) string {
	if len(name) == 1 {
		return "-" + name
	}
	return "--" + name
}

// ---- bash ----

func bash(prog string, flags []Flag) string {
	var words, files, free []string
	var enumCases strings.Builder
	for _, f := range flags {
		words = append(words, spelling(f.Name))
		pat := "-" + f.Name + "|--" + f.Name
		switch {
		case f.Bool:
		case len(f.Values) > 0:
			fmt.Fprintf(&enumCases, "        %s) _%s_values %q; return 0 ;;\n", pat, prog, strings.Join(f.Values, " "))
		case f.File:
			files = append(files, pat)
		default:
			free = append(free, pat)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# bash completion for %s. Load it with:  source <(%s -complete bash)\n\n", prog, prog)
	fmt.Fprintf(&b, `# _%[1]s_values WORDS completes the current word from WORDS. If the word is
# `+"`--flag=val`"+` (when '=' is not a word break), the flag part is kept.
_%[1]s_values() {
    local prefix=""
    if [[ "$cur" == -*=* ]]; then prefix="${cur%%%%=*}="; fi
    COMPREPLY=($(compgen -W "$1" -P "$prefix" -- "${cur#*=}"))
}

_%[1]s() {
    local cur prev flag
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    flag="$prev"
    if [[ "$prev" == "=" ]]; then
        flag="${COMP_WORDS[COMP_CWORD-2]}"      # --flag=<cursor>, '=' is a word break
    elif [[ "$cur" == "=" ]]; then
        flag="$prev"; cur=""                    # --flag=<cursor> right after the '='
    elif [[ "$cur" == -*=* ]]; then
        flag="${cur%%%%=*}"                       # --flag=val in one word
    fi

    case "$flag" in
`, prog)
	b.WriteString(enumCases.String())
	if len(files) > 0 {
		fmt.Fprintf(&b, "        %s)\n            compopt -o filenames 2>/dev/null\n            COMPREPLY=($(compgen -f -- \"${cur#*=}\"))\n            return 0 ;;\n", strings.Join(files, "|"))
	}
	if len(free) > 0 {
		fmt.Fprintf(&b, "        %s) return 0 ;;   # free-form value, nothing to offer\n", strings.Join(free, "|"))
	}
	fmt.Fprintf(&b, `    esac

    if [[ "$cur" == -* ]]; then
        COMPREPLY=($(compgen -W %q -- "$cur"))
        return 0
    fi

    # Arguments are .scad files and directories.
    compopt -o filenames 2>/dev/null
    local IFS=$'\n'
    COMPREPLY=($(compgen -d -- "$cur") $(compgen -f -X '!*.scad' -- "$cur"))
}

complete -F _%[2]s %[2]s
`, strings.Join(words, " "), prog)
	return b.String()
}

// ---- zsh ----

// zshEscape makes s safe inside a single-quoted _arguments spec, in the
// [description] part.
func zshEscape(s string) string {
	r := strings.NewReplacer(`'`, `'\''`, `[`, `\[`, `]`, `\]`, `:`, `\:`)
	return r.Replace(s)
}

func zsh(prog string, flags []Flag) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#compdef %s\n# zsh completion for %s. Put it in a directory of $fpath as _%s,\n# or load it with:  source <(%s -complete zsh)\n\n", prog, prog, prog, prog)
	fmt.Fprintf(&b, "_%s() {\n  _arguments -s \\\n", prog)
	for _, f := range flags {
		opt := spelling(f.Name)
		switch {
		case f.Bool:
			fmt.Fprintf(&b, "    '%s[%s]' \\\n", opt, zshEscape(f.Usage))
		case len(f.Values) > 0:
			fmt.Fprintf(&b, "    '%s=[%s]:%s:(%s)' \\\n", opt, zshEscape(f.Usage), f.Name, strings.Join(f.Values, " "))
		case f.File:
			fmt.Fprintf(&b, "    '%s=[%s]:file:_files' \\\n", opt, zshEscape(f.Usage))
		default:
			fmt.Fprintf(&b, "    '%s=[%s]:%s: ' \\\n", opt, zshEscape(f.Usage), f.Name)
		}
	}
	b.WriteString("    '*:file or directory:_files -g \"*.scad\"'\n}\n\n")
	fmt.Fprintf(&b, "if [[ \"${funcstack[1]}\" == _%[1]s ]]; then\n  _%[1]s \"$@\"\nelse\n  compdef _%[1]s %[1]s\nfi\n", prog)
	return b.String()
}

// ---- fish ----

func fishEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s)
}

func fish(prog string, flags []Flag) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# fish completion for %s. Put it in ~/.config/fish/completions/%s.fish,\n# or load it with:  %s -complete fish | source\n\n", prog, prog, prog)
	fmt.Fprintf(&b, "complete -c %s -f -a '(__fish_complete_suffix .scad)'\n", prog)
	for _, f := range flags {
		sel := "-l " + f.Name
		if len(f.Name) == 1 {
			sel = "-s " + f.Name
		}
		line := fmt.Sprintf("complete -c %s %s", prog, sel)
		switch {
		case f.Bool:
		case len(f.Values) > 0:
			line += fmt.Sprintf(" -x -a '%s'", strings.Join(f.Values, " "))
		case f.File:
			line += " -r -F"
		default:
			line += " -x"
		}
		fmt.Fprintf(&b, "%s -d '%s'\n", line, fishEscape(f.Usage))
	}
	return b.String()
}
