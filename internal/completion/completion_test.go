package completion

import (
	"strings"
	"testing"
)

var testFlags = []Flag{
	{Name: "w", Usage: "write result", Bool: true},
	{Name: "chain", Usage: "chains: auto, flat or break", Values: []string{"auto", "flat", "break"}},
	{Name: "config", Usage: "use this config file", File: true},
	{Name: "indent", Usage: "spaces per level"},
	{Name: "quote", Usage: "it's [odd]: really", Bool: true},
}

func gen(t *testing.T, shell string) string {
	t.Helper()
	s, err := Generate(shell, "scadfmt", testFlags)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestUnknownShell(t *testing.T) {
	if _, err := Generate("tcsh", "scadfmt", testFlags); err == nil || !strings.Contains(err.Error(), "bash, zsh, fish") {
		t.Errorf("want an error listing the shells, got %v", err)
	}
}

func TestEveryShellMentionsEveryFlag(t *testing.T) {
	for _, shell := range Shells() {
		s := gen(t, shell)
		for _, f := range testFlags {
			if !strings.Contains(s, f.Name) {
				t.Errorf("%s: flag %s missing", shell, f.Name)
			}
		}
		if !strings.Contains(s, ".scad") {
			t.Errorf("%s: no .scad file completion", shell)
		}
	}
}

func TestBashStructure(t *testing.T) {
	s := gen(t, "bash")
	for _, want := range []string{
		`-chain|--chain) _scadfmt_values "auto flat break"`,
		"-config|--config)\n            compopt -o filenames", // file flag
		`-indent|--indent) return 0`,
		`complete -F _scadfmt scadfmt`,
		`"-w --chain --config --indent --quote"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("bash script lacks %q", want)
		}
	}
	// The bool flag takes no value, so it has no case arm.
	if strings.Contains(s, "-w|--w") || strings.Contains(s, "-quote|--quote)") {
		t.Error("bool flags must not have value arms")
	}
}

func TestZshStructure(t *testing.T) {
	s := gen(t, "zsh")
	for _, want := range []string{
		"#compdef scadfmt",
		`'-w[write result]'`,
		`'--chain=[chains\: auto, flat or break]:chain:(auto flat break)'`,
		`'--config=[use this config file]:file:_files'`,
		`'--indent=[spaces per level]:indent: '`,
		`'*:file or directory:_files -g "*.scad"'`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("zsh script lacks %q\n%s", want, s)
		}
	}
	// Quote, brackets and colon in a description are escaped.
	if !strings.Contains(s, `'--quote[it'\''s \[odd\]\: really]'`) {
		t.Errorf("zsh escaping wrong:\n%s", s)
	}
}

func TestFishStructure(t *testing.T) {
	s := gen(t, "fish")
	for _, want := range []string{
		`complete -c scadfmt -f -a '(__fish_complete_suffix .scad)'`,
		`complete -c scadfmt -s w -d 'write result'`,
		`complete -c scadfmt -l chain -x -a 'auto flat break' -d 'chains: auto, flat or break'`,
		`complete -c scadfmt -l config -r -F -d 'use this config file'`,
		`complete -c scadfmt -l indent -x -d 'spaces per level'`,
		`complete -c scadfmt -l quote -d 'it\'s [odd]: really'`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("fish script lacks %q\n%s", want, s)
		}
	}
}
