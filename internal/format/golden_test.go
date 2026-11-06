package format

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGolden formats testdata/golden/*.in.scad and compares against the
// matching *.out.scad. It also checks idempotency on every output.
func TestGolden(t *testing.T) {
	inputs, err := filepath.Glob("../../testdata/golden/*.in.scad")
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range inputs {
		name := strings.TrimSuffix(filepath.Base(in), ".in.scad")
		t.Run(name, func(t *testing.T) {
			src := mustRead(t, in)
			want := mustRead(t, strings.TrimSuffix(in, ".in.scad")+".out.scad")
			got, err := Format(src, DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
			again, err := Format(got, DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(again, got) {
				t.Errorf("not idempotent\n--- first ---\n%s\n--- second ---\n%s", got, again)
			}
		})
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
