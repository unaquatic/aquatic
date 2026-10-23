package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// halfBlocks draws g back in half blocks, two pixel rows to a line.
func halfBlocks(g Grid) string {
	var lines []string
	for y := 0; y < g.Height(); y += 2 {
		var line strings.Builder
		for x := range g.Width() {
			top, bottom := g.Filled(x, y), g.Filled(x, y+1)
			switch {
			case top && bottom:
				line.WriteRune('█')
			case top:
				line.WriteRune('▀')
			case bottom:
				line.WriteRune('▄')
			default:
				line.WriteRune(' ')
			}
		}
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}
	return strings.Join(lines, "\n")
}

// mustGrid parses art or fails t.
func mustGrid(t *testing.T, art string) Grid {
	t.Helper()
	g, err := ParseGrid(art)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestParseGridLogo checks that the logo parses into its 30 by 15 pixels,
// which draw the logo again under the blank pixel row that parsing crops off.
func TestParseGridLogo(t *testing.T) {
	g := mustGrid(t, FlowArt)
	if g.Width() != 30 || g.Height() != 15 {
		t.Errorf("grid is %d by %d, want 30 by 15", g.Width(), g.Height())
	}
	want := strings.TrimSuffix(FlowArt, "\n") // the file's last newline ends a line, and draws nothing
	if got := halfBlocks(append(Grid{make([]bool, g.Width())}, g...)); got != want {
		t.Errorf("grid draws\n%s\nwant\n%s", got, want)
	}
	if got := g.FirstLetter(); got != 7 {
		t.Errorf("first letter is %d columns wide, want 7", got)
	}
}

// TestParseGridCropsAndRejects checks that parsing crops to the filled pixel
// rows and columns, not to whole lines, and fails on art without blocks or
// with other characters.
func TestParseGridCropsAndRejects(t *testing.T) {
	g := mustGrid(t, "\n   \n  ▄▄\n  █")
	want := Grid{{true, true}, {true, false}, {true, false}}
	if !slices.EqualFunc(g, want, slices.Equal[[]bool]) {
		t.Errorf("cropped grid = %v, want %v", g, want)
	}
	for _, art := range []string{"", "  \n ", "█x"} {
		if _, err := ParseGrid(art); err == nil {
			t.Errorf("ParseGrid(%q) succeeded, want an error", art)
		}
	}
}

// area returns the signed area that loop encloses, positive when it runs
// clockwise with the y axis pointing down.
func area(loop []Point) int {
	sum := 0
	for i, p := range loop {
		q := loop[(i+1)%len(loop)]
		sum += p.X*q.Y - q.X*p.Y
	}
	return sum / 2
}

// TestOutline checks that the loops of a grid run along the axes and enclose
// as much area as it has filled pixels, holes counting against it.
func TestOutline(t *testing.T) {
	logo := mustGrid(t, FlowArt)
	first, rest := logo.Split(logo.FirstLetter())
	tests := []struct {
		name string
		g    Grid
	}{
		{name: "logo", g: logo},
		{name: "first letter", g: first},
		{name: "rest", g: rest},
		{name: "ring", g: mustGrid(t, "█▀█\n▀▀▀")},
		{name: "corners touch", g: Grid{{true, false}, {false, true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pixels, total := 0, 0
			for range tt.g.Pixels() {
				pixels++
			}
			for _, loop := range tt.g.Outline() {
				for i, p := range loop {
					q := loop[(i+1)%len(loop)]
					if p.X != q.X && p.Y != q.Y {
						t.Errorf("loop runs off the axes from %v to %v", p, q)
					}
				}
				total += area(loop)
			}
			if total != pixels {
				t.Errorf("loops enclose %d, want %d", total, pixels)
			}
		})
	}
}

func TestWriteSVG(t *testing.T) {
	g := mustGrid(t, "█")
	tests := []struct {
		name string
		o    Options
		want string
	}{
		// The padding widens the image by 2 on each side and heightens it by 1.
		{name: "outline", o: Options{Scale: 10, PadX: 2, PadY: 1, Fg: "#fff", Bg: "#000", Radius: 0.5}, want: `<svg xmlns="http://www.w3.org/2000/svg" width="50" height="40" viewBox="0 0 5 4" role="img" aria-label="flow">
  <rect width="5" height="4" rx="0.5" fill="#000"/>
  <g transform="translate(2 1)">
    <path fill="#fff" d="M0 0H1V2H0Z"/>
  </g>
</svg>
`},
		{name: "round pixels and shadow", o: Options{Scale: 1, Fg: "#fff", Gap: 0.2, Round: 0.5, Shadow: "#333", ShadowX: 0.5, ShadowY: -1}, want: `<svg xmlns="http://www.w3.org/2000/svg" width="1" height="2" viewBox="0 0 1 2" role="img" aria-label="flow">
  <g transform="translate(0 0)">
    <g fill="#333" transform="translate(0.5 -1)">
      <rect x="0.1" y="0.1" width="0.8" height="0.8" rx="0.4"/>
      <rect x="0.1" y="1.1" width="0.8" height="0.8" rx="0.4"/>
    </g>
    <g fill="#fff">
      <rect x="0.1" y="0.1" width="0.8" height="0.8" rx="0.4"/>
      <rect x="0.1" y="1.1" width="0.8" height="0.8" rx="0.4"/>
    </g>
  </g>
</svg>
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			if err := WriteSVG(&out, g, tt.o); err != nil {
				t.Fatal(err)
			}
			if out.String() != tt.want {
				t.Errorf("svg =\n%s\nwant\n%s", out.String(), tt.want)
			}
		})
	}
}

// TestWriteSVGLabel checks that a label replaces the product name, escaped
// for the attribute.
func TestWriteSVGLabel(t *testing.T) {
	var out strings.Builder
	if err := WriteSVG(&out, mustGrid(t, "█"), Options{Scale: 1, Fg: "#fff", Label: `a "b" & c`}); err != nil {
		t.Fatal(err)
	}
	if want := `aria-label="a &#34;b&#34; &amp; c"`; !strings.Contains(out.String(), want) {
		t.Errorf("svg =\n%s\nwant it to hold %s", out.String(), want)
	}
}

// TestWriteSVGAccent checks that an accent color draws the first letter in a
// path of its own.
func TestWriteSVGAccent(t *testing.T) {
	o, err := ParseOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	o.Accent = "#f5c66e"
	var out strings.Builder
	if err := WriteSVG(&out, mustGrid(t, FlowArt), o); err != nil {
		t.Fatal(err)
	}
	for _, fill := range []string{o.Accent, o.Fg} {
		if n := strings.Count(out.String(), `<path fill="`+fill+`"`); n != 1 {
			t.Errorf("svg has %d paths filled %s, want 1", n, fill)
		}
	}
}

func TestValidate(t *testing.T) {
	valid, err := ParseOptions(nil)
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	tests := []struct {
		name   string
		change func(*Options)
	}{
		{"zero scale", func(o *Options) { o.Scale = 0 }},
		{"negative px", func(o *Options) { o.PadX = -1 }},
		{"negative py", func(o *Options) { o.PadY = -1 }},
		{"whole gap", func(o *Options) { o.Gap = 1 }},
		{"round past a circle", func(o *Options) { o.Round = 0.6 }},
		{"no fg", func(o *Options) { o.Fg = "" }},
		{"markup in a color", func(o *Options) { o.Bg = `#000" onload="alert(1)` }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := valid
			tt.change(&o)
			if o.Validate() == nil {
				t.Errorf("options %+v are valid, want an error", o)
			}
		})
	}
}

// TestDocsLogosAreCurrent checks that the logos the READMEs show are the
// images of their art with their options. It skips the logos of fluid and
// sqlite when their repositories are not checked out next to this one.
func TestDocsLogosAreCurrent(t *testing.T) {
	for _, l := range docsLogos {
		t.Run(l.Path, func(t *testing.T) {
			got, err := os.ReadFile(l.Path)
			if errors.Is(err, fs.ErrNotExist) && strings.HasPrefix(l.Path, "../../../") {
				t.Skipf("%s is not checked out", l.Path)
			}
			if err != nil {
				t.Fatal(err)
			}
			o, err := ParseOptions(l.Options)
			if err != nil {
				t.Fatal(err)
			}
			o.Label = l.Label
			g, err := LoadGrid(l.Art)
			if err != nil {
				t.Fatal(err)
			}
			var want strings.Builder
			WriteSVG(&want, g, o)
			if string(got) != want.String() {
				t.Errorf("%s is out of date; run go generate ./cmd/logo", l.Path)
			}
		})
	}
}

// TestGenerateArt checks that the root command draws the art in the file
// that -art names, and fails on art that is not half blocks.
func TestGenerateArt(t *testing.T) {
	o, err := ParseOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	art, out := filepath.Join(dir, "art.txt"), filepath.Join(dir, "art.svg")
	if err := os.WriteFile(art, []byte("▄█\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate(RootOptions{Options: o, Art: art, Out: out}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var want strings.Builder
	WriteSVG(&want, mustGrid(t, "▄█"), o)
	if string(got) != want.String() {
		t.Errorf("file =\n%s\nwant\n%s", got, want.String())
	}

	if err := os.WriteFile(art, []byte("ab"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate(RootOptions{Options: o, Art: art, Out: out}); err == nil || !strings.HasPrefix(err.Error(), art+": ") {
		t.Errorf("err = %v, want one naming %s", err, art)
	}
}

// TestGenerate checks that the root command writes the file it names.
func TestGenerate(t *testing.T) {
	o, err := ParseOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "logo.svg")
	if err := Generate(RootOptions{Options: o, Out: path}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want strings.Builder
	WriteSVG(&want, mustGrid(t, FlowArt), o)
	if string(got) != want.String() {
		t.Errorf("file =\n%s\nwant\n%s", got, want.String())
	}
}
