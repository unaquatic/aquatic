package main

import (
	"errors"
	"flag"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// update copies the demo frames golden file into index.html.
var update = flag.Bool("update", false, "copy the demo frames golden file into index.html")

func TestHandler(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()
	tests := []struct {
		path        string
		status      int
		contentType string
		body        string // a part of the body
	}{
		{"/", http.StatusOK, "text/html; charset=utf-8", "<title>Flow</title>"},
		{"/missing", http.StatusNotFound, "text/plain; charset=utf-8", "not found"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			res, err := http.Get(srv.URL + tt.path)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != tt.status || res.Header.Get("Content-Type") != tt.contentType {
				t.Errorf("status %d, type %q; want %d, %q", res.StatusCode, res.Header.Get("Content-Type"), tt.status, tt.contentType)
			}
			if !strings.Contains(string(body), tt.body) {
				t.Errorf("body holds no %q", tt.body)
			}
		})
	}
}

// TestPageIsSelfContained checks that the page embeds its font, so that it
// needs nothing but itself.
func TestPageIsSelfContained(t *testing.T) {
	if !strings.Contains(string(page), "url(data:font/ttf;base64,") {
		t.Error("the page does not embed the type16 font")
	}
}

// DemoGolden is the golden file of the frames that the page's terminal
// replays, which TestLandingDemoFrames in flow's internal/terminal writes, in
// flow's checkout next to this repository.
const DemoGolden = "../../../flowcode/internal/terminal/testdata/landing_demo.seq.golden"

// The page holds DemoGolden between DemoOpen and DemoClose.
const (
	DemoOpen  = "<script type=\"text/plain\" id=\"demo-frames\">\n"
	DemoClose = "</script>"
)

// TestPageHoldsDemoFrames checks that the page holds the current DemoGolden,
// so that its terminal replays flow's UI as it is. -update copies it in. It
// skips when flow is not checked out next to this repository.
func TestPageHoldsDemoFrames(t *testing.T) {
	want, err := os.ReadFile(DemoGolden)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("%s is not checked out", DemoGolden)
	}
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(want)), "</script") {
		t.Fatalf("%s holds </script, which would end the page's block early", DemoGolden)
	}
	before, rest, ok := strings.Cut(string(page), DemoOpen)
	if !ok {
		t.Fatal("the page has no demo-frames block")
	}
	got, after, ok := strings.Cut(rest, DemoClose)
	if !ok {
		t.Fatal("the page's demo-frames block does not end")
	}
	if *update {
		if err := os.WriteFile("index.html", []byte(before+DemoOpen+string(want)+DemoClose+after), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if got != string(want) {
		t.Errorf("the page's demo frames differ from %s; run go test ./cmd/landing -update", DemoGolden)
	}
}
