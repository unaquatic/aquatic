// Command aquatic serves aquatic.dev's landing page: a full-screen sea of
// dots with the aquatic wordmark afloat in it, and a bar under it with the
// logo and links to flow, the catalog and GitHub. The page is one
// self-contained file, index.html, with the type16 font embedded.
//
//	go run .              # serve it on http://localhost:4095
//	go run . -port 5000
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// ServeOptions are the command's flags.
type ServeOptions struct {
	Port int
}

func main() {
	var o ServeOptions
	flag.IntVar(&o.Port, "port", 4095, "Port to listen on")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := ListenAndServe(ctx, o.Port); err != nil {
		fmt.Fprintln(os.Stderr, "aquatic:", err)
		os.Exit(1)
	}
}
