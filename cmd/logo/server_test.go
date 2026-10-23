package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseOptions(t *testing.T) {
	o, err := ParseOptions(url.Values{"scale": {"4", "12"}, "fg": {"#ffffff"}, "bg": {""}, "px": {"4"}})
	if err != nil {
		t.Fatal(err)
	}
	if o.Scale != 12 || o.Fg != "#ffffff" || o.Bg != "" || o.PadX != 4 || o.PadY != 3 {
		t.Errorf("options = %+v, want scale 12 from the last value, fg #ffffff, no bg, px 4 and the default py", o)
	}
	for _, q := range []url.Values{
		{"size": {"4"}},
		{"pad": {"4"}},
		{"scale": {"big"}},
		{"gap": {"1"}},
		{"py": {"-1"}},
	} {
		if _, err := ParseOptions(q); err == nil {
			t.Errorf("ParseOptions(%v) succeeded, want an error", q)
		}
	}
}

func TestHandler(t *testing.T) {
	srv := httptest.NewServer(Handler(mustGrid(t, FlowArt)))
	defer srv.Close()
	tests := []struct {
		path        string
		status      int
		contentType string
		body        string // a part of the body
	}{
		{"/", http.StatusOK, "text/html; charset=utf-8", "<title>flow logo</title>"},
		{"/logo.svg?scale=4&bg=", http.StatusOK, "image/svg+xml", `width="144"`},
		{"/logo.svg?size=4", http.StatusBadRequest, "text/plain; charset=utf-8", `no option is named "size"`},
		{"/defaults.json", http.StatusOK, "application/json", `"scale":"8"`},
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
				t.Errorf("body =\n%s\nwant it to hold %q", body, tt.body)
			}
		})
	}
}

// TestPageHasEveryOption checks that the editor has a control for every
// option.
func TestPageHasEveryOption(t *testing.T) {
	for name := range Defaults() {
		if !strings.Contains(string(page), `id="`+name+`"`) {
			t.Errorf("the page has no control for option %q", name)
		}
	}
}
