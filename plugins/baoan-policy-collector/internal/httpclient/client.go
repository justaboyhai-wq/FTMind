package httpclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrHostNotAllowed = errors.New("host not allowed")
	ErrBodyTooLarge   = errors.New("response body too large")
)

type Options struct {
	AllowedHosts         []string
	MaxBytes             int64
	Interval             time.Duration
	Timeout              time.Duration
	Retries              int
	AllowPrivateNetworks bool
	Transport            http.RoundTripper
}

type Response struct {
	URL          string
	StatusCode   int
	ContentType  string
	ETag         string
	LastModified string
	RetryAfter   string
	Body         []byte
	FetchedAt    time.Time
	SHA256       string
}

type Client struct {
	opts        Options
	http        *http.Client
	mu          sync.Mutex
	lastRequest time.Time
}

type redirectError struct{ err error }

func (e redirectError) Error() string { return e.err.Error() }
func (e redirectError) Unwrap() error { return e.err }

func New(opts Options) *Client {
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = 10 << 20
	}
	if opts.Interval <= 0 {
		opts.Interval = time.Second
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.Retries < 0 {
		opts.Retries = 0
	}
	c := &Client{opts: opts}
	c.http = &http.Client{Timeout: opts.Timeout, Transport: opts.Transport}
	c.http.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return http.ErrUseLastResponse
		}
		if err := c.validateURL(req.Context(), req.URL); err != nil {
			return redirectError{err}
		}
		return nil
	}
	return c
}

func (c *Client) Get(ctx context.Context, rawURL string) (Response, error) {
	return c.get(ctx, rawURL, c.opts.MaxBytes)
}

func (c *Client) GetWithMaxBytes(ctx context.Context, rawURL string, maxBytes int64) (Response, error) {
	if maxBytes <= 0 {
		maxBytes = c.opts.MaxBytes
	}
	return c.get(ctx, rawURL, maxBytes)
}

func (c *Client) get(ctx context.Context, rawURL string, maxBytes int64) (Response, error) {
	if err := c.validateURL(ctx, mustParse(rawURL)); err != nil {
		return Response{}, err
	}
	var last error
	for attempt := 0; attempt <= c.opts.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return Response{}, ctx.Err()
			case <-time.After(time.Duration(1<<(attempt-1)) * time.Second):
			}
		}
		if err := c.wait(ctx); err != nil {
			return Response{}, err
		}
		resp, err := c.do(ctx, rawURL, maxBytes)
		if err == nil && !retryStatus(resp.StatusCode) {
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return Response{}, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
			}
			return resp, nil
		}
		if err != nil {
			last = err
			if !retryableError(err) {
				break
			}
		} else {
			last = fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
		}
		if err == nil && resp.StatusCode == http.StatusTooManyRequests {
			if wait := retryAfter(resp); wait > 0 {
				select {
				case <-ctx.Done():
					return Response{}, ctx.Err()
				case <-time.After(wait):
				}
			}
		}
		if err != nil && (errors.Is(err, ErrHostNotAllowed) || errors.Is(err, ErrBodyTooLarge)) {
			break
		}
	}
	return Response{}, last
}

func (c *Client) do(ctx context.Context, rawURL string, maxBytes int64) (Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("User-Agent", "FMind-BaoanPolicyCollector/1.0")
	r, err := c.http.Do(req)
	if err != nil {
		return Response{}, unwrapRedirect(err)
	}
	defer r.Body.Close()
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return Response{URL: r.Request.URL.String(), StatusCode: r.StatusCode, ETag: r.Header.Get("ETag"), LastModified: r.Header.Get("Last-Modified"), RetryAfter: r.Header.Get("Retry-After")}, nil
	}
	limited := io.LimitReader(r.Body, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return Response{}, err
	}
	if int64(len(body)) > maxBytes {
		return Response{}, ErrBodyTooLarge
	}
	sum := sha256.Sum256(body)
	return Response{URL: r.Request.URL.String(), StatusCode: r.StatusCode, ContentType: r.Header.Get("Content-Type"), ETag: r.Header.Get("ETag"), LastModified: r.Header.Get("Last-Modified"), RetryAfter: r.Header.Get("Retry-After"), Body: body, FetchedAt: time.Now().UTC(), SHA256: hex.EncodeToString(sum[:])}, nil
}

func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	delay := c.opts.Interval - time.Since(c.lastRequest)
	c.mu.Unlock()
	if delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	c.mu.Lock()
	c.lastRequest = time.Now()
	c.mu.Unlock()
	return nil
}

func (c *Client) validateURL(ctx context.Context, u *url.URL) error {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Hostname() == "" {
		return ErrHostNotAllowed
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	allowed := false
	for _, item := range c.opts.AllowedHosts {
		if strings.EqualFold(strings.TrimSuffix(item, "."), host) {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("%w: %s", ErrHostNotAllowed, host)
	}
	if c.opts.AllowPrivateNetworks {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return err
		}
		for _, candidate := range ips {
			if isPrivate(candidate) {
				return fmt.Errorf("%w: private address", ErrHostNotAllowed)
			}
		}
		return nil
	}
	if isPrivate(ip) {
		return fmt.Errorf("%w: private address", ErrHostNotAllowed)
	}
	return nil
}

func isPrivate(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsLinkLocalMulticast()
}
func mustParse(raw string) *url.URL { u, _ := url.Parse(raw); return u }
func unwrapRedirect(err error) error {
	var e redirectError
	if errors.As(err, &e) {
		return e.err
	}
	return err
}
func retryStatus(status int) bool {
	return status == 429 || status == 502 || status == 503 || status == 504
}
func retryAfter(r Response) time.Duration {
	if r.StatusCode != 429 {
		return 0
	}
	if n, err := strconv.Atoi(strings.TrimSpace(r.RetryAfter)); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return 0
}

func retryableError(err error) bool {
	var nerr net.Error
	return errors.As(err, &nerr) && (nerr.Timeout() || nerr.Temporary())
}
