package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()
	tests := []struct {
		path        string
		status      int
		contentType string
		body        string // a part of the body
	}{
		{"/", http.StatusOK, "text/html; charset=utf-8", "<title>Aquatic</title>"},
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
