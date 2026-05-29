package spawner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Config holds spawn configuration loaded from environment at startup.
type Config struct {
	ClaudeCmd            string // CLI binary name, default "claude"
	GeminiCmd            string // CLI binary name, default "agy"
	DeepseekCmd          string // CLI binary name, default "claude-ds"
	WorkDir              string // working directory for spawned CLIs
	ClaudeDefaultModel   string // --model value when no variant specified; empty = CLI default
	GeminiDefaultModel   string // --model value when no variant specified; empty = CLI default
	DeepseekDefaultModel string // --model value when no variant specified; empty = CLI default
}

var claudeModels = map[string]string{
	"opus":   "claude-opus-4-7",
	"sonnet": "claude-sonnet-4-6",
	"haiku":  "claude-haiku-4-5-20251001",
}

var geminiModels = map[string]string{
	"pro":   "gemini-2.5-pro",
	"flash": "gemini-2.5-flash",
}

// Spawner execs CLI processes for summoned agents.
type Spawner struct {
	cfg Config
}

// New creates a Spawner. Default CLI command names are applied if not set.
func New(cfg Config) *Spawner {
	if cfg.ClaudeCmd == "" {
		cfg.ClaudeCmd = "claude"
	}
	if cfg.GeminiCmd == "" {
		cfg.GeminiCmd = "agy"
	}
	if cfg.DeepseekCmd == "" {
		cfg.DeepseekCmd = "claude-ds"
	}
	return &Spawner{cfg: cfg}
}

// Spawn runs the CLI for the given model and blocks until it exits.
// Call in a goroutine. payload is the fully-formatted -p argument.
func (s *Spawner) Spawn(ctx context.Context, name, variant, payload string) error {
	cmd, modelID := s.buildCmd(ctx, name, variant, payload)
	if cmd == nil {
		return fmt.Errorf("unknown model: %q", name)
	}

	slog.Info("spawning agent", "model", name, "variant", variant, "modelID", modelID)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		exitCode := -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		slog.Error("agent exited with error",
			"model", name,
			"elapsed", elapsed,
			"exit_code", exitCode,
			"error", err,
			"stdout", stdout.String(),
			"stderr", stderr.String(),
		)
		return fmt.Errorf("spawn %s: %w", name, err)
	}

	slog.Info("agent exited cleanly", "model", name, "elapsed", elapsed)
	if out := stdout.String(); out != "" {
		slog.Debug("agent stdout", "model", name, "output", out)
	}
	if serr := stderr.String(); serr != "" {
		slog.Warn("agent stderr on clean exit", "model", name, "stderr", serr)
	}
	return nil
}

func (s *Spawner) buildCmd(ctx context.Context, name, variant, payload string) (*exec.Cmd, string) {
	switch name {
	case "claude":
		modelID := resolveModel(variant, claudeModels, s.cfg.ClaudeDefaultModel)
		args := []string{"-p", payload}
		if modelID != "" {
			args = append([]string{"--model", modelID}, args...)
		}
		cmd := exec.CommandContext(ctx, s.cfg.ClaudeCmd, args...)
		cmd.Dir = s.cfg.WorkDir
		cmd.Env = os.Environ()
		return cmd, modelID

	case "gemini":
		modelID := resolveModel(variant, geminiModels, s.cfg.GeminiDefaultModel)
		args := []string{"-p", payload}
		if modelID != "" {
			args = append([]string{"--model", modelID}, args...)
		}
		cmd := exec.CommandContext(ctx, s.cfg.GeminiCmd, args...)
		cmd.Dir = s.cfg.WorkDir
		cmd.Env = os.Environ()
		return cmd, modelID

	case "deepseek":
		modelID := s.cfg.DeepseekDefaultModel
		args := []string{"-p", payload}
		if modelID != "" {
			args = append([]string{"--model", modelID}, args...)
		}
		cmd := exec.CommandContext(ctx, s.cfg.DeepseekCmd, args...)
		cmd.Dir = s.cfg.WorkDir
		cmd.Env = os.Environ()
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
