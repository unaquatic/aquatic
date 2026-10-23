package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"
)

// page is the editor that Handler serves at /.
//
//go:embed logo.html
var page []byte

// ParseOptions returns the default options with the ones that q names by
// flag, such as scale=12 or fg=#ffffff, set. It fails on a name that is no
// flag, a value that does not parse, and options out of range.
func ParseOptions(q url.Values) (Options, error) {
	var o Options
	fs := flag.NewFlagSet("logo", flag.ContinueOnError)
	BindFlags(fs, &o)
	for _, name := range slices.Sorted(maps.Keys(q)) {
		if fs.Lookup(name) == nil {
			return Options{}, fmt.Errorf("no option is named %q", name)
		}
		values := q[name]
		if err := fs.Set(name, values[len(values)-1]); err != nil {
			return Options{}, fmt.Errorf("%s: %w", name, err)
		}
	}
	return o, o.Validate()
}

// Defaults returns the default value of every option by flag name, as text.
func Defaults() map[string]string {
	fs := flag.NewFlagSet("logo", flag.ContinueOnError)
	BindFlags(fs, new(Options))
	defaults := make(map[string]string)
	fs.VisitAll(func(f *flag.Flag) { defaults[f.Name] = f.DefValue })
	return defaults
}

// Handler serves the editor page at /, the image of g at /logo.svg, with the
// options its query names by flag, and the default options at
// /defaults.json.
func Handler(g Grid) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page)
	})
	mux.HandleFunc("GET /logo.svg", func(w http.ResponseWriter, r *http.Request) {
		o, err := ParseOptions(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "no-store")
		WriteSVG(w, g, o)
	})
	mux.HandleFunc("GET /defaults.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Defaults())
	})
	return mux
}

// ListenAndServe serves Handler(g) on port of localhost until ctx is done.
func ListenAndServe(ctx context.Context, g Grid, port int) error {
	srv := &http.Server{
		Addr:    net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		Handler: Handler(g),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	fmt.Printf("logo editor on http://localhost:%d (Ctrl-C stops it)\n", port)
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
