// internal/config/config_test.go
package config_test

import (
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/config"
)

func TestLoadDefaultsAndOverrides(t *testing.T) {
	// Simulate a minimal environment. A real API key is NOT required: the
	// openaimodel constructor only stores config; it does not dial out.
	// t.Setenv sets the var and registers automatic cleanup on test exit, so
	// no manual os.Unsetenv (or errcheck on os.Setenv's return) is needed.
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", "http://localhost")
	t.Setenv("OPENAI_MODEL", "gpt-test")
	t.Setenv("MODEL_STRONG", "gpt-strong")
	t.Setenv("MODEL_CHEAP", "gpt-cheap")
	t.Setenv("AUTO_APPROVE", "true")
	t.Setenv("MAX_LOOP_ITER", "5")
	t.Setenv("GBRAIN_DIR", "./brand")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.AutoApprove {
		t.Error("AutoApprove must be true when AUTO_APPROVE=true")
	}
	if cfg.MaxLoopIter != 5 {
		t.Errorf("MaxLoopIter = %d, want 5", cfg.MaxLoopIter)
	}
	if cfg.GBrainDir != "./brand" {
		t.Errorf("GBrainDir = %q, want ./brand", cfg.GBrainDir)
	}
	// Both roles resolve to non-nil models built from the named env models.
	if cfg.Models.Strong() == nil || cfg.Models.Cheap() == nil {
		t.Error("Models.Strong/Cheap must be non-nil")
	}
}
