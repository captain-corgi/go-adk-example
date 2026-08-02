// internal/brain/brain_test.go
package brain_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/adk/v2/memory"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/captain-corgi/go-adk-example/internal/brain"
)

func TestBrandContextConcatenatesMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "voice.md"), []byte("voice: bold"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "offers.md"), []byte("offers: x"), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := brain.New(dir, memory.InMemoryService())
	if err != nil {
		t.Fatal(err)
	}
	ctx := b.BrandContext()
	if !strings.Contains(ctx, "voice: bold") || !strings.Contains(ctx, "offers: x") {
		t.Errorf("BrandContext missing files: %q", ctx)
	}
}

func TestLoadReturnsEmptyWhenMemoryEmpty(t *testing.T) {
	b, err := brain.New(t.TempDir(), memory.InMemoryService())
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Load(context.Background(), "app", "u1", "anything")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("Load on empty memory = %q, want empty", got)
	}
}

func TestLoadReturnsSeededMemory(t *testing.T) {
	dir := t.TempDir()
	mem := memory.InMemoryService()
	// Seed memory by ingesting a session with one user message.
	ctx := context.Background()
	ss := session.InMemoryService()
	created, err := ss.Create(ctx, &session.CreateRequest{AppName: "app", UserID: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	sess := created.Session
	ev := session.NewEvent(ctx, "seed")
	ev.Author = "user"
	ev.LLMResponse = model.LLMResponse{Content: genai.NewContentFromText("Tokyo trip converted well", genai.RoleUser)}
	if err := ss.AppendEvent(ctx, sess, ev); err != nil {
		t.Fatal(err)
	}
	if err := mem.AddSessionToMemory(ctx, sess); err != nil {
		t.Fatal(err)
	}

	b, err := brain.New(dir, mem)
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Load(ctx, "app", "u1", "Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Tokyo") {
		t.Errorf("Load missed seeded memory: %q", got)
	}
}
