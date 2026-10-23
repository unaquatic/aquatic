package main

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"iter"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Options control how SVG draws the logo. Lengths are in logo pixels, the
// squares the logo is drawn with, except Scale.
type Options struct {
	// Scale is the size of a logo pixel in the image, in CSS pixels.
	Scale float64
	// PadX is the space between the logo and the left and right edges of the
	// image, and PadY the space between it and the top and bottom edges.
	PadX, PadY float64
	// Fg is the color of the logo.
	Fg string
	// Accent is the color of the letter that Letter picks. Empty means Fg.
	Accent string
	// Letter is the letter that Accent colors, counted from 1 at the left.
	// The letters are the runs of columns between empty ones, as Letters
	// yields them.
	Letter int
	// Bg is the color of the background. Empty leaves it transparent.
	Bg string
	// Radius rounds the corners of the background.
	Radius float64
	// Gap is the space between neighboring pixels, as a fraction of a pixel.
	Gap float64
	// Round rounds the corners of every pixel, as a fraction of a pixel. A
	// half draws circles.
	Round float64
	// Shadow is the color of a copy of the logo drawn behind it. Empty draws
	// none.
	Shadow string
	// ShadowX and ShadowY offset the shadow from the logo.
	ShadowX, ShadowY float64
	// Label names the image for screen readers. Empty means FlowName.
	// BindFlags leaves it out, since it does not change how the image looks.
	Label string
}

// BindFlags binds the fields of o to flags in fs, set to their defaults.
func BindFlags(fs *flag.FlagSet, o *Options) {
	fs.Float64Var(&o.Scale, "scale", 8, "Size of a logo `pixel` in the image, in CSS pixels")
	fs.Float64Var(&o.PadX, "px", 3, "Space left and right of the logo, in logo `pixels`")
	fs.Float64Var(&o.PadY, "py", 3, "Space above and below the logo, in logo `pixels`")
	fs.StringVar(&o.Fg, "fg", "#e6e6e6", "Logo `color`")
	fs.StringVar(&o.Accent, "accent", "", "`color` of the letter -letter picks, the logo color when empty")
	fs.IntVar(&o.Letter, "letter", 1, "Draw letter `n`, counted from 1 at the left, in the accent color")
	fs.StringVar(&o.Bg, "bg", "#1c1c1c", "Background `color`, transparent when empty")
	fs.Float64Var(&o.Radius, "radius", 0, "Corner radius of the background, in logo `pixels`")
	fs.Float64Var(&o.Gap, "gap", 0, "Space between pixels, as a `fraction` of a pixel")
	fs.Float64Var(&o.Round, "round", 0, "Corner radius of each pixel, as a `fraction` of a pixel, up to 0.5")
	fs.StringVar(&o.Shadow, "shadow", "", "Shadow `color`, no shadow when empty")
	fs.Float64Var(&o.ShadowX, "shadow-x", 1, "Shadow offset to the right, in logo `pixels`")
	fs.Float64Var(&o.ShadowY, "shadow-y", 1, "Shadow offset down, in logo `pixels`")
}

// Validate reports every option of o that is out of range.
func (o Options) Validate() error {
	var errs []error
	check := func(ok bool, format string, args ...any) {
		if !ok {
			errs = append(errs, fmt.Errorf(format, args...))
		}
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	check(finite(o.Scale) && o.Scale > 0, "scale %v is not positive", o.Scale)
	check(finite(o.PadX) && o.PadX >= 0, "px %v is negative", o.PadX)
	check(finite(o.PadY) && o.PadY >= 0, "py %v is negative", o.PadY)
	check(finite(o.Radius) && o.Radius >= 0, "radius %v is negative", o.Radius)
	check(finite(o.Gap) && o.Gap >= 0 && o.Gap < 1, "gap %v is not in [0, 1)", o.Gap)
	check(finite(o.Round) && o.Round >= 0 && o.Round <= 0.5, "round %v is not in [0, 0.5]", o.Round)
	check(finite(o.ShadowX) && finite(o.ShadowY), "shadow offset %v, %v is not finite", o.ShadowX, o.ShadowY)
	check(o.Letter >= 1, "letter %d is not positive", o.Letter)
	for _, c := range []struct{ name, value string }{
		{"fg", o.Fg}, {"accent", o.Accent}, {"bg", o.Bg}, {"shadow", o.Shadow},
	} {
		check(ValidColor(c.value), "%s %q is not a color", c.name, c.value)
	}
	check(o.Fg != "", "fg is empty")
	return errors.Join(errs...)
}

// ValidColor reports whether s can be a CSS color, such as "#ececec" or
// "rgb(0 0 0 / 50%)": it holds only letters, digits, spaces and the
// characters #(),./%-. The empty string is valid and means no color.
func ValidColor(s string) bool {
	for _, r := range s {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune(" #(),./%-", r)
		if !ok {
			return false
		}
	}
	return true
}

// A Point is a corner between logo pixels, or the pixel whose top left
// corner it is.
type Point struct{ X, Y int }

// A Grid holds the logo's pixels row by row. True marks a filled pixel.
type Grid [][]bool

// ParseGrid reads art drawn in full blocks, upper and lower half blocks and
// spaces, two pixel rows to a line, and crops it to its filled pixels.
func ParseGrid(art string) (Grid, error) {
	lines := strings.Split(art, "\n")
	width := 0
	for _, line := range lines {
		width = max(width, len([]rune(line)))
	}
	g := make(Grid, 2*len(lines))
	for y := range g {
		g[y] = make([]bool, width)
	}
	for i, line := range lines {
		top, bottom := g[2*i], g[2*i+1]
		for x, r := range []rune(line) {
			switch r {
			case '█':
				top[x], bottom[x] = true, true
			case '▀':
				top[x] = true
			case '▄':
				bottom[x] = true
			case ' ':
			default:
				return nil, fmt.Errorf("line %d, column %d: %q is not a block", i+1, x+1, r)
			}
		}
	}
	return g.Crop()
}

// Crop returns the part of g between its outermost filled pixels. It fails
// when g has none.
func (g Grid) Crop() (Grid, error) {
	minX, minY, maxX, maxY := math.MaxInt, math.MaxInt, -1, -1
	for p := range g.Pixels() {
		minX, maxX = min(minX, p.X), max(maxX, p.X)
		minY, maxY = min(minY, p.Y), max(maxY, p.Y)
	}
	if maxX < 0 {
		return nil, errors.New("the art has no blocks")
	}
	out := make(Grid, maxY-minY+1)
	for y := range out {
		out[y] = g[minY+y][minX : maxX+1]
	}
	return out, nil
}

// Width returns the number of pixel columns of g.
func (g Grid) Width() int {
	if len(g) == 0 {
		return 0
	}
	return len(g[0])
}

// Height returns the number of pixel rows of g.
func (g Grid) Height() int { return len(g) }

// Filled reports whether the pixel at x, y is filled. Pixels outside g are
// not.
func (g Grid) Filled(x, y int) bool {
	return y >= 0 && y < len(g) && x >= 0 && x < len(g[y]) && g[y][x]
}

// Pixels yields the filled pixels of g, row by row from the top.
func (g Grid) Pixels() iter.Seq[Point] {
	return func(yield func(Point) bool) {
		for y, row := range g {
			for x, filled := range row {
				if filled && !yield(Point{x, y}) {
					return
				}
			}
		}
	}
}

// FirstLetter returns the width of the first letter of g: the index of the
// first empty column, or the width of g when no column is empty.
func (g Grid) FirstLetter() int {
	for x := range g.Width() {
		empty := true
		for y := range g.Height() {
			empty = empty && !g[y][x]
		}
		if empty {
			return x
		}
	}
	return g.Width()
}

// Letters yields the columns of each letter of g, left to right, from its
// first column to the one after its last: the runs of columns that hold a
// filled pixel, between empty columns.
func (g Grid) Letters() iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		from := -1
		for x := range g.Width() + 1 {
			filled := false
			for y := range g.Height() {
				filled = filled || g.Filled(x, y)
			}
			switch {
			case filled && from < 0:
				from = x
			case !filled && from >= 0:
				if !yield(from, x) {
					return
				}
				from = -1
			}
		}
	}
}

// Letter returns the columns of letter n of g, counted from 1 at the left, as
// Letters yields them, and false when g has fewer letters.
func (g Grid) Letter(n int) (from, to int, ok bool) {
	i := 0
	for from, to := range g.Letters() {
		if i++; i == n {
			return from, to, true
		}
	}
	return 0, 0, false
}

// Pick returns two grids of the size of g: one with the pixels of the columns
// from from up to to, and one with the rest.
func (g Grid) Pick(from, to int) (in, out Grid) {
	in, out = make(Grid, len(g)), make(Grid, len(g))
	for y, row := range g {
		in[y], out[y] = make([]bool, len(row)), slices.Clone(row)
		copy(in[y][from:to], row[from:to])
		clear(out[y][from:to])
	}
	return in, out
}

// Split returns two grids of the size of g: one with the pixels of the
// columns before x, and one with the rest.
func (g Grid) Split(x int) (left, right Grid) {
	left, right = make(Grid, len(g)), make(Grid, len(g))
	for y, row := range g {
		left[y], right[y] = make([]bool, len(row)), make([]bool, len(row))
		copy(left[y][:x], row[:x])
		copy(right[y][x:], row[x:])
	}
	return left, right
}

// An Edge is a side of a pixel, directed so that the pixel lies on its right
// with the y axis pointing down.
type Edge struct{ From, To Point }

// Outline returns the closed loops of corners that bound the filled pixels of
// g. Outer loops run clockwise and holes counterclockwise, with the y axis
// pointing down, so that the nonzero rule fills exactly the filled pixels.
// Corners within a straight side are left out.
func (g Grid) Outline() [][]Point {
	// Every side of a filled pixel that faces an empty one is an edge.
	var edges []Edge
	for p := range g.Pixels() {
		x, y := p.X, p.Y
		if !g.Filled(x, y-1) {
			edges = append(edges, Edge{Point{x, y}, Point{x + 1, y}})
		}
		if !g.Filled(x+1, y) {
			edges = append(edges, Edge{Point{x + 1, y}, Point{x + 1, y + 1}})
		}
		if !g.Filled(x, y+1) {
			edges = append(edges, Edge{Point{x + 1, y + 1}, Point{x, y + 1}})
		}
		if !g.Filled(x-1, y) {
			edges = append(edges, Edge{Point{x, y + 1}, Point{x, y}})
		}
	}
	leaving := make(map[Point][]int)
	for i, e := range edges {
		leaving[e.From] = append(leaving[e.From], i)
	}
	used := make([]bool, len(edges))
	var loops [][]Point
	for i, e := range edges {
		if used[i] {
			continue
		}
		// Every corner has as many edges leaving it as reaching it, so the
		// walk returns to its start.
		loop := []Point{e.From}
		for cur := i; ; {
			used[cur] = true
			to := edges[cur].To
			if to == e.From {
				break
			}
			loop = append(loop, to)
			k := slices.IndexFunc(leaving[to], func(j int) bool { return !used[j] })
			if k < 0 {
				break // Unreachable while corners balance.
			}
			cur = leaving[to][k]
		}
		loops = append(loops, Straighten(loop))
	}
	return loops
}

// Straighten returns the corners of the closed loop that turn, dropping the
// ones within a straight side.
func Straighten(loop []Point) []Point {
	n := len(loop)
	var out []Point
	for i, p := range loop {
		prev, next := loop[(i+n-1)%n], loop[(i+1)%n]
		straight := prev.X == p.X && p.X == next.X || prev.Y == p.Y && p.Y == next.Y
		if !straight {
			out = append(out, p)
		}
	}
	return out
}

// PathData returns the SVG path data that draws loops.
func PathData(loops [][]Point) string {
	var b strings.Builder
	for _, loop := range loops {
		for i, p := range loop {
			switch {
			case i == 0:
				fmt.Fprintf(&b, "M%d %d", p.X, p.Y)
			case p.Y == loop[i-1].Y:
				fmt.Fprintf(&b, "H%d", p.X)
			default:
				fmt.Fprintf(&b, "V%d", p.Y)
			}
		}
		b.WriteString("Z")
	}
	return b.String()
}

// WriteSVG writes g to w as an SVG image drawn as o says. o must be valid, and
// g must have the letter that o.Letter picks when o has an accent.
func WriteSVG(w io.Writer, g Grid, o Options) error {
	width, height := float64(g.Width())+2*o.PadX, float64(g.Height())+2*o.PadY
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" role="img" aria-label="%s">`+"\n",
		Num(width*o.Scale), Num(height*o.Scale), Num(width), Num(height), html.EscapeString(cmp.Or(o.Label, FlowName)))
	if o.Bg != "" {
		fmt.Fprintf(&b, `  <rect width="%s" height="%s" rx="%s" fill="%s"/>`+"\n", Num(width), Num(height), Num(o.Radius), o.Bg)
	}
	fmt.Fprintf(&b, `  <g transform="translate(%s %s)">`+"\n", Num(o.PadX), Num(o.PadY))
	if o.Shadow != "" {
		WriteShape(&b, g, o, o.Shadow, o.ShadowX, o.ShadowY)
	}
	if o.Accent != "" && o.Accent != o.Fg {
		from, to, ok := g.Letter(o.Letter)
		if !ok {
			return fmt.Errorf("letter %d: the art has fewer letters", o.Letter)
		}
		letter, rest := g.Pick(from, to)
		WriteShape(&b, letter, o, o.Accent, 0, 0)
		WriteShape(&b, rest, o, o.Fg, 0, 0)
	} else {
		WriteShape(&b, g, o, o.Fg, 0, 0)
	}
	b.WriteString("  </g>\n</svg>\n")
	_, err := io.WriteString(w, b.String())
	return err
}

// WriteShape writes the pixels of g in color to b, moved by dx and dy. Without
// a gap or rounded pixels it draws their outline as one path, which shows no
// seams between pixels at any scale, and otherwise every pixel on its own.
func WriteShape(b *strings.Builder, g Grid, o Options, color string, dx, dy float64) {
	move := ""
	if dx != 0 || dy != 0 {
		move = fmt.Sprintf(` transform="translate(%s %s)"`, Num(dx), Num(dy))
	}
	if o.Gap == 0 && o.Round == 0 {
		fmt.Fprintf(b, `    <path fill="%s"%s d="%s"/>`+"\n", color, move, PathData(g.Outline()))
		return
	}
	size := 1 - o.Gap
	rx := ""
	if o.Round > 0 {
		rx = fmt.Sprintf(` rx="%s"`, Num(o.Round*size))
	}
	fmt.Fprintf(b, `    <g fill="%s"%s>`+"\n", color, move)
	for p := range g.Pixels() {
		fmt.Fprintf(b, `      <rect x="%s" y="%s" width="%s" height="%s"%s/>`+"\n",
			Num(float64(p.X)+o.Gap/2), Num(float64(p.Y)+o.Gap/2), Num(size), Num(size), rx)
	}
	b.WriteString("    </g>\n")
}

// Num formats v for SVG, rounded to three decimals and without trailing
// zeros.
func Num(v float64) string {
	v = math.Round(v*1000) / 1000
	if v == 0 {
		v = 0 // Drops the sign of -0.
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
