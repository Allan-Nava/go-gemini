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
//
// Beyond a single call: GenerateContentStream streams the answer as an iterator,
// NewChat keeps a multi-turn conversation, WithSystemInstruction and
// WithGenerationConfig set per-client defaults, and transient failures (429, 5xx)
// are retried with exponential backoff according to WithRetry.
//
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

// These values change between releases, so they are not exported: an exported
// constant whose value changes is an incompatible API change. Use Client.Model
// to read the model a client uses.
const (
	// version is the SDK version, sent in the User-Agent header.
	version = "1.0.0"
	// defaultModel is used when WithModel is not given. It follows Google's
	// recommended model and may change in a minor release.
	defaultModel = "gemini-3.8-flash"
	// defaultTimeout bounds one attempt when WithTimeout is not given; see WithTimeout.
	defaultTimeout = 5 * time.Minute
)

const (
	// DefaultBaseURL is the Gemini API host.
	DefaultBaseURL = "https://generativelanguage.googleapis.com"
	// APIKeyEnv is the environment variable read when WithAPIKey is not given.
	APIKeyEnv = "GEMINI_API_KEY"

	userAgent    = "go-gemini/" + version
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
	// ErrInvalidRetryPolicy is returned by New when WithRetry is given an unusable policy.
	ErrInvalidRetryPolicy = errors.New("gogemini: invalid retry policy")
	// ErrRedirectOtherHost is returned when the server redirects to another host or scheme.
	// The redirect is not followed, so the API key is never sent there.
	ErrRedirectOtherHost = errors.New("gogemini: refused redirect to another host")
)

// Client calls the Gemini API. It is safe for concurrent use.
type Client struct {
	apiKey            string
	baseURL           string
	model             string
	timeout           time.Duration
	httpClient        *http.Client
	retry             RetryPolicy
	systemInstruction *Content
	generationConfig  *GenerationConfig
}

// Option configures a Client.
type Option func(*Client)

// WithAPIKey sets the API key. It takes precedence over GEMINI_API_KEY.
func WithAPIKey(key string) Option { return func(c *Client) { c.apiKey = key } }

// WithModel sets the model, for example "gemini-3.8-flash". A "models/" prefix is accepted.
// Without it the client uses Google's currently recommended model (see Client.Model),
// which may change in a minor release; pin a model if your output must not change.
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

// WithTimeout limits one attempt: for Generate and GenerateContent, the request and the
// whole reply; for a stream, the wait until the reply starts, after which only ctx bounds
// it, so a long answer is never cut. Each retry gets the limit again. The default is
// 5 minutes; d must be positive. It also applies with WithHTTPClient, together with that
// client's own Timeout, if any (leave that at 0 for long streams).
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

// WithRetry sets how transient failures are retried; see RetryPolicy. Without it the
// client uses DefaultRetryPolicy. RetryPolicy{MaxAttempts: 1} disables retries.
func WithRetry(p RetryPolicy) Option { return func(c *Client) { c.retry = p } }

// WithSystemInstruction sets a system instruction for every request that does not
// carry its own GenerateContentRequest.SystemInstruction. A blank text sets none.
func WithSystemInstruction(text string) Option {
	return func(c *Client) {
		if strings.TrimSpace(text) == "" {
			c.systemInstruction = nil
			return
		}
		c.systemInstruction = &Content{Parts: []Part{{Text: text}}}
	}
}

// WithGenerationConfig sets the generation parameters for every request that does not
// carry its own GenerateContentRequest.GenerationConfig. The config is copied.
func WithGenerationConfig(cfg GenerationConfig) Option {
	return func(c *Client) {
		cfg.StopSequences = append([]string(nil), cfg.StopSequences...)
		c.generationConfig = &cfg
	}
}

// New returns a Client. It fails with ErrMissingAPIKey when no key is available, and with
// ErrInvalidBaseURL, ErrInvalidTimeout, ErrEmptyModel or ErrInvalidRetryPolicy for invalid options.
func New(opts ...Option) (*Client, error) {
	c := &Client{baseURL: DefaultBaseURL, model: defaultModel, timeout: defaultTimeout, retry: DefaultRetryPolicy()}
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
	if err := c.retry.validate(); err != nil {
		return nil, err
	}
	switch {
	case c.httpClient == nil:
		// No http.Client.Timeout: it would also cut the body of a long stream. The SDK
		// bounds each attempt itself, through the request's context (see WithTimeout).
		c.httpClient = &http.Client{CheckRedirect: sameHostRedirect}
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
		return fmt.Errorf("%w: %w", ErrInvalidBaseURL, err)
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
