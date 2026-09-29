package gogemini_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"

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
