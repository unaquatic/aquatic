package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/unaquatic/catalog/data"
	"github.com/unaquatic/catalog/gateway"
)

// testCatalog returns a catalog with no providers or models, from source.
func testCatalog(source string) *gateway.Catalog {
	return &gateway.Catalog{Version: gateway.Version, Source: source}
}

// testServer returns a Server serving testCatalog("test").
func testServer(t *testing.T) *Server {
	t.Helper()
	s, err := NewServer(testCatalog("test"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestLoaderFor checks that the loader reads a catalog document, refusing
// one of another version, downloads one from a catalog server, and takes
// the embedded catalog by default.
func TestLoaderFor(t *testing.T) {
	dir := t.ArtifactDir()
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, fmt.Appendf(nil, `{"version": %d, "source": "file", "providers": {}, "models": {}}`, gateway.Version), 0o644); err != nil {
		t.Fatal(err)
	}
	if c, err := LoaderFor(ServeOptions{Catalog: good})(t.Context()); err != nil || c.Source != "file" {
		t.Errorf("load %s = %+v, %v", good, c, err)
	}
	old := filepath.Join(dir, "old.json")
	if err := os.WriteFile(old, []byte(`{"version": 4}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoaderFor(ServeOptions{Catalog: old})(t.Context()); !errors.Is(err, gateway.ErrVersion) {
		t.Errorf("load %s: err = %v, want ErrVersion", old, err)
	}

	srv := httptest.NewServer(testServer(t))
	defer srv.Close()
	if c, err := LoaderFor(ServeOptions{Source: srv.URL + "/"})(t.Context()); err != nil || c.Source != "test" {
		t.Errorf("load from %s = %+v, %v", srv.URL, c, err)
	}
	if _, err := LoaderFor(ServeOptions{Source: srv.URL + "/missing"})(t.Context()); err == nil {
		t.Errorf("load from %s/missing: no error, want one", srv.URL)
	}

	want, err := data.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if c, err := LoaderFor(ServeOptions{})(t.Context()); err != nil || c != want {
		t.Errorf("load by default = %v, want the embedded catalog", err)
	}
}

func TestRefresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, err := NewServer(testCatalog("0"))
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		load := func(context.Context) (*gateway.Catalog, error) {
			calls++
			if calls == 2 {
				return nil, errors.New("source down")
			}
			return testCatalog(strconv.Itoa(calls)), nil
		}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() {
			Refresh(ctx, time.Hour, load, s)
			close(done)
		}()

		for _, want := range []string{"1", "1", "3"} { // the second load fails
			time.Sleep(time.Hour)
			synctest.Wait()
			if got := s.Catalog().Source; got != want {
				t.Errorf("after %d refreshes the catalog is %q, want %q", calls, got, want)
			}
		}
		cancel()
		<-done
	})
}

// lines is a writer that sends what each Write writes.
type lines chan string

func (l lines) Write(p []byte) (int, error) {
	l <- string(p)
	return len(p), nil
}

func TestListenAndServe(t *testing.T) {
	s := testServer(t)
	ctx, cancel := context.WithCancel(t.Context())
	out := make(lines, 1)
	served := make(chan error, 1)
	go func() { served <- ListenAndServe(ctx, "127.0.0.1:0", s, out) }()

	line := <-out
	_, rest, ok := strings.Cut(line, "http://")
	addr, _, _ := strings.Cut(rest, " ")
	if !ok || !strings.Contains(line, "with 0 models from test") {
		t.Fatalf("server says %q", line)
	}
	resp, err := http.Get("http://" + addr + "/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"source":"test"`) {
		t.Errorf("GET /catalog.json: status %d, body %s", resp.StatusCode, body)
	}
	cancel()
	if err := <-served; err != nil {
		t.Errorf("ListenAndServe after cancel: %v", err)
	}
}

// TestServer checks that the server serves what the site does, the page,
// the catalog and its schema, and nothing else.
func TestServer(t *testing.T) {
	srv := httptest.NewServer(testServer(t))
	defer srv.Close()
	tests := []struct {
		path, wantType, wantBody string
		wantStatus               int
	}{
		{"/", "text/html; charset=utf-8", "<title>Catalog</title>", http.StatusOK},
		{"/catalog.json", "application/json", `"source":"test"`, http.StatusOK},
		{"/schema.json", "application/schema+json", `"$id": "https://aquatic.dev/catalog/schema.json"`, http.StatusOK},
		{"/v1/providers/opencode", "text/plain; charset=utf-8", "404 page not found", http.StatusNotFound},
		{"/index.html", "text/plain; charset=utf-8", "404 page not found", http.StatusNotFound},
	}
	for _, tt := range tests {
		resp, err := srv.Client().Get(srv.URL + tt.path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != tt.wantStatus || resp.Header.Get("Content-Type") != tt.wantType || !strings.Contains(string(body), tt.wantBody) {
			t.Errorf("GET %s: status %d, type %q, body has %q: %v", tt.path, resp.StatusCode,
				resp.Header.Get("Content-Type"), tt.wantBody, strings.Contains(string(body), tt.wantBody))
		}
	}
}

// TestServerDocument checks that the server serves the catalog under an
// ETag, answers a request naming it with 304 Not Modified, and serves the
// catalog Set gives it, under a new ETag.
func TestServerDocument(t *testing.T) {
	s := testServer(t)
	srv := httptest.NewServer(s)
	defer srv.Close()
	get := func(etag string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+gateway.DocumentPath, nil)
		if err != nil {
			t.Fatal(err)
		}
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp, body
	}
	resp, body := get("")
	etag := resp.Header.Get("ETag")
	want, _ := json.Marshal(s.Catalog())
	if resp.StatusCode != http.StatusOK || etag == "" || string(body) != string(want) {
		t.Fatalf("status %d, etag %q, body matches %v", resp.StatusCode, etag, string(body) == string(want))
	}
	if resp, body := get(etag); resp.StatusCode != http.StatusNotModified || len(body) != 0 {
		t.Errorf("conditional GET: status %d, %d bytes; want 304 and none", resp.StatusCode, len(body))
	}
	if err := s.Set(testCatalog("elsewhere")); err != nil {
		t.Fatal(err)
	}
	resp, body = get(etag)
	if c, err := gateway.Decode(body); resp.StatusCode != http.StatusOK || resp.Header.Get("ETag") == etag || err != nil || c.Source != "elsewhere" {
		t.Errorf("after Set: status %d, etag %q, %v; want the new catalog under a new ETag", resp.StatusCode, resp.Header.Get("ETag"), err)
	}
}

// TestPageIsSelfContained checks that the page embeds its font, type16, a
// copy of the one flow's landing page embeds, so that it needs nothing but
// itself.
func TestPageIsSelfContained(t *testing.T) {
	if !strings.Contains(string(page), `font-family: "type16"`) || !strings.Contains(string(page), "url(data:font/ttf;base64,") {
		t.Error("the page does not embed the type16 font")
	}
}
