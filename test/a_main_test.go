package test

import (
	"os"
	"testing"

	"github.com/Allan-Nava/go-gemini/configuration"
	"github.com/Allan-Nava/go-gemini/env"
	"github.com/Allan-Nava/go-gemini/gogemini"
)

func TestMain(m *testing.M) {
	if os.Getenv("APP_ENV") == "" {
		err := os.Setenv("APP_ENV", "test")
		if err != nil {
			panic("could not set test env")
		}
	}
	env.Load()
	m.Run()
}

func GoGetGemini() gogemini.IGoGemini {
	//
	configuration := configuration.GetConfiguration()
	g := gogemini.NewGoGemini(
		configuration,
	)
	//
	return g
}
