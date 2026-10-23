package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/unaquatic/catalog/data"
	"github.com/unaquatic/catalog/gateway"
)

// Loader loads a catalog to serve.
type Loader func(ctx context.Context) (*gateway.Catalog, error)

// LoaderFor returns the loader that o names: one reading the catalog
// document o.Catalog, one downloading the catalog from the catalog server
// o.Source, or else one taking the embedded catalog, data.Catalog.
func LoaderFor(o ServeOptions) Loader {
	switch {
	case o.Catalog != "":
		return func(context.Context) (*gateway.Catalog, error) {
			return ReadCatalog(o.Catalog)
		}
	case o.Source != "":
		return func(ctx context.Context) (*gateway.Catalog, error) {
			return FetchCatalog(ctx, http.DefaultClient, o.Source)
		}
	default:
		return func(context.Context) (*gateway.Catalog, error) {
			return data.Catalog()
		}
	}
}

// ReadCatalog reads the catalog document in the file at path.
func ReadCatalog(path string) (*gateway.Catalog, error) {
	doc, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c, err := gateway.Decode(doc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// FetchCatalog downloads the catalog document that the catalog server at
// source serves at gateway.DocumentPath.
func FetchCatalog(ctx context.Context, client *http.Client, source string) (*gateway.Catalog, error) {
	url := strings.TrimSuffix(source, "/") + gateway.DocumentPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	doc, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	c, err := gateway.Decode(doc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	return c, nil
}

// Serve loads the catalog as o says and serves it on o.Host and o.Port until
// ctx is done, loading it again every o.Refresh when that is set. It writes
// where it listens to out.
func Serve(ctx context.Context, o ServeOptions, out io.Writer) error {
	load := LoaderFor(o)
	c, err := load(ctx)
	if err != nil {
		return err
	}
	s, err := NewServer(c)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if o.Refresh > 0 {
		wg.Go(func() { Refresh(ctx, o.Refresh, load, s) })
	}
	return ListenAndServe(ctx, net.JoinHostPort(o.Host, strconv.Itoa(o.Port)), s, out)
}

// Refresh loads the catalog with load every interval until ctx is done, and
// makes s serve each catalog it loads. A load that fails leaves s serving
// the catalog it has, and is logged.
func Refresh(ctx context.Context, every time.Duration, load Loader, s *Server) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		c, err := load(ctx)
		if err == nil {
			err = s.Set(c)
		}
		if err != nil {
			slog.Warn("catalog refresh failed; serving the catalog loaded before", "err", err)
		}
	}
}

// ListenAndServe serves s on addr until ctx is done, and writes where it
// listens to out.
func ListenAndServe(ctx context.Context, addr string, s *Server, out io.Writer) error {
	srv := &http.Server{Addr: addr, Handler: s, ReadHeaderTimeout: 10 * time.Second}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()
	c := s.Catalog()
	fmt.Fprintf(out, "catalog browser on http://%s with %d models from %s (Ctrl-C stops it)\n",
		ln.Addr(), len(c.Models), c.Source)
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
