package syntax

import (
	"strings"
	"testing"
)

func eq(t *testing.T, a, b string) error {
	t.Helper()
	fa, _, err := parse(t, a)
	if err != nil {
		t.Fatalf("%q: %v", a, err)
	}
	fb, _, err := parse(t, b)
	if err != nil {
		t.Fatalf("%q: %v", b, err)
	}
	return Equal(fa, fb, EqualOptions{})
}

func TestEqualIgnoresLayout(t *testing.T) {
	for _, c := range [][2]string{
		{"cube([1,2,3]);", "cube( [1, 2, 3] ) ;"},
		{"x=1; // c\n", "x = 1;\n/* other */"},
		{"use <a.scad>", "use   <a.scad>"},
		{"f(a, b,);", "f(a, b);"},
		{"x = [1, 2,];", "x = [\n  1,\n  2\n];"},
	} {
		if err := eq(t, c[0], c[1]); err != nil {
			t.Errorf("%q vs %q: %v", c[0], c[1], err)
		}
	}
}

// Each pair is a change the formatter must never make; Equal has to see it.
func TestEqualCatchesChanges(t *testing.T) {
	for _, c := range [][2]string{
		{"x = a + b * c;", "x = (a + b) * c;"},
		{"x = a + b + c;", "x = a + (b + c);"},
		{"x = 1;", "x = 2;"},
		{"x = 1.5;", "x = 1.50;"},
		{"cube(1);", "cube(2);"},
		{"cube(1); sphere(1);", "cube(1) sphere(1);"},
		{"if (a) b(); else c();", "if (a) b();"},
		{"if (a) { b(); } else c();", "if (a) { b(); } c();"},
		{"x = a ? b : c;", "x = a ? c : b;"},
		{"#cube(1);", "cube(1);"},
		{"cube(a = 1);", "cube(1);"},
		{"x = [1, 2];", "x = [1, 2, 3];"},
		{"x = [1:2];", "x = [1, 2];"},
		{"x = -a;", "x = +a;"},
		{"x = \"a\";", "x = \"b\";"},
		{"module m() {}", "function m() = 1;"},
		{"x = a ^ b ^ c;", "x = (a ^ b) ^ c;"},
		{"{ a(); b(); }", "{ b(); a(); }"},
	} {
		if err := eq(t, c[0], c[1]); err == nil {
			t.Errorf("%q vs %q: difference not detected", c[0], c[1])
		}
	}
}

func TestEqualNumberNormalizer(t *testing.T) {
	fa, _, _ := parse(t, "x = 1.50;")
	fb, _, _ := parse(t, "x = 1.5;")
	if Equal(fa, fb, EqualOptions{}) == nil {
		t.Error("without a normalizer 1.50 and 1.5 differ")
	}
	norm := func(s string) string { return strings.TrimRight(s, "0") }
	if err := Equal(fa, fb, EqualOptions{Number: norm}); err != nil {
		t.Errorf("with a normalizer they match: %v", err)
	}
}
