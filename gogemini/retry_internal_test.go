package gogemini

import (
	"testing"
	"time"
)

func TestRetryWaitBounds(t *testing.T) {
	p := RetryPolicy{MaxAttempts: 10, InitialDelay: 100 * time.Millisecond, MaxDelay: time.Second}
	for n, want := range map[int]time.Duration{1: 100 * time.Millisecond, 2: 200 * time.Millisecond, 3: 400 * time.Millisecond, 4: 800 * time.Millisecond, 5: time.Second, 9: time.Second} {
		for range 200 {
			d, ok := p.wait(n, 0)
			if !ok || d < want/2 || d > want {
				t.Fatalf("wait(%d) = %v, %v; want within [%v, %v]", n, d, ok, want/2, want)
			}
		}
	}
	if d, ok := p.wait(1, 700*time.Millisecond); !ok || d < 700*time.Millisecond {
		t.Errorf("wait with server delay 700ms = %v, %v", d, ok)
	}
	if _, ok := p.wait(1, 2*time.Second); ok {
		t.Error("a server delay above MaxDelay must give up")
	}
}

func TestDefaultClientHasNoHTTPTimeout(t *testing.T) {
	c, err := New(WithAPIKey("k"))
	if err != nil {
		t.Fatal(err)
	}
	// http.Client.Timeout would also cut the body of a long stream.
	if c.httpClient.Timeout != 0 {
		t.Errorf("default http.Client.Timeout = %v, want 0", c.httpClient.Timeout)
	}
	if c.timeout != 5*time.Minute {
		t.Errorf("default attempt timeout = %v, want 5m", c.timeout)
	}
}
