package httpclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u.Hostname()
}

func TestClientRejectsUntrustedRedirect(t *testing.T) {
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://evil.invalid/redirect", http.StatusFound)
	}))
	defer public.Close()
	c := New(Options{AllowedHosts: []string{hostOf(public.URL)}, MaxBytes: 1024, AllowPrivateNetworks: true, Interval: time.Nanosecond})
	_, err := c.Get(context.Background(), public.URL)
	if !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("err=%v", err)
	}
}

func TestClientRejectsOversizedBody(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("123456")) }))
	defer s.Close()
	c := New(Options{AllowedHosts: []string{hostOf(s.URL)}, MaxBytes: 5, AllowPrivateNetworks: true, Interval: time.Nanosecond})
	_, err := c.Get(context.Background(), s.URL)
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("err=%v", err)
	}
}

func TestClientRetries503(t *testing.T) {
	count := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count < 2 {
			w.WriteHeader(503)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer s.Close()
	c := New(Options{AllowedHosts: []string{hostOf(s.URL)}, MaxBytes: 100, Retries: 1, AllowPrivateNetworks: true, Interval: time.Nanosecond})
	got, err := c.Get(context.Background(), s.URL)
	if err != nil || string(got.Body) != "ok" || count != 2 {
		t.Fatalf("got=%+v err=%v count=%d", got, err, count)
	}
}
