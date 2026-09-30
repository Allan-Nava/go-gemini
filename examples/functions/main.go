// Command functions lets the model call a Go function to answer: here multiply,
// so the product is exact instead of guessed.
//
//	export GEMINI_API_KEY=...
//	go run ./examples/functions "What is 48213 times 7919?"
//	go run ./examples/functions -model gemini-3.5-flash-lite "Multiply 12.5 by 8, then by 3"
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Allan-Nava/go-gemini/gogemini"
)

func main() {
	model := flag.String("model", "", "model name (default: the SDK's recommended model)")
	flag.Parse()
	prompt := strings.Join(flag.Args(), " ")
	if prompt == "" {
		fmt.Fprintln(os.Stderr, "usage: functions [-model name] <prompt>")
		os.Exit(2)
	}

	var opts []gogemini.Option
	if *model != "" {
		opts = append(opts, gogemini.WithModel(*model))
	}
	client, err := gogemini.New(opts...)
	if err != nil {
		fail(err)
	}

	chat := client.NewChat()
	chat.AddFunction(gogemini.FunctionDeclaration{
		Name:        "multiply",
		Description: "Multiplies two numbers exactly. Use it for any multiplication.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			"required": []string{"a", "b"},
		},
	}, func(ctx context.Context, args json.RawMessage) (any, error) {
		var p struct{ A, B float64 }
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, err
		}
		num := func(x float64) string { return strconv.FormatFloat(x, 'f', -1, 64) }
		fmt.Fprintf(os.Stderr, "[multiply(%s, %s) = %s]\n", num(p.A), num(p.B), num(p.A*p.B))
		return map[string]float64{"product": p.A * p.B}, nil
	})

	resp, err := chat.Send(context.Background(), prompt)
	if err != nil {
		fail(err)
	}
	fmt.Println(resp.Text())
	fmt.Fprintf(os.Stderr, "[%d contents in the history]\n", len(chat.History()))
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
