// Command landing serves flow's landing page: a full-screen grid of dots on
// which the ƒlow wordmark's own dots ripple and spark, burst out in every
// direction and gather again, a bar under it with the logo and Install,
// which sticks to the top as the page scrolls, and the slogan and the install
// command beside a terminal that replays real frames of flow's UI. ?adjust in
// the URL shows an Adjust sidebar that changes the dots, their motion and the
// logo; ?effect=sea shows the dots as a sea instead. The page is one
// self-contained file, index.html, with the type16 font and the terminal's
// frames embedded. flow's tests write the frames, to
// internal/terminal/testdata/landing_demo.seq.golden in flow, which must be
// checked out next to this repository to copy them in.
//
//	go run ./cmd/landing              # serve it on http://localhost:4099
//	go run ./cmd/landing -port 5000
//
//	# new frames: write them in flow, then copy them into the page
//	(cd ../flowcode && go test ./internal/terminal/ -run LandingDemo -update)
//	go test ./cmd/landing -update
package main

import (
	"context"
	"flag"
	"os/signal"
	"syscall"

	"github.com/unaquatic/aquatic/internal/cmdline"
)

func main() {
	cmdline.Main(context.Background(), rootCmd)
}

// ServeOptions are the flags of the root command, which serves the page.
type ServeOptions struct {
	Port int
}

var rootCmd = &cmdline.Command[ServeOptions]{
	Name:  "landing",
	Short: "Serve the landing page (localhost only)",
	Flags: func(fs *flag.FlagSet, o *ServeOptions) {
		fs.IntVar(&o.Port, "port", 4099, "Port to listen on")
	},
	Run: func(ctx context.Context, o ServeOptions, _ []string) error {
		ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		return ListenAndServe(ctx, o.Port)
	},
}
