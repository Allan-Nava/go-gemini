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

	client, _ := gogemini.New(
		gogemini.WithAPIKey("YOUR_API_KEY"),
		gogemini.WithBaseURL(srv.URL),
		// A 429 is retried by default; one attempt shows the error straight away.
		gogemini.WithRetry(gogemini.RetryPolicy{MaxAttempts: 1}),
	)
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

func ExampleClient_GenerateContentStream() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, piece := range []string{"Go", "rou", "tines"} {
			fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":%q}]}}]}\n\n", piece)
		}
	}))
	defer srv.Close()

	client, err := gogemini.New(gogemini.WithAPIKey("YOUR_API_KEY"), gogemini.WithBaseURL(srv.URL))
	if err != nil {
		log.Fatal(err)
	}
	for chunk, err := range client.GenerateContentStream(context.Background(), "Say goroutines") {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(chunk.Text())
	}
	fmt.Println()
	// Output: Goroutines
}

func ExampleChat() {
	srv := fakeGemini()
	defer srv.Close()

	client, err := gogemini.New(gogemini.WithAPIKey("YOUR_API_KEY"), gogemini.WithBaseURL(srv.URL))
	if err != nil {
		log.Fatal(err)
	}
	chat := client.NewChat()
	if _, err := chat.Send(context.Background(), "What are goroutines?"); err != nil {
		log.Fatal(err)
	}
	// The next Send carries the first question and its answer.
	fmt.Println(len(chat.History()), "contents in the history")
	// Output: 2 contents in the history
}

func ExampleWithGenerationConfig() {
	client, err := gogemini.New(
		gogemini.WithAPIKey("YOUR_API_KEY"),
		gogemini.WithSystemInstruction("Answer in one short sentence."),
		gogemini.WithGenerationConfig(gogemini.GenerationConfig{
			Temperature:     gogemini.Ptr(0.0), // zero is sent, so use a pointer
			MaxOutputTokens: 256,
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(client.Model() != "")
	// Output: true
}

func ExampleWithRetry() {
	// Retry 429 and 5xx up to 6 times, waiting from 500 ms up to 1 minute.
	client, err := gogemini.New(
		gogemini.WithAPIKey("YOUR_API_KEY"),
		gogemini.WithRetry(gogemini.RetryPolicy{MaxAttempts: 6, InitialDelay: 500 * time.Millisecond, MaxDelay: time.Minute}),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(client.Model() != "")
	// Output: true
}
