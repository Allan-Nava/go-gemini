package gogemini_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Allan-Nava/go-gemini/gogemini"
)

// fakeGemini stands in for the real API so the examples run offline.
func fakeGemini() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"Goroutines are lightweight threads managed by the Go runtime."}]}}]}`)
	}))
}

func ExampleNew() {
	// In real code, leave out WithAPIKey and set GEMINI_API_KEY instead.
	client, err := gogemini.New(
		gogemini.WithAPIKey("YOUR_API_KEY"),
		gogemini.WithModel("gemini-3.8-flash"),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(client.Model())
	// Output: gemini-3.8-flash
}

func ExampleClient_GenerateContent() {
	srv := fakeGemini()
	defer srv.Close()

	client, err := gogemini.New(gogemini.WithAPIKey("YOUR_API_KEY"), gogemini.WithBaseURL(srv.URL))
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.GenerateContent(context.Background(), "Explain goroutines in one sentence")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp.Text())
	// Output: Goroutines are lightweight threads managed by the Go runtime.
}

func ExampleClient_Generate() {
	srv := fakeGemini()
	defer srv.Close()

	client, err := gogemini.New(gogemini.WithAPIKey("YOUR_API_KEY"), gogemini.WithBaseURL(srv.URL))
	if err != nil {
		log.Fatal(err)
	}
	// Build the contents yourself, for example to replay a conversation.
	resp, err := client.Generate(context.Background(), &gogemini.GenerateContentRequest{
		Contents: []gogemini.Content{
			{Role: "user", Parts: []gogemini.Part{{Text: "What are goroutines?"}}},
			{Role: "model", Parts: []gogemini.Part{{Text: "Lightweight threads."}}},
			{Role: "user", Parts: []gogemini.Part{{Text: "Say it as one full sentence."}}},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp.Text())
	// Output: Goroutines are lightweight threads managed by the Go runtime.
}

func ExampleAPIError() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":{"code":429,"message":"Quota exceeded","status":"RESOURCE_EXHAUSTED"}}`)
	}))
	defer srv.Close()

	client, _ := gogemini.New(gogemini.WithAPIKey("YOUR_API_KEY"), gogemini.WithBaseURL(srv.URL))
	_, err := client.GenerateContent(context.Background(), "hi")

	var apiErr *gogemini.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests {
		fmt.Println("rate limited:", apiErr.Status)
	}
	// Output: rate limited: RESOURCE_EXHAUSTED
}

func ExampleWithHTTPClient() {
	// Your own client keeps transport, TLS and timeout settings under your control.
	// It is copied, and given the SDK's same-host redirect rule, if CheckRedirect is nil.
	hc := &http.Client{Timeout: 10 * time.Second}
	client, err := gogemini.New(gogemini.WithAPIKey("YOUR_API_KEY"), gogemini.WithHTTPClient(hc))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(client.Model() != "")
	// Output: true
}
