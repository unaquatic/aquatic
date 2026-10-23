package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

// page is the landing page that Handler serves at /.
//
//go:embed index.html
var page []byte

// Handler serves the landing page at /.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page)
	})
	return mux
}

// ListenAndServe serves Handler on port of localhost until ctx is done.
func ListenAndServe(ctx context.Context, port int) error {
	srv := &http.Server{
		Addr:              net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		Handler:           Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	})
	defer stop()
	fmt.Printf("landing page on http://localhost:%d (Ctrl-C stops it)\n", port)
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
