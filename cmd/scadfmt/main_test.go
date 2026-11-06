package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(t *testing.T, stdin string, args ...string) (code int, out, errs string) {
	t.Helper()
	var o, e bytes.Buffer
	code = run(args, strings.NewReader(stdin), &o, &e)
	return code, o.String(), e.String()
}

// project makes a temp dir with the given files and returns it. A stray config
// above the temp dir would change results, so tests pass --config or write their
// own.
func project(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestStdin(t *testing.T) {
	code, out, errs := runCLI(t, "cube( 1 );\n", "--no-config")
	if code != 0 || out != "cube(1);\n" {
		t.Fatalf("code=%d out=%q err=%q", code, out, errs)
	}
}

func TestCheck(t *testing.T) {
	if code, _, _ := runCLI(t, "cube(1);\n", "--no-config", "-check"); code != 0 {
		t.Errorf("clean input: code %d", code)
	}
	code, out, _ := runCLI(t, "cube( 1 );\n", "--no-config", "-check")
	if code != 1 || out != "<stdin>\n" {
		t.Errorf("dirty input: code %d out %q", code, out)
	}
}

func TestWriteFile(t *testing.T) {
	dir := project(t, map[string]string{"a.scad": "x=1;\n"})
	p := filepath.Join(dir, "a.scad")
	if code, _, errs := runCLI(t, "", "--no-config", "-w", p); code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	if b, _ := os.ReadFile(p); string(b) != "x = 1;\n" {
		t.Errorf("file = %q", b)
	}
}

func TestConfigFileAndFlagPrecedence(t *testing.T) {
	dir := project(t, map[string]string{
		".scadfmt.yaml": "indent: 4\n",
		"a.scad":        "{\ncube(1);\n}\n",
	})
	p := filepath.Join(dir, "a.scad")
	if _, out, _ := runCLI(t, "", p); out != "{\n    cube(1);\n}\n" {
		t.Errorf("config not applied: %q", out)
	}
	if _, out, _ := runCLI(t, "", "-indent", "3", p); out != "{\n   cube(1);\n}\n" {
		t.Errorf("flag must beat config: %q", out)
	}
	if _, out, _ := runCLI(t, "", "--no-config", p); out != "{\n  cube(1);\n}\n" {
		t.Errorf("--no-config: %q", out)
	}
	other := project(t, map[string]string{"c.yaml": "indent: 8\n"})
	if _, out, _ := runCLI(t, "", "--config", filepath.Join(other, "c.yaml"), p); out != "{\n        cube(1);\n}\n" {
		t.Errorf("--config: %q", out)
	}
}

func TestEachDirectoryUsesItsOwnConfig(t *testing.T) {
	dir := project(t, map[string]string{
		".scadfmt.yaml":   "indent: 4\n",
		"x/.scadfmt.yaml": "indent: 3\n",
		"a.scad":          "{\ncube(1);\n}\n",
		"x/b.scad":        "{\ncube(1);\n}\n",
	})
	_, out, _ := runCLI(t, "", filepath.Join(dir, "a.scad"), filepath.Join(dir, "x", "b.scad"))
	if out != "{\n    cube(1);\n}\n{\n   cube(1);\n}\n" {
		t.Errorf("out = %q", out)
	}
}

func TestStdinFilepath(t *testing.T) {
	dir := project(t, map[string]string{".scadfmt.yaml": "indent: 4\n"})
	_, out, _ := runCLI(t, "{\ncube(1);\n}\n", "--stdin-filepath", filepath.Join(dir, "x.scad"))
	if out != "{\n    cube(1);\n}\n" {
		t.Errorf("out = %q", out)
	}
}

func TestPresetAndExplicitFalse(t *testing.T) {
	src := "a=1;\nlonger=2;\n"
	_, out, _ := runCLI(t, src, "--no-config", "--preset", "pretty")
	if out != "a      = 1;\nlonger = 2;\n" {
		t.Errorf("pretty: %q", out)
	}
	_, out, _ = runCLI(t, src, "--no-config", "--preset", "pretty", "--align-assignments=false")
	if out != "a = 1;\nlonger = 2;\n" {
		t.Errorf("explicit false must override preset: %q", out)
	}
}

func TestDirectoryArgument(t *testing.T) {
	dir := project(t, map[string]string{"a.scad": "x=1;\n", "sub/b.scad": "y=2;\n", "sub/skip.txt": "z"})
	code, out, errs := runCLI(t, "", "--no-config", "-check", dir)
	if code != 1 || !strings.Contains(out, "a.scad") || !strings.Contains(out, "b.scad") || strings.Contains(out, "skip") {
		t.Errorf("code=%d out=%q err=%q", code, out, errs)
	}
}

func TestErrors(t *testing.T) {
	if code, _, errs := runCLI(t, "cube(", "--no-config"); code != 2 || !strings.Contains(errs, "<stdin>") {
		t.Errorf("syntax error: code %d err %q", code, errs)
	}
	for _, args := range [][]string{
		{"--chain", "sideways"}, {"--matrix", "x"}, {"--indent", "0"}, {"--preset", "nope"}, {"--bogus"},
	} {
		if code, _, _ := runCLI(t, "x=1;\n", append([]string{"--no-config"}, args...)...); code != 2 {
			t.Errorf("%v: code %d, want 2", args, code)
		}
	}
	dir := project(t, map[string]string{".scadfmt.yaml": "bogus: 1\n", "a.scad": "x=1;\n"})
	if code, _, errs := runCLI(t, "", filepath.Join(dir, "a.scad")); code != 2 || !strings.Contains(errs, "bogus") {
		t.Errorf("bad config: code %d err %q", code, errs)
	}
	if code, _, _ := runCLI(t, "", filepath.Join(dir, "missing.scad")); code != 2 {
		t.Errorf("missing file: code %d", code)
	}
}

func TestVersion(t *testing.T) {
	code, out, _ := runCLI(t, "", "-version")
	if code != 0 || !strings.HasPrefix(out, "scadfmt ") {
		t.Errorf("code=%d out=%q", code, out)
	}
}
