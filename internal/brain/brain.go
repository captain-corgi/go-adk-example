// Package brain is gBrain: frozen brand context (read from disk) plus an
// in-memory memory.Service queried at research time.
package brain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/adk/v2/memory"
)

type Brain struct {
	brand string
	mem   memory.Service
}

// New reads every *.md under dir (sorted by name) and concatenates them into
// the frozen brand context. Missing dir is non-fatal: BrandContext() is empty.
func New(dir string, mem memory.Service) (*Brain, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return &Brain{mem: mem}, nil
		}
		return nil, fmt.Errorf("read brand dir %q: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var sb strings.Builder
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", n, err)
		}
		sb.WriteString(string(b))
		sb.WriteString("\n")
	}
	return &Brain{brand: sb.String(), mem: mem}, nil
}

func (b *Brain) BrandContext() string   { return b.brand }
func (b *Brain) Memory() memory.Service { return b.mem }

// Load queries gBrain memory and returns the concatenated matching text, or "".
func (b *Brain) Load(ctx context.Context, appName, userID, query string) (string, error) {
	if b.mem == nil {
		return "", nil
	}
	resp, err := b.mem.SearchMemory(ctx, &memory.SearchRequest{
		Query: query, AppName: appName, UserID: userID,
	})
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, m := range resp.Memories {
		if m.Content == nil {
			continue
		}
		for _, p := range m.Content.Parts {
			if p.Text != "" {
				sb.WriteString(p.Text)
				sb.WriteString("\n")
			}
		}
	}
	return sb.String(), nil
}
