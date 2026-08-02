// Command engine runs the Marketing Engine: one raw idea in, one landing-page
// deliverable out, end to end, on adk-go.
package main

import (
	"context"
	"log"
	"os"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagents/sequentialagent"
	"google.golang.org/adk/v2/artifact"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
	"google.golang.org/adk/v2/memory"

	"github.com/captain-corgi/go-adk-example/internal/brain"
	"github.com/captain-corgi/go-adk-example/internal/build/graph"
	"github.com/captain-corgi/go-adk-example/internal/config"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/brief"
	"github.com/captain-corgi/go-adk-example/internal/stages/ideation"
	"github.com/captain-corgi/go-adk-example/internal/stages/research"
	"github.com/captain-corgi/go-adk-example/internal/stages/results"
	"github.com/captain-corgi/go-adk-example/internal/stages/signoff"
	"github.com/captain-corgi/go-adk-example/internal/stages/synthesis"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx := context.Background()

	mem := memory.InMemoryService()
	art := artifact.InMemoryService()
	b, err := brain.New(cfg.GBrainDir, mem)
	if err != nil {
		log.Fatalf("brain: %v", err)
	}
	brand := b.BrandContext()

	briefAg, err := brief.New(stages.Config{Model: cfg.Models.Cheap(), Brand: brand})
	if err != nil {
		log.Fatalf("brief: %v", err)
	}
	ideationAg, err := ideation.New(stages.Config{Model: cfg.Models.Cheap(), Brand: brand})
	if err != nil {
		log.Fatalf("ideation: %v", err)
	}
	researchAg, err := research.New(stages.Config{Model: cfg.Models.Cheap(), Brand: brand}, b)
	if err != nil {
		log.Fatalf("research: %v", err)
	}
	synthesisAg, err := synthesis.New(stages.Config{Model: cfg.Models.Strong(), Brand: brand})
	if err != nil {
		log.Fatalf("synthesis: %v", err)
	}
	signoffAg, err := signoff.New(cfg.AutoApprove)
	if err != nil {
		log.Fatalf("signoff: %v", err)
	}
	buildAg, err := graph.New(cfg.Models, cfg.MaxLoopIter)
	if err != nil {
		log.Fatalf("build: %v", err)
	}
	resultsAg, err := results.New(mem)
	if err != nil {
		log.Fatalf("results: %v", err)
	}

	pipeline, err := sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name:        "marketing_engine",
			Description: "Raw marketing idea in, shipped landing-page campaign out.",
			SubAgents: []agent.Agent{
				briefAg, ideationAg, researchAg, synthesisAg, signoffAg, buildAg, resultsAg,
			},
		},
	})
	if err != nil {
		log.Fatalf("pipeline: %v", err)
	}

	launcherCfg := &launcher.Config{
		AgentLoader:     agent.NewSingleLoader(pipeline),
		ArtifactService: art,
		MemoryService:   mem,
	}

	l := full.NewLauncher()
	if err := l.Execute(ctx, launcherCfg, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
