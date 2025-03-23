package reefd

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer(t *testing.T) {
	ctx := context.Background()
	s, err := newServer(ctx, &Config{})
	if err != nil {
		t.Fatal("new server: ", err)
	}

	httpServer := httptest.NewServer(s)
	defer httpServer.Close()

	client := httpServer.Client()
	resp, err := client.Get(httpServer.URL)
	if err != nil {
		t.Fatal("get url: ", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status code %d, want %d", resp.StatusCode, http.StatusOK)
	}

	want := "Hello, World!"
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if string(got) != want {
		t.Errorf("got body %q, want %q", got, want)
	}
}
