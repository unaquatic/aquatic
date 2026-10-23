// Command catalog serves flow's model catalog over HTTP as
// https://aquatic.dev/catalog/ does: a page at / that browses it, the
// catalog at /catalog.json and its JSON Schema at /schema.json. The catalog
// is the one that the catalog module this repository requires embeds,
// data.Catalog, unless -catalog names a catalog document or -source a
// catalog server to download one from. The catalog repository builds the
// catalog, from models.dev, and releases it; the site takes catalog.json and
// schema.json from its latest release, and the page from here.
//
//	go run ./cmd/catalog serve                                     # browse http://localhost:4100
//	go run ./cmd/catalog serve -catalog catalog.json               # as join writes it in the catalog
//	go run ./cmd/catalog serve -source https://aquatic.dev/catalog -refresh 1h
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/unaquatic/aquatic/internal/cmdline"
)

func main() {
	cmdline.Main(context.Background(), rootCmd)
}

var rootCmd = &cmdline.Command[cmdline.NoOptions]{
	Name:     "catalog",
	Short:    "Serve flow's model catalog with a page that browses it",
	Banner:   "Serve flow's model catalog over HTTP as aquatic.dev/catalog does, with a page that browses it.",
	Commands: []cmdline.Node{serveCmd},
}

// ServeOptions are the flags of the serve command.
type ServeOptions struct {
	// Catalog is a file holding a catalog document, served in place of the
	// embedded catalog.
	Catalog string
	// Source is a catalog server to download the catalog from, at
	// gateway.DocumentPath, in place of the embedded catalog.
	Source string
	Host   string
	Port   int
	// Refresh is how often to load the catalog again; zero never does.
	Refresh time.Duration
}

var serveCmd = &cmdline.Command[ServeOptions]{
	Name:  "serve",
	Short: "Serve the catalog and a page that browses it over HTTP",
	Flags: func(fs *flag.FlagSet, o *ServeOptions) {
		fs.StringVar(&o.Catalog, "catalog", "", "Serve the catalog document in `file`, such as the catalog's join writes, instead of the embedded catalog")
		fs.StringVar(&o.Source, "source", "", "Download the catalog from the catalog server at `url`, such as https://aquatic.dev/catalog, instead of serving the embedded catalog")
		fs.StringVar(&o.Host, "host", "127.0.0.1", "Listen on `address`")
		fs.IntVar(&o.Port, "port", 4100, "Port to listen on")
		fs.DurationVar(&o.Refresh, "refresh", 0, "Load the catalog again every `interval`, such as 1h; 0 never does")
	},
	Exclusive: [][]string{{"catalog", "source"}},
	Run: func(ctx context.Context, o ServeOptions, _ []string) error {
		ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		return Serve(ctx, o, os.Stderr)
	},
}
