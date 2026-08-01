// Package main implements a minimal "hello world" agent built on Google's
// Agent Development Kit for Go (adk-go v2).
//
// The agent runs on any model exposed through the OpenAI Responses API — either
// OpenAI itself (api.openai.com) or an OpenAI-compatible endpoint pointed at via
// OPENAI_BASE_URL (recent Ollama, LM Studio, or vLLM). Configuration is read
// from a local .env file at startup; see .env.example.
package main

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
	openaimodel "google.golang.org/adk/v2/model/openaimodel"
)

// defaultModel is used when OPENAI_MODEL is unset. It targets the OpenAI
// Responses API, which the openaimodel integration is built around.
const defaultModel = "gpt-5.6-sol"

// main loads configuration, creates the hello agent, and runs it with the provided command-line arguments.
func main() {
	// Load variables from .env for local development. A missing file is fine —
	// the process environment may already provide them (e.g. CI, containers).
	// Other failures (permission denied, malformed contents) are not, so they
	// must abort instead of silently falling back to an incomplete config.
	if err := godotenv.Load(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("No .env file found (%v); relying on process environment", err)
		} else {
			log.Fatalf("Failed to load .env: %v", err)
		}
	}

	ctx := context.Background()

	modelName := os.Getenv("OPENAI_MODEL")
	if modelName == "" {
		modelName = defaultModel
	}

	// APIKey authenticates against api.openai.com; BaseURL redirects calls to an
	// OpenAI-compatible server. Either may be empty (the underlying client also
	// reads OPENAI_API_KEY from the environment).
	model, err := openaimodel.NewModel(ctx, modelName, &openaimodel.ClientConfig{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	helloAgent, err := llmagent.New(llmagent.Config{
		Name:        "hello_agent",
		Model:       model,
		Description: "A friendly greeter that says hello and answers questions.",
		Instruction: "You are a helpful, friendly assistant. Greet the user warmly and answer their questions concisely.",
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(helloAgent),
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, config, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
