package main

import (
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Every flag of the command line must show up in the generated scripts, and the
// fixed-value flags must offer exactly the values the options accept.
func TestCompletionCoversAllFlags(t *testing.T) {
	declared := map[string]bool{}
	newFlagSet(&flags{}, io.Discard).VisitAll(func(f *flag.Flag) { declared[f.Name] = true })
	described := map[string]bool{}
	for _, f := range completionFlags() {
		described[f.Name] = true
	}
	for name := range declared {
		if !described[name] {
			t.Errorf("flag -%s missing from completion", name)
		}
	}
	for _, shell := range []string{"bash", "zsh", "fish"} {
		code, out, errs := runCLI(t, "", "-completion", shell)
		if code != 0 {
			t.Fatalf("%s: code %d: %s", shell, code, errs)
		}
		for name := range declared {
			if !strings.Contains(out, name) {
				t.Errorf("%s script lacks flag %s", shell, name)
			}
		}
	}
	for name, values := range flagChoices {
		if !declared[name] {
			t.Errorf("choices for unknown flag %s", name)
		}
		for _, v := range values {
			// Each offered value must be one the CLI really accepts.
			if name == "completion" {
				if code, _, _ := runCLI(t, "", "-completion", v); code != 0 {
					t.Errorf("-completion %s rejected", v)
				}
				continue
			}
			if code, _, errs := runCLI(t, "x = 1;\n", "--no-config", "-"+name, v); code != 0 {
				t.Errorf("-%s %s offered by completion but rejected: %s", name, v, errs)
			}
		}
	}
}

func TestCompletionUnknownShell(t *testing.T) {
	code, _, errs := runCLI(t, "", "-completion", "tcsh")
	if code != 2 || !strings.Contains(errs, "tcsh") {
		t.Errorf("code=%d err=%q", code, errs)
	}
}

// TestBashCompletion runs the generated script in a real bash and checks what
// it offers for a series of command lines.
func TestBashCompletion(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash")
	}
	_, script, _ := runCLI(t, "", "-completion", "bash")
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "completion.bash")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(dir, "work")
	for _, p := range []string{"a.scad", "b.txt", "sub/c.scad", "sub/d.txt"} {
		os.MkdirAll(filepath.Dir(filepath.Join(work, p)), 0o755)
		os.WriteFile(filepath.Join(work, p), nil, 0o644)
	}

	// complete runs the completion function for words, with the cursor on word
	// number cword (0 is the command itself).
	complete := func(cword int, words ...string) []string {
		t.Helper()
		var quoted []string
		for _, w := range words {
			quoted = append(quoted, "'"+w+"'")
		}
		cmd := `cd "$1"; source "$2"; COMP_WORDS=(` + strings.Join(quoted, " ") + `); COMP_CWORD=` +
			string(rune('0'+cword)) + `; _scadfmt; printf '%s\n' "${COMPREPLY[@]}"`
		out, err := exec.Command(bash, "-c", cmd, "bash", work, scriptPath).CombinedOutput()
		if err != nil {
			t.Fatalf("bash failed: %v\n%s", err, out)
		}
		var got []string
		for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if l != "" {
				got = append(got, l)
			}
		}
		sort.Strings(got)
		return got
	}
	check := func(name string, got []string, want ...string) {
		t.Helper()
		sort.Strings(want)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s: got %q want %q", name, got, want)
		}
	}

	check("flag prefix", complete(1, "scadfmt", "--ch"), "--chain", "--chain-indent", "--check")
	check("short flag", complete(1, "scadfmt", "-w"), "-w")
	check("enum", complete(2, "scadfmt", "--chain", ""), "auto", "flat", "break")
	check("enum prefix", complete(2, "scadfmt", "--chain", "a"), "auto")
	check("single dash enum", complete(2, "scadfmt", "-matrix", "re"), "reflow")
	// '=' is a word break by default: --matrix=re arrives as three words.
	check("equals split", complete(3, "scadfmt", "--matrix", "=", "re"), "reflow")
	check("equals cursor", complete(2, "scadfmt", "--matrix", "="), "preserve", "reflow", "align")
	// ...but not everywhere: then it is one word and the flag part is kept.
	check("equals one word", complete(1, "scadfmt", "--matrix=pr"), "--matrix=preserve")
	check("preset", complete(2, "scadfmt", "--preset", ""), "default", "pretty")
	check("shell", complete(2, "scadfmt", "--completion", ""), "bash", "zsh", "fish")
	check("free-form value", complete(2, "scadfmt", "--indent", ""))
	check("scad files and dirs", complete(1, "scadfmt", ""), "a.scad", "sub")
	check("into a directory", complete(1, "scadfmt", "sub/"), "sub/c.scad")
	check("after a bool flag", complete(2, "scadfmt", "-w", ""), "a.scad", "sub")
	check("config takes any file", complete(2, "scadfmt", "--config", ""), "a.scad", "b.txt", "sub")
}
