package spawner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// Config holds spawn configuration loaded from environment variables at startup.
type Config struct {
	ClaudeCmd          string // CLI command name, default "claude"
	GeminiCmd          string // CLI command name, default "gemini"
	NFSMount           string // working directory for spawned CLIs
	ClaudeDefaultModel string // --model value when no variant specified; empty = CLI default
	GeminiDefaultModel string // --model value when no variant specified; empty = CLI default
}

// claudeModels maps variant shortnames to Anthropic model IDs.
var claudeModels = map[string]string{
	"opus":   "claude-opus-4-7",
	"sonnet": "claude-sonnet-4-6",
	"haiku":  "claude-haiku-4-5-20251001",
}

// geminiModels maps variant shortnames to Gemini model IDs.
var geminiModels = map[string]string{
	"pro":   "gemini-2.5-pro",
	"flash": "gemini-2.5-flash",
}

// Spawner execs CLI processes for summoned agents.
type Spawner struct {
	cfg Config
}

// New creates a Spawner with the given configuration.
// Default CLI command names are applied if not set.
func New(cfg Config) *Spawner {
	if cfg.ClaudeCmd == "" {
		cfg.ClaudeCmd = "claude"
	}
	if cfg.GeminiCmd == "" {
		cfg.GeminiCmd = "gemini"
	}
	return &Spawner{cfg: cfg}
}

// Spawn runs the CLI for the given model and blocks until it exits.
// Intended to be called in a goroutine.
// name is "claude" or "gemini"; variant is e.g. "opus" or "" for the CLI default.
func (s *Spawner) Spawn(ctx context.Context, name, variant, prompt string) error {
	cmd, modelID := s.buildCmd(ctx, name, variant, prompt)
	if cmd == nil {
		return fmt.Errorf("unknown model: %q", name)
	}

	slog.Info("spawning agent", "model", name, "variant", variant, "modelID", modelID)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		slog.Error("agent exited with error", "model", name, "error", err,
			"stdout", stdout.String(), "stderr", stderr.String())
		return fmt.Errorf("spawn %s: %w", name, err)
	}

	slog.Info("agent exited cleanly", "model", name)
	return nil
}

func (s *Spawner) buildCmd(ctx context.Context, name, variant, prompt string) (*exec.Cmd, string) {
	payload := FormatPayload(prompt)

	switch name {
	case "claude":
		modelID := resolveModel(variant, claudeModels, s.cfg.ClaudeDefaultModel)
		args := []string{"-p", payload}
		if modelID != "" {
			args = append([]string{"--model", modelID}, args...)
		}
		cmd := exec.CommandContext(ctx, s.cfg.ClaudeCmd, args...)
		cmd.Dir = s.cfg.NFSMount
		cmd.Env = os.Environ() // inherits ANTHROPIC_API_KEY
		return cmd, modelID

	case "gemini":
		modelID := resolveModel(variant, geminiModels, s.cfg.GeminiDefaultModel)
		args := []string{"-p", payload}
		if modelID != "" {
			args = append([]string{"--model", modelID}, args...)
		}
		cmd := exec.CommandContext(ctx, s.cfg.GeminiCmd, args...)
		cmd.Dir = s.cfg.NFSMount
		cmd.Env = os.Environ() // inherits GEMINI_API_KEY
		return cmd, modelID
	}

	return nil, ""
}

func resolveModel(variant string, table map[string]string, defaultModel string) string {
	if variant != "" {
		if id, ok := table[variant]; ok {
			return id
		}
	}
	return defaultModel
}
