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
package gogemini

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the Gemini API host.
	DefaultBaseURL = "https://generativelanguage.googleapis.com"
	// DefaultModel is used when WithModel is not given.
	DefaultModel = "gemini-3.8-flash"
	// DefaultTimeout bounds each request when WithHTTPClient and WithTimeout are not given.
	DefaultTimeout = 60 * time.Second
	// APIKeyEnv is the environment variable read when WithAPIKey is not given.
	APIKeyEnv = "GEMINI_API_KEY"
)

// ErrMissingAPIKey is returned by New when no key was passed and GEMINI_API_KEY is empty.
var ErrMissingAPIKey = errors.New("gogemini: missing API key (use WithAPIKey or set " + APIKeyEnv + ")")

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
	return func(c *Client) { c.model = strings.TrimPrefix(model, "models/") }
}

// WithBaseURL points the client at another host, such as a proxy or a test server.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") } }

// WithHTTPClient replaces the HTTP client, so transport, TLS and timeouts stay under the caller's control.
func WithHTTPClient(hc *http.Client) Option { return func(c *Client) { c.httpClient = hc } }

// WithTimeout sets the timeout of the default HTTP client. It has no effect with WithHTTPClient.
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

// New returns a Client. It fails with ErrMissingAPIKey when no key is available.
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
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: c.timeout}
	}
	return c, nil
}

// Model returns the model the client sends requests to.
func (c *Client) Model() string { return c.model }
