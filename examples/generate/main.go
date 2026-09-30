// Command generate sends one prompt to Gemini and prints the answer.
//
//	export GEMINI_API_KEY=...          # from Google AI Studio
//	go run ./examples/generate "Explain goroutines in one sentence"
//	go run ./examples/generate -model gemini-3.5-flash-lite "Say hello"
//	go run ./examples/generate -stream "Tell me a short story"
//	go run ./examples/generate -image photo.png "What is in this picture?"
//	go run ./examples/generate -count "How many tokens is this?"
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Allan-Nava/go-gemini/gogemini"
)

func main() {
	model := flag.String("model", "", "model name (default: the SDK's recommended model)")
	timeout := flag.Duration("timeout", 5*time.Minute, "time limit for one attempt (for -stream: until the answer starts)")
	stream := flag.Bool("stream", false, "print the answer as it is generated")
	system := flag.String("system", "", "system instruction")
	image := flag.String("image", "", "path of an image (PNG, JPEG, WebP, …) to send with the prompt")
	count := flag.Bool("count", false, "only count the prompt's tokens")
	flag.Parse()

	prompt := strings.Join(flag.Args(), " ")
	if prompt == "" {
		fmt.Fprintln(os.Stderr, "usage: generate [-model name] [-timeout 5m] [-stream] [-system text] [-image file] [-count] <prompt>")
		os.Exit(2)
	}

	opts := []gogemini.Option{gogemini.WithTimeout(*timeout), gogemini.WithSystemInstruction(*system)}
	if *model != "" {
		opts = append(opts, gogemini.WithModel(*model))
	}
	client, err := gogemini.New(opts...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	parts := []gogemini.Part{gogemini.TextPart(prompt)}
	if *image != "" {
		data, err := os.ReadFile(*image)
		if err != nil {
			fail(err)
		}
		parts = append(parts, gogemini.InlineDataPart(http.DetectContentType(data), data))
	}
	req := &gogemini.GenerateContentRequest{Contents: []gogemini.Content{gogemini.UserContent(parts...)}}
	ctx := context.Background()

	if *count {
		n, err := client.CountTokens(ctx, req)
		if err != nil {
			fail(err)
		}
		fmt.Println(n.TotalTokens, "tokens")
		return
	}

	if *stream {
		var last *gogemini.Response
		for chunk, err := range client.GenerateStream(ctx, req) {
			if err != nil {
				fail(err)
			}
			fmt.Print(chunk.Text())
			last = chunk
		}
		fmt.Println()
		usage(last)
		return
	}

	resp, err := client.Generate(ctx, req)
	if err != nil {
		fail(err)
	}

	if text := resp.Text(); text != "" {
		fmt.Println(text)
	} else if resp.PromptFeedback != nil && resp.PromptFeedback.BlockReason != "" {
		fmt.Fprintln(os.Stderr, "prompt blocked:", resp.PromptFeedback.BlockReason)
		os.Exit(1)
	}
	usage(resp)
}

func fail(err error) {
	var apiErr *gogemini.APIError
	if errors.As(err, &apiErr) {
		fmt.Fprintf(os.Stderr, "Gemini API error: HTTP %d %s: %s\n", apiErr.StatusCode, apiErr.Status, apiErr.Message)
	} else {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(1)
}

func usage(resp *gogemini.Response) {
	if resp != nil && resp.UsageMetadata != nil {
		fmt.Fprintf(os.Stderr, "[%s · %d tokens]\n", resp.ModelVersion, resp.UsageMetadata.TotalTokenCount)
	}
}
