// Package config loads environment, builds the OpenAI-compatible model(s),
// and exposes the routing.Models registry plus engine knobs.
package config

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"google.golang.org/adk/v2/model"
	openaimodel "google.golang.org/adk/v2/model/openaimodel"

	"github.com/captain-corgi/go-adk-example/internal/routing"
)

const defaultModel = "gpt-5.6-sol"

// Config holds everything main.go needs to assemble the engine.
type Config struct {
	Models      routing.Models
	AutoApprove bool
	MaxLoopIter int
	GBrainDir   string
}

// Load reads .env (optional) + the process environment and builds the models.
func Load() (*Config, error) {
	// Missing .env is fine; malformed .env is not.
	if err := godotenv.Load(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("load .env: %w", err)
		}
		log.Printf("No .env file (%v); relying on process environment", err)
	}

	ctx := context.Background()
	strong, err := buildModel(ctx, firstNonEmpty(os.Getenv("MODEL_STRONG"), os.Getenv("OPENAI_MODEL")))
	if err != nil {
		return nil, err
	}
	cheap, err := buildModel(ctx, firstNonEmpty(os.Getenv("MODEL_CHEAP"), os.Getenv("OPENAI_MODEL")))
	if err != nil {
		return nil, err
	}

	return &Config{
		Models:      routing.NewModels(strong, cheap),
		AutoApprove: boolEnv("AUTO_APPROVE"),
		MaxLoopIter: intEnv("MAX_LOOP_ITER", 3),
		GBrainDir:   firstNonEmpty(os.Getenv("GBRAIN_DIR"), "./brand"),
	}, nil
}

// buildModel constructs an OpenAI-compatible model. A missing name falls back
// to defaultModel. A construction failure is returned to the caller (config.Load)
// so the error contract of Load — not a process-killing log.Fatalf here — is
// honored and callers (main.go, future tests) can report or recover.
// Tests inject fakes and never call buildModel.
func buildModel(ctx context.Context, name string) (model.LLM, error) {
	if name == "" {
		name = defaultModel
	}
	m, err := openaimodel.NewModel(ctx, name, &openaimodel.ClientConfig{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
	})
	if err != nil {
		return nil, fmt.Errorf("create model %q: %w", name, err)
	}
	return m, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
func boolEnv(key string) bool { return os.Getenv(key) == "true" || os.Getenv(key) == "1" }
func intEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
