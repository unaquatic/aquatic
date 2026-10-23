// Command logo draws flow's logo, the half-block art in art/flow.txt, or other
// art in half blocks, as an SVG image, and serves a page that previews the
// image while its options change. art/flow.txt is product.Logo in flow,
// which flow's help and /status draw.
//
// It also draws the README logos of this repository and of flow, fluid and
// sqlite, the repositories next to it, whose art is in art/. Each README shows
// docs/logo-dark.svg in dark mode and docs/logo-light.svg, its other colors
// swapped, in light mode. They draw in the colors flow's help gives its logo
// on a terminal: one letter in orange (PrimaryColor), the first but in
// fluid's, whose u it is, and the rest in
// light grey (xterm color 254, #e4e4e4), on near-black (xterm color 234,
// #1c1c1c), with 4 pixels of space left and right of the logo and 3 above
// and below it. In light mode the orange letter is a darker orange,
// lightAccent, which stands out from the light grey. go generate writes them
// all again, which needs flow (in flowcode), fluid and sqlite checked out
// next to this repository.
//
//	go run ./cmd/logo -o logo.svg
//	go run ./cmd/logo -px 4 -py 2 -o logo-wide.svg
//	go run ./cmd/logo -bg '' -fg '#000000' -o logo-light.svg
//	go run ./cmd/logo -art other.txt -o other.svg
//	go run ./cmd/logo serve
package main

//go:generate go run . -art art/aquatic.txt -label aquatic -accent "#ff5f00" -fg "#e4e4e4" -px 4 -py 3 -o ../../docs/logo-dark.svg
//go:generate go run . -art art/aquatic.txt -label aquatic -accent "#e65600" -fg "#1c1c1c" -bg "#e4e4e4" -px 4 -py 3 -o ../../docs/logo-light.svg
//go:generate go run . -accent "#ff5f00" -fg "#e4e4e4" -px 4 -py 3 -o ../../../flowcode/docs/logo-dark.svg
//go:generate go run . -accent "#e65600" -fg "#1c1c1c" -bg "#e4e4e4" -px 4 -py 3 -o ../../../flowcode/docs/logo-light.svg
//go:generate go run . -art art/fluid.txt -label fluid -accent "#ff5f00" -letter 3 -fg "#e4e4e4" -px 4 -py 3 -o ../../../fluid/docs/logo-dark.svg
//go:generate go run . -art art/fluid.txt -label fluid -accent "#e65600" -letter 3 -fg "#1c1c1c" -bg "#e4e4e4" -px 4 -py 3 -o ../../../fluid/docs/logo-light.svg
//go:generate go run . -art art/sqlite.txt -label sqlite -accent "#ff5f00" -fg "#e4e4e4" -px 4 -py 3 -o ../../../sqlite/docs/logo-dark.svg
//go:generate go run . -art art/sqlite.txt -label sqlite -accent "#e65600" -fg "#1c1c1c" -bg "#e4e4e4" -px 4 -py 3 -o ../../../sqlite/docs/logo-light.svg

import (
	"bytes"
	"context"
	_ "embed"
	"flag"
	"fmt"
	"maps"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/unaquatic/aquatic/internal/cmdline"
)

// FlowArt is flow's logo in half blocks, which the command draws unless
// -art names other art: product.Logo in flow, 30 columns wide and 8 rows
// tall, over a blank top pixel row that drawing crops off.
//
//go:embed art/flow.txt
var FlowArt string

// FlowName names flow's logo for screen readers, unless -label names it.
const FlowName = "flow"

// PrimaryColor is flow's orange, which draws the first letter of its logo:
// product.PrimaryColor in flow.
const PrimaryColor = "#ff5f00"

// A DocsLogo is a logo that a README shows, which go generate writes.
type DocsLogo struct {
	// Path is the image file, relative to this package's directory.
	Path string
	// Art is the file holding the art the image draws, relative to this
	// package's directory, or "" for FlowArt.
	Art string
	// Label names the image for screen readers, or is "" for FlowName.
	Label string
	// Options are the options by flag name that the image sets apart from the
	// defaults.
	Options url.Values
}

// lightAccent is the orange letter's color in the README logos shown in light
// mode: PrimaryColor at 90% of its HSV value, of the same hue. On
// the light grey background it has a contrast of 2.9:1, where the primary
// color has 2.4:1.
const lightAccent = "#e65600"

// darkOptions and lightOptions are the options of the README logos shown in
// dark and in light mode.
var (
	darkOptions = url.Values{"accent": {PrimaryColor}, "fg": {"#e4e4e4"},
		"px": {"4"}, "py": {"3"}}
	lightOptions = url.Values{"accent": {lightAccent}, "fg": {"#1c1c1c"}, "bg": {"#e4e4e4"},
		"px": {"4"}, "py": {"3"}}
)

// OnLetter returns the options o with the accent on letter n, counted from 1
// at the left.
func OnLetter(o url.Values, n int) url.Values {
	o = maps.Clone(o)
	o.Set("letter", strconv.Itoa(n))
	return o
}

// docsLogos are the README logos that go generate writes.
var docsLogos = []DocsLogo{
	{Path: "../../docs/logo-dark.svg", Art: "art/aquatic.txt", Label: "aquatic", Options: darkOptions},
	{Path: "../../docs/logo-light.svg", Art: "art/aquatic.txt", Label: "aquatic", Options: lightOptions},
	{Path: "../../../flowcode/docs/logo-dark.svg", Options: darkOptions},
	{Path: "../../../flowcode/docs/logo-light.svg", Options: lightOptions},
	{Path: "../../../fluid/docs/logo-dark.svg", Art: "art/fluid.txt", Label: "fluid", Options: OnLetter(darkOptions, 3)},
	{Path: "../../../fluid/docs/logo-light.svg", Art: "art/fluid.txt", Label: "fluid", Options: OnLetter(lightOptions, 3)},
	{Path: "../../../sqlite/docs/logo-dark.svg", Art: "art/sqlite.txt", Label: "sqlite", Options: darkOptions},
	{Path: "../../../sqlite/docs/logo-light.svg", Art: "art/sqlite.txt", Label: "sqlite", Options: lightOptions},
}

func main() {
	cmdline.Main(context.Background(), rootCmd)
}

// ArtFlag binds the flag naming the art to draw to art.
func ArtFlag(fs *flag.FlagSet, art *string) {
	fs.StringVar(art, "art", "", "Draw the half-block art in `file` instead of flow's logo")
}

// LoadGrid returns the pixels of the art in the file at path, or of
// FlowArt when path is empty.
func LoadGrid(path string) (Grid, error) {
	if path == "" {
		return ParseGrid(FlowArt)
	}
	art, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	g, err := ParseGrid(string(art))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return g, nil
}

// RootOptions are the flags of the root command: the options of the image,
// the art it draws, and the file it goes to.
type RootOptions struct {
	Options
	// Art is the file holding the art to draw, or "" for FlowArt.
	Art string
	// Out is the file to write, or "" for standard output.
	Out string
}

var rootCmd = &cmdline.Command[RootOptions]{
	Name: "logo",
	Flags: func(fs *flag.FlagSet, o *RootOptions) {
		BindFlags(fs, &o.Options)
		ArtFlag(fs, &o.Art)
		fs.StringVar(&o.Label, "label", "", "Name the image `text` for screen readers, "+FlowName+" when empty")
		const out = "Write the image to `file` instead of standard output"
		fs.StringVar(&o.Out, "out", "", out)
		fs.StringVar(&o.Out, "o", "", out)
	},
	Run: func(_ context.Context, o RootOptions, _ []string) error {
		return Generate(o)
	},
	Commands: []cmdline.Node{serveCmd},
}

// Generate draws the art of o as o says and writes it to o.Out, or to
// standard output when o.Out is empty.
func Generate(o RootOptions) error {
	if err := o.Validate(); err != nil {
		return err
	}
	g, err := LoadGrid(o.Art)
	if err != nil {
		return err
	}
	if o.Out == "" {
		return WriteSVG(os.Stdout, g, o.Options)
	}
	var buf bytes.Buffer
	if err := WriteSVG(&buf, g, o.Options); err != nil {
		return err
	}
	return os.WriteFile(o.Out, buf.Bytes(), 0o644)
}

// ServeOptions are the flags of the serve command.
type ServeOptions struct {
	Port int
	// Art is the file holding the art to draw, or "" for FlowArt.
	Art string
}

var serveCmd = &cmdline.Command[ServeOptions]{
	Name:  "serve",
	Short: "Serve a page that previews the logo while you change its options (localhost only)",
	Flags: func(fs *flag.FlagSet, o *ServeOptions) {
		fs.IntVar(&o.Port, "port", 4097, "Port to listen on")
		ArtFlag(fs, &o.Art)
	},
	Run: func(ctx context.Context, o ServeOptions, _ []string) error {
		ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		g, err := LoadGrid(o.Art)
		if err != nil {
			return err
		}
		return ListenAndServe(ctx, g, o.Port)
	},
}
