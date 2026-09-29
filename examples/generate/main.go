// Command generate sends one prompt to Gemini and prints the answer.
//
//	export GEMINI_API_KEY=...          # from Google AI Studio
//	go run ./examples/generate "Explain goroutines in one sentence"
//	go run ./examples/generate -model gemini-3.5-flash-lite "Say hello"
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Allan-Nava/go-gemini/gogemini"
)

func main() {
	model := flag.String("model", gogemini.DefaultModel, "model name")
	timeout := flag.Duration("timeout", 60*time.Second, "request timeout")
	flag.Parse()

	prompt := strings.Join(flag.Args(), " ")
	if prompt == "" {
		fmt.Fprintln(os.Stderr, "usage: generate [-model name] [-timeout 60s] <prompt>")
		os.Exit(2)
	}

	client, err := gogemini.New(gogemini.WithModel(*model), gogemini.WithTimeout(*timeout))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	resp, err := client.GenerateContent(context.Background(), prompt)
	var apiErr *gogemini.APIError
	switch {
	case errors.As(err, &apiErr):
		fmt.Fprintf(os.Stderr, "Gemini API error: HTTP %d %s: %s\n", apiErr.StatusCode, apiErr.Status, apiErr.Message)
		os.Exit(1)
	case err != nil:
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if text := resp.Text(); text != "" {
		fmt.Println(text)
	} else if resp.PromptFeedback != nil && resp.PromptFeedback.BlockReason != "" {
		fmt.Fprintln(os.Stderr, "prompt blocked:", resp.PromptFeedback.BlockReason)
		os.Exit(1)
	}
	if u := resp.UsageMetadata; u != nil {
		fmt.Fprintf(os.Stderr, "[%s · %d tokens]\n", resp.ModelVersion, u.TotalTokenCount)
	}
}
