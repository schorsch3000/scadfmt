package format

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"scadfmt/internal/lexer"
	"scadfmt/internal/syntax"
)

// astDump runs `openscad --export-format ast` on a file and returns its output.
func astDump(bin, path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-o", "-", "--export-format", "ast", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out.String(), err
}

// TestOpenSCADAgrees formats the OpenSCAD test files and checks that OpenSCAD's
// own AST dump is identical for the original and the formatted file. This is
// the independent check; the built-in verify step uses our parser on both
// sides. It needs an `openscad` binary (set OPENSCAD to choose one) and is
// skipped without it.
//
// Files with include/use are skipped, since they resolve relative paths.
func TestOpenSCADAgrees(t *testing.T) {
	bin := os.Getenv("OPENSCAD")
	if bin == "" {
		var err error
		if bin, err = exec.LookPath("openscad"); err != nil {
			t.Skip("no openscad binary; set OPENSCAD to run this test")
		}
	}
	root := "../../openscad/tests/data/scad"
	if _, err := os.Stat(root); err != nil {
		t.Skip("no openscad checkout")
	}
	tmp := t.TempDir()
	checked, skipped := 0, 0
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".scad") {
			return nil
		}
		src, _ := os.ReadFile(p)
		toks, err := lexer.Lex(src)
		if err != nil {
			return nil
		}
		if _, err := syntax.Parse(toks); err != nil {
			return nil
		}
		for _, tk := range toks {
			if tk.Kind == lexer.Include || tk.Kind == lexer.Use {
				skipped++
				return nil
			}
		}
		want, err := astDump(bin, p)
		if err != nil {
			skipped++ // OpenSCAD itself rejects or crashes on this input
			return nil
		}
		for name, opts := range map[string]Options{
			"default": DefaultOptions(),
			"align":   fuzzOptions(3),
		} {
			out, err := Format(src, opts)
			if err != nil {
				t.Errorf("%s [%s]: %v", p, name, err)
				continue
			}
			f := filepath.Join(tmp, "formatted.scad")
			os.WriteFile(f, out, 0o644)
			got, err := astDump(bin, f)
			if err != nil {
				t.Errorf("%s [%s]: openscad fails on the formatted file: %v", p, name, err)
			} else if got != want {
				t.Errorf("%s [%s]: openscad AST differs after formatting", p, name)
			}
		}
		checked++
		return nil
	})
	t.Logf("checked %d files, skipped %d", checked, skipped)
}
