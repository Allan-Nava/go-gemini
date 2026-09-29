// Package gogemini is a small client for the Google Gemini API
// (https://ai.google.dev/api), built on net/http.
//
//	client, err := gogemini.New(gogemini.WithAPIKey(key))
//	if err != nil {
//		return err
//	}
//	resp, err := client.GenerateContent(ctx, "Explain goroutines in one sentence")
//	if err != nil {
//		return err
//	}
//	fmt.Println(resp.Text())
//
// Without WithAPIKey the key is read from the GEMINI_API_KEY environment variable.
// The key is sent in the x-goog-api-key header, only over HTTPS (plain HTTP is
// accepted for loopback test servers), and never to a host other than the base URL:
// redirects to another host are refused with ErrRedirectOtherHost.
package gogemini

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// Version is the SDK version, sent in the User-Agent header.
	Version = "0.2.1"
	// DefaultBaseURL is the Gemini API host.
	DefaultBaseURL = "https://generativelanguage.googleapis.com"
	// DefaultModel is used when WithModel is not given.
	DefaultModel = "gemini-3.8-flash"
	// DefaultTimeout bounds each request when WithHTTPClient and WithTimeout are not given.
	DefaultTimeout = 60 * time.Second
	// APIKeyEnv is the environment variable read when WithAPIKey is not given.
	APIKeyEnv = "GEMINI_API_KEY"

	userAgent    = "go-gemini/" + Version
	maxRedirects = 10
)

var (
	// ErrMissingAPIKey is returned by New when no key was passed and GEMINI_API_KEY is empty.
	ErrMissingAPIKey = errors.New("gogemini: missing API key (use WithAPIKey or set " + APIKeyEnv + ")")
	// ErrInvalidBaseURL is returned by New for a base URL that is malformed, has no host,
	// or uses plain HTTP towards a host that is not loopback.
	ErrInvalidBaseURL = errors.New("gogemini: invalid base URL")
	// ErrInvalidTimeout is returned by New when WithTimeout is given a duration <= 0.
	ErrInvalidTimeout = errors.New("gogemini: timeout must be positive")
	// ErrEmptyModel is returned by New when WithModel is given an empty name.
	ErrEmptyModel = errors.New("gogemini: empty model name")
	// ErrEmptyRequest is returned, without sending anything, for a nil request,
	// a request with no contents, or an empty prompt.
	ErrEmptyRequest = errors.New("gogemini: empty request")
	// ErrRedirectOtherHost is returned when the server redirects to another host or scheme.
	// The redirect is not followed, so the API key is never sent there.
	ErrRedirectOtherHost = errors.New("gogemini: refused redirect to another host")
)

// Client calls the Gemini API. It is safe for concurrent use.
type Client struct {
	apiKey     string
	baseURL    string
	model      string
	timeout    time.Duration
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithAPIKey sets the API key. It takes precedence over GEMINI_API_KEY.
func WithAPIKey(key string) Option { return func(c *Client) { c.apiKey = key } }

// WithModel sets the model, for example "gemini-3.8-flash". A "models/" prefix is accepted.
func WithModel(model string) Option {
	return func(c *Client) { c.model = strings.TrimPrefix(strings.TrimSpace(model), "models/") }
}

// WithBaseURL points the client at another host, such as a proxy or a test server.
// It must be HTTPS, except for loopback hosts (127.0.0.1, ::1, localhost).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") } }

// WithHTTPClient replaces the HTTP client, so transport, TLS and timeouts stay under the caller's control.
// If its CheckRedirect is nil, the client is copied and given the same same-host redirect policy
// as the default one; a CheckRedirect you set yourself is kept as it is.
func WithHTTPClient(hc *http.Client) Option { return func(c *Client) { c.httpClient = hc } }

// WithTimeout sets the timeout of the default HTTP client. It must be positive, and has
// no effect with WithHTTPClient.
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

// New returns a Client. It fails with ErrMissingAPIKey when no key is available, and with
// ErrInvalidBaseURL, ErrInvalidTimeout or ErrEmptyModel for invalid options.
func New(opts ...Option) (*Client, error) {
	c := &Client{baseURL: DefaultBaseURL, model: DefaultModel, timeout: DefaultTimeout}
	for _, opt := range opts {
		opt(c)
	}
	if c.apiKey == "" {
		c.apiKey = os.Getenv(APIKeyEnv)
	}
	if c.apiKey == "" {
		return nil, ErrMissingAPIKey
	}
	if c.model == "" {
		return nil, ErrEmptyModel
	}
	if c.timeout <= 0 {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTimeout, c.timeout)
	}
	if err := checkBaseURL(c.baseURL); err != nil {
		return nil, err
	}
	switch {
	case c.httpClient == nil:
		c.httpClient = &http.Client{Timeout: c.timeout, CheckRedirect: sameHostRedirect}
	case c.httpClient.CheckRedirect == nil:
		hc := *c.httpClient
		hc.CheckRedirect = sameHostRedirect
		c.httpClient = &hc
	}
	return c, nil
}

// Model returns the model the client sends requests to.
func (c *Client) Model() string { return c.model }

func checkBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBaseURL, err)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: %q has no host", ErrInvalidBaseURL, raw)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopback(u.Hostname()) {
			return nil
		}
		return fmt.Errorf("%w: %q uses plain HTTP, which would send the API key in clear", ErrInvalidBaseURL, raw)
	default:
		return fmt.Errorf("%w: unsupported scheme %q", ErrInvalidBaseURL, u.Scheme)
	}
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// sameHostRedirect follows redirects only within the scheme and host of the first request:
// net/http copies custom headers such as x-goog-api-key to any redirect target.
func sameHostRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("gogemini: stopped after %d redirects", maxRedirects)
	}
	first := via[0].URL
	if req.URL.Scheme != first.Scheme || req.URL.Host != first.Host {
		return ErrRedirectOtherHost
	}
	return nil
}
