# Summoner: Architect Roundtable Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `roundtable` session mode where a designated leader agent drives a structured multi-model design discussion on Discord, with Summoner orchestrating re-spawns based on who last spoke.

**Architecture:** New `@Summoner roundtable [leader-model] [prompt]` command spawns all configured models immediately — leader gets a ReAct-loop prompt, participants get a "wait to be asked" prompt. Summoner watches who posts: human or participant message → re-spawn leader; leader message with participant @mention → re-spawn that participant; leader @Summoner command → execute it (leader is on the command allowlist). The formatted `-p` payload is stored in `ActiveModel.Prompt` at session creation and reused on every re-spawn unchanged. Deepseek is added as a third model using the `claude-ds` CLI wrapper.

**Tech Stack:** Go 1.25, github.com/bwmarrin/discordgo, stdlib only.

---

## File Map

```
internal/trigger/parser.go          add CommandRoundtable + Leader field to Command
internal/trigger/parser_test.go     add roundtable parse tests
internal/session/session.go         add SetLeader, LeaderModel, IsRoundtable, ParticipantNames, Model
internal/session/session_test.go    add roundtable state tests
internal/spawner/payload.go         add FormatLeaderPayload, FormatParticipantPayload; keep FormatPayload
internal/spawner/payload_test.go    add leader/participant payload tests
internal/spawner/spawner.go         rename prompt→payload in buildCmd/Spawn; add deepseek; update Config
cmd/summoner/main.go                handleRoundtable, roundtable message dispatch, config additions
deploy/deployment.yaml              add discord-mcp-deepseek sidecar
```

---

## Task 1: Trigger parser — add `roundtable` command

**Files:**
- Modify: `internal/trigger/parser.go`
- Modify: `internal/trigger/parser_test.go`

The parser already knows `claude`, `gemini`, `both`, `dismiss`. Add `roundtable` as a new command type. After `roundtable`, the next token is taken as the leader model name if it matches a known model (`claude`, `gemini`, `deepseek`); otherwise the entire remainder is the prompt and the leader field is left empty (main.go will apply the configured default).

- [ ] **Step 1: Write the failing tests**

Add to `internal/trigger/parser_test.go`:

```go
func TestParse_RoundtableClaudeLeader(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> roundtable claude design the auth system", "123456")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Type != trigger.CommandRoundtable {
		t.Fatalf("expected roundtable, got %v", cmd.Type)
	}
	if cmd.Leader != "claude" {
		t.Fatalf("expected leader claude, got %q", cmd.Leader)
	}
	if cmd.Prompt != "design the auth system" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}

func TestParse_RoundtableGeminiProLeader(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> roundtable gemini pro design the cache", "123456")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Type != trigger.CommandRoundtable {
		t.Fatalf("expected roundtable, got %v", cmd.Type)
	}
	if cmd.Leader != "gemini" {
		t.Fatalf("expected leader gemini, got %q", cmd.Leader)
	}
	if cmd.Variant != "pro" {
		t.Fatalf("expected variant pro, got %q", cmd.Variant)
	}
	if cmd.Prompt != "design the cache" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}

func TestParse_RoundtableDeepseekLeader(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> roundtable deepseek tradeoffs on storage backends", "123456")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Leader != "deepseek" {
		t.Fatalf("expected leader deepseek, got %q", cmd.Leader)
	}
	if cmd.Prompt != "tradeoffs on storage backends" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}

func TestParse_RoundtableDefaultLeader(t *testing.T) {
	cmd, ok := trigger.Parse("<@123456> roundtable design the auth system", "123456")
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Type != trigger.CommandRoundtable {
		t.Fatalf("expected roundtable, got %v", cmd.Type)
	}
	if cmd.Leader != "" {
		t.Fatalf("expected empty leader (use default), got %q", cmd.Leader)
	}
	if cmd.Prompt != "design the auth system" {
		t.Fatalf("unexpected prompt: %q", cmd.Prompt)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd /Users/bnaylor/src/summoner && go test ./internal/trigger/...
```

Expected: `trigger.CommandRoundtable undefined` compile error.

- [ ] **Step 3: Implement**

Replace `internal/trigger/parser.go`:

```go
package trigger

import (
	"strings"
)

// CommandType identifies the kind of command parsed from a Discord mention.
type CommandType int

const (
	CommandUnknown    CommandType = iota
	CommandSummon                 // summon a single model or "both"
	CommandDismiss                // end the active session
	CommandRoundtable             // start a structured multi-model design session
)

// Command is the result of parsing a Discord message that mentions the Summoner bot.
type Command struct {
	Type    CommandType
	Model   string // "claude", "gemini", "deepseek", "both" — for CommandSummon
	Leader  string // "claude", "gemini", "deepseek", or "" (use default) — for CommandRoundtable
	Variant string // "opus", "sonnet", "haiku", "pro", "flash", or ""
	Prompt  string
}

var knownModels = map[string]bool{
	"claude":   true,
	"gemini":   true,
	"deepseek": true,
}

var claudeVariants = map[string]bool{"opus": true, "sonnet": true, "haiku": true}
var geminiVariants = map[string]bool{"pro": true, "flash": true}

// Parse extracts a Command from a Discord message content string.
// summonerID is the Discord user ID of the Summoner bot.
// Returns (Command{}, false) if the Summoner bot is not mentioned.
func Parse(content, summonerID string) (Command, bool) {
	mention := "<@" + summonerID + ">"
	mentionBang := "<@!" + summonerID + ">"

	idx := strings.Index(content, mention)
	if idx == -1 {
		idx = strings.Index(content, mentionBang)
		if idx == -1 {
			return Command{}, false
		}
		content = strings.Replace(content, mentionBang, "", 1)
	} else {
		content = strings.Replace(content, mention, "", 1)
	}

	tokens := strings.Fields(content)
	if len(tokens) == 0 {
		return Command{Type: CommandUnknown}, true
	}

	first := strings.ToLower(tokens[0])

	switch first {
	case "dismiss":
		return Command{Type: CommandDismiss}, true

	case "roundtable":
		return parseRoundtable(tokens[1:]), true

	case "claude", "gemini", "deepseek", "both":
		return parseSummon(first, tokens[1:]), true

	default:
		return Command{Type: CommandUnknown}, true
	}
}

func parseRoundtable(tokens []string) Command {
	cmd := Command{Type: CommandRoundtable}
	if len(tokens) == 0 {
		return cmd
	}

	first := strings.ToLower(tokens[0])
	if knownModels[first] {
		cmd.Leader = first
		tokens = tokens[1:]
	}

	// Optional variant after leader model
	if len(tokens) > 0 && cmd.Leader != "" {
		second := strings.ToLower(tokens[0])
		if isVariantFor(cmd.Leader, second) {
			cmd.Variant = second
			tokens = tokens[1:]
		}
	}

	cmd.Prompt = strings.Join(tokens, " ")
	return cmd
}

func parseSummon(model string, tokens []string) Command {
	cmd := Command{Type: CommandSummon, Model: model}
	if len(tokens) > 0 {
		second := strings.ToLower(tokens[0])
		if isVariantFor(model, second) {
			cmd.Variant = second
			tokens = tokens[1:]
		}
	}
	cmd.Prompt = strings.Join(tokens, " ")
	return cmd
}

func isVariantFor(model, token string) bool {
	switch model {
	case "claude":
		return claudeVariants[token]
	case "gemini":
		return geminiVariants[token]
	case "both":
		return claudeVariants[token] || geminiVariants[token]
	}
	return false
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/trigger/... -v
```

Expected: all PASS, including existing tests.

- [ ] **Step 5: Commit**

```bash
git add internal/trigger/parser.go internal/trigger/parser_test.go
git commit -m "feat: add roundtable command to trigger parser"
```

---

## Task 2: Session — roundtable state

**Files:**
- Modify: `internal/session/session.go`
- Modify: `internal/session/session_test.go`

Add leader tracking to `Session`. `SetLeader` marks the session as a roundtable and names the leader model. `Model(name)` retrieves a single active model by name (needed by main.go for targeted re-spawns).

- [ ] **Step 1: Write the failing tests**

Add to `internal/session/session_test.go`:

```go
func TestSession_SetLeaderIsRoundtable(t *testing.T) {
	s := session.NewSession("chan1")
	if s.IsRoundtable() {
		t.Fatal("should not be roundtable before SetLeader")
	}
	s.AddModel("claude", "opus", "payload-for-claude")
	s.AddModel("gemini", "", "payload-for-gemini")
	s.SetLeader("claude")
	if !s.IsRoundtable() {
		t.Fatal("should be roundtable after SetLeader")
	}
	if s.LeaderModel() != "claude" {
		t.Fatalf("expected leader claude, got %q", s.LeaderModel())
	}
}

func TestSession_ParticipantNamesExcludesLeader(t *testing.T) {
	s := session.NewSession("chan1")
	s.AddModel("claude", "opus", "payload-claude")
	s.AddModel("gemini", "", "payload-gemini")
	s.AddModel("deepseek", "", "payload-deepseek")
	s.SetLeader("claude")
	names := s.ParticipantNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(names))
	}
	for _, n := range names {
		if n == "claude" {
			t.Fatal("leader should not appear in ParticipantNames")
		}
	}
}

func TestSession_ModelLookup(t *testing.T) {
	s := session.NewSession("chan1")
	s.AddModel("claude", "opus", "the-payload")
	m, ok := s.Model("claude")
	if !ok {
		t.Fatal("expected to find claude")
	}
	if m.Prompt != "the-payload" {
		t.Fatalf("unexpected prompt: %q", m.Prompt)
	}
	_, ok = s.Model("gemini")
	if ok {
		t.Fatal("should not find gemini (not added)")
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/session/...
```

Expected: `s.IsRoundtable undefined` compile error.

- [ ] **Step 3: Implement**

Replace `internal/session/session.go`:

```go
package session

import (
	"sync"
	"time"
)

// ActiveModel holds the details needed to re-spawn a summoned CLI process.
// Prompt stores the fully-formatted -p payload (not the raw user input).
type ActiveModel struct {
	Name    string
	Variant string
	Prompt  string // fully-formatted CLI payload, reused on every re-spawn
}

// Session tracks a single active consulting session in one Discord channel.
type Session struct {
	ChannelID string
	mu          sync.Mutex
	models      map[string]*ActiveModel
	timer       *time.Timer
	leaderModel string
	isRoundtable bool
}

// NewSession creates a new Session for the given Discord channel ID.
func NewSession(channelID string) *Session {
	return &Session{
		ChannelID: channelID,
		models:    make(map[string]*ActiveModel),
	}
}

// AddModel registers a model as active. If already present, updates variant and prompt.
func (s *Session) AddModel(name, variant, prompt string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.models[name] = &ActiveModel{Name: name, Variant: variant, Prompt: prompt}
}

// HasModel reports whether the named model is currently active.
func (s *Session) HasModel(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.models[name]
	return ok
}

// Model returns the ActiveModel for the given name, if present.
func (s *Session) Model(name string) (ActiveModel, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.models[name]
	if !ok {
		return ActiveModel{}, false
	}
	return *m, true
}

// Models returns a snapshot of all active models.
func (s *Session) Models() []ActiveModel {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ActiveModel, 0, len(s.models))
	for _, m := range s.models {
		out = append(out, *m)
	}
	return out
}

// SetLeader marks this session as a roundtable and designates the named model as leader.
// The model must already have been added via AddModel.
func (s *Session) SetLeader(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leaderModel = model
	s.isRoundtable = true
}

// IsRoundtable reports whether this is a structured roundtable session.
func (s *Session) IsRoundtable() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isRoundtable
}

// LeaderModel returns the name of the leader model, or "" for non-roundtable sessions.
func (s *Session) LeaderModel() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.leaderModel
}

// ParticipantNames returns the names of all active models that are not the leader.
func (s *Session) ParticipantNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for name := range s.models {
		if name != s.leaderModel {
			out = append(out, name)
		}
	}
	return out
}

// ResetTimer starts or restarts the inactivity timer. fn is called when it fires.
func (s *Session) ResetTimer(d time.Duration, fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(d, fn)
}

// StopTimer cancels the inactivity timer.
func (s *Session) StopTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/session/... -v
```

Expected: all PASS, including existing tests.

- [ ] **Step 5: Commit**

```bash
git add internal/session/session.go internal/session/session_test.go
git commit -m "feat: add roundtable state to Session"
```

---

## Task 3: Payload — leader and participant prompts

**Files:**
- Modify: `internal/spawner/payload.go`
- Modify: `internal/spawner/payload_test.go`

Add `FormatLeaderPayload` and `FormatParticipantPayload`. Keep `FormatPayload` unchanged for non-roundtable summons.

The leader payload tells the agent to drive the ReAct loop: read history, ask the next question, address participants by @mentioning their Discord display name, write artifacts to `artifactsDir` when consensus is reached, then `@Summoner dismiss`.

The participant payload tells the agent to wait until addressed, respond substantively, and not write files.

- [ ] **Step 1: Write the failing tests**

Add to `internal/spawner/payload_test.go`:

```go
func TestFormatLeaderPayload_ContainsTopic(t *testing.T) {
	out := spawner.FormatLeaderPayload("design the caching layer", "/artifacts", []string{"BTGemini", "BTDeepseek"})
	if !strings.Contains(out, "design the caching layer") {
		t.Fatal("leader payload missing topic")
	}
}

func TestFormatLeaderPayload_ContainsParticipants(t *testing.T) {
	out := spawner.FormatLeaderPayload("topic", "/artifacts", []string{"BTGemini", "BTDeepseek"})
	if !strings.Contains(out, "BTGemini") {
		t.Fatal("leader payload missing BTGemini")
	}
	if !strings.Contains(out, "BTDeepseek") {
		t.Fatal("leader payload missing BTDeepseek")
	}
}

func TestFormatLeaderPayload_ContainsArtifactsDir(t *testing.T) {
	out := spawner.FormatLeaderPayload("topic", "/my/artifacts", []string{"BTGemini"})
	if !strings.Contains(out, "/my/artifacts") {
		t.Fatal("leader payload missing artifacts dir")
	}
}

func TestFormatLeaderPayload_ContainsLastCall(t *testing.T) {
	out := spawner.FormatLeaderPayload("topic", "/artifacts", []string{"BTGemini"})
	if !strings.Contains(out, "Last call") {
		t.Fatal("leader payload missing Last call instruction")
	}
}

func TestFormatLeaderPayload_ContainsDismissInstruction(t *testing.T) {
	out := spawner.FormatLeaderPayload("topic", "/artifacts", []string{"BTGemini"})
	if !strings.Contains(out, "@Summoner dismiss") {
		t.Fatal("leader payload missing dismiss instruction")
	}
}

func TestFormatParticipantPayload_ContainsTopic(t *testing.T) {
	out := spawner.FormatParticipantPayload("design the caching layer", "BTClaude")
	if !strings.Contains(out, "design the caching layer") {
		t.Fatal("participant payload missing topic")
	}
}

func TestFormatParticipantPayload_ContainsLeaderName(t *testing.T) {
	out := spawner.FormatParticipantPayload("topic", "BTClaude")
	if !strings.Contains(out, "BTClaude") {
		t.Fatal("participant payload missing leader name")
	}
}

func TestFormatParticipantPayload_ContainsWaitInstruction(t *testing.T) {
	out := spawner.FormatParticipantPayload("topic", "BTClaude")
	if !strings.Contains(out, "wait") && !strings.Contains(out, "addressed") {
		t.Fatal("participant payload missing wait/addressed instruction")
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/spawner/...
```

Expected: `spawner.FormatLeaderPayload undefined` compile error.

- [ ] **Step 3: Implement**

Replace `internal/spawner/payload.go`:

```go
package spawner

import (
	"fmt"
	"strings"
)

const participantTemplate = `You are being summoned as a seasoned architect to join an ongoing technical
discussion on Discord. A colleague has called you in with the following
context: %s

Read the recent channel history and any relevant files in your working
directory to get up to speed, then engage as a thoughtful design partner.
Ask clarifying questions, surface tradeoffs, and push back where appropriate.

You are a guest in this conversation — be deliberate, not hasty. Do not
produce implementation artifacts; the team will handle those after consensus.

When you sense the discussion has reached consensus, say so clearly and
indicate you are stepping out.`

const leaderTemplate = `You are the session leader for a multi-agent design roundtable on Discord.

Topic: %s

The following agents are participating and will respond when you address them:
%s

Your responsibilities:
- Drive the discussion. Each time you are spawned, read the channel history
  to understand where the conversation left off, then continue from there.
- Ask targeted questions. Address a participant by @mentioning their display
  name (e.g. @BTGemini) in your message — this signals Summoner to re-spawn them.
- After each participant responds, synthesize what you heard before asking
  the next question or moving to the next topic.
- Keep the discussion focused. If it drifts, redirect it.
- When you believe consensus is near, announce "Last call!" and explicitly ask
  whether anyone has lingering concerns before closing the topic.
- Once consensus is confirmed, write the agreed design as a Markdown document
  to: %s
  Then post a brief summary of what was decided and issue: @Summoner dismiss

You can also add a model mid-session if needed: @Summoner summon <model>

Each time you are re-spawned, read the full channel history — your previous
messages are there. Pick up exactly where you left off.`

// FormatPayload returns the -p payload for a non-roundtable summoned CLI.
func FormatPayload(initialPrompt string) string {
	return fmt.Sprintf(participantTemplate, initialPrompt)
}

// FormatLeaderPayload returns the -p payload for the roundtable leader.
// participants is the list of display names (e.g. "BTGemini", "BTDeepseek").
// artifactsDir is the filesystem path where the leader should write output docs.
func FormatLeaderPayload(topic, artifactsDir string, participants []string) string {
	participantList := "  - " + strings.Join(participants, "\n  - ")
	return fmt.Sprintf(leaderTemplate, topic, participantList, artifactsDir)
}

// FormatParticipantPayload returns the -p payload for a roundtable participant.
// leaderDisplayName is the Discord display name of the leader (e.g. "BTClaude").
func FormatParticipantPayload(topic, leaderDisplayName string) string {
	const tmpl = `You are a participant in a multi-model design roundtable on Discord.

Topic: %s

%s is leading this session. Your role:
- Wait until the leader directly addresses you before contributing.
  Do not post on your own initiative.
- When addressed, respond substantively and concisely. Challenge assumptions,
  surface alternatives, and flag risks you see.
- Do not write files or take unilateral action — that is the leader's job.
- Do not dismiss the session — that is the leader's call.

Each time you are spawned, read the channel history to understand the current
state of the discussion, then respond to whatever the leader asked you.`
	return fmt.Sprintf(tmpl, topic, leaderDisplayName)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/spawner/... -v
```

Expected: all PASS, including existing `TestFormatPayload_*` tests.

- [ ] **Step 5: Commit**

```bash
git add internal/spawner/payload.go internal/spawner/payload_test.go
git commit -m "feat: add leader and participant payload formatters"
```

---

## Task 4: Spawner — decouple payload, add Deepseek

**Files:**
- Modify: `internal/spawner/spawner.go`

Two changes: (1) `Spawn` now receives a pre-formatted payload instead of a raw prompt — remove the internal `FormatPayload` call from `buildCmd`. (2) Add `deepseek` as a model using a configurable CLI command (default: `claude-ds`).

No new test file — the spawner shells out to real binaries. Payload formation is now tested in Task 3. The build check confirms correctness.

- [ ] **Step 1: Implement**

Replace `internal/spawner/spawner.go`:

```go
package spawner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// Config holds spawn configuration loaded from environment at startup.
type Config struct {
	ClaudeCmd            string // CLI binary name, default "claude"
	GeminiCmd            string // CLI binary name, default "gemini"
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
		cfg.GeminiCmd = "gemini"
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

	if err := cmd.Run(); err != nil {
		slog.Error("agent exited with error", "model", name, "error", err,
			"stdout", stdout.String(), "stderr", stderr.String())
		return fmt.Errorf("spawn %s: %w", name, err)
	}

	slog.Info("agent exited cleanly", "model", name)
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
```

- [ ] **Step 2: Build check**

```bash
go build ./...
```

Expected: compile error in `main.go` — `NFSMount` field no longer exists in `Config`. That's expected; fix it in Task 5.

- [ ] **Step 3: Confirm spawner package itself builds**

```bash
go build ./internal/spawner/...
```

Expected: exits 0.

- [ ] **Step 4: Commit**

```bash
git add internal/spawner/spawner.go
git commit -m "feat: decouple payload from Spawn; add deepseek model"
```

---

## Task 5: Main — roundtable orchestration

**Files:**
- Modify: `cmd/summoner/main.go`
- Modify: `deploy/deployment.yaml`

This is the largest task. Changes:

1. **Config additions**: `btDeepseekToken`, `deepseekCmd`, `deepseekDefaultModel`, `artifactsDir`, `roundtableLeader`, `workDir` (replaces `nfsMount`).
2. **Startup**: resolve Deepseek bot ID alongside Claude and Gemini; build `modelBotIDs map[string]string` (model name → Discord bot ID) in addition to the flat `agentIDs` list.
3. **`handleSummon`**: call `FormatPayload(cmd.Prompt)` before `AddModel`/`Spawn` (payload is no longer formed inside `Spawn`).
4. **`handleRoundtable`**: new function — sets leader, spawns all configured models with role-appropriate payloads.
5. **Message dispatch**: for roundtable sessions, route based on who posted rather than re-spawning everyone.
6. **`agentDisplayName`**: add `deepseek → BTDeepseek`.
7. **`deploy/deployment.yaml`**: add `discord-mcp-deepseek` sidecar.

- [ ] **Step 1: Write the updated main.go**

Replace `cmd/summoner/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bnaylor/summoner/internal/discord"
	"github.com/bnaylor/summoner/internal/session"
	"github.com/bnaylor/summoner/internal/spawner"
	"github.com/bnaylor/summoner/internal/trigger"
	"github.com/bwmarrin/discordgo"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := loadConfig()

	// Resolve bot IDs for all agent tokens at startup.
	// agentIDs: flat list for "is this an agent?" suppression checks.
	// modelBotIDs: model name → Discord bot ID, used for roundtable dispatch.
	var agentIDs []string
	modelBotIDs := make(map[string]string)

	for _, entry := range []struct{ model, token string }{
		{"claude", cfg.btClaudeToken},
		{"gemini", cfg.btGeminiToken},
		{"deepseek", cfg.btDeepseekToken},
	} {
		if entry.token == "" {
			continue
		}
		id, err := discord.LookupBotID(entry.token)
		if err != nil {
			slog.Warn("could not resolve agent bot ID", "model", entry.model, "error", err)
			continue
		}
		slog.Info("resolved agent ID", "model", entry.model, "id", id)
		agentIDs = append(agentIDs, id)
		modelBotIDs[entry.model] = id
	}

	client, err := discord.New(cfg.summonerToken)
	if err != nil {
		slog.Error("failed to connect to Discord", "error", err)
		os.Exit(1)
	}
	defer client.Close()

	slog.Info("summoner connected", "id", client.ID())

	mgr := session.NewManager(agentIDs)
	sp := spawner.New(spawner.Config{
		WorkDir:              cfg.workDir,
		ClaudeDefaultModel:   cfg.claudeDefaultModel,
		GeminiDefaultModel:   cfg.geminiDefaultModel,
		DeepseekCmd:          cfg.deepseekCmd,
		DeepseekDefaultModel: cfg.deepseekDefaultModel,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client.OnMessage(func(m *discordgo.MessageCreate) {
		if m.Author.ID == client.ID() {
			return
		}

		channelID := m.ChannelID
		sess := mgr.Get(channelID)

		// Roundtable sessions need special dispatch.
		if sess != nil && sess.IsRoundtable() {
			handleRoundtableMessage(ctx, client, mgr, sp, m, sess, modelBotIDs, cfg)
			return
		}

		// @Summoner command — only accepted from humans in non-roundtable context.
		cmd, isSummonerCmd := trigger.Parse(m.Content, client.ID())
		if isSummonerCmd {
			switch cmd.Type {
			case trigger.CommandSummon:
				handleSummon(ctx, client, mgr, sp, channelID, cmd, cfg.inactivityTimeout)
			case trigger.CommandRoundtable:
				handleRoundtable(ctx, client, mgr, sp, channelID, cmd, modelBotIDs, cfg)
			case trigger.CommandDismiss:
				handleDismiss(client, mgr, channelID)
			default:
				_ = client.Send(channelID, "Usage: `@Summoner roundtable [leader] [prompt]` · `@Summoner <claude|gemini|deepseek|both> [variant] [prompt]` · `@Summoner dismiss`")
			}
			return
		}

		// Non-roundtable re-spawn: any non-agent message wakes all active models.
		if sess == nil || mgr.IsAgent(m.Author.ID) {
			return
		}
		for _, model := range sess.Models() {
			model := model
			go func() {
				if err := sp.Spawn(ctx, model.Name, model.Variant, model.Prompt); err != nil {
					slog.Error("spawn error", "model", model.Name, "error", err)
				}
			}()
		}
		sess.ResetTimer(cfg.inactivityTimeout, func() {
			announceInactiveDeparture(client, mgr, channelID, sess.Models())
		})
	})

	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "ok")
		})
		if err := http.ListenAndServe(":8080", nil); err != nil {
			slog.Error("healthz server error", "error", err)
		}
	}()

	slog.Info("summoner ready")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down")
	cancel()
}

// handleRoundtableMessage routes incoming messages for an active roundtable session.
//
// Routing rules:
//   - Leader posts a @Summoner command → execute it (leader is on the allowlist)
//   - Leader posts and @mentions a participant → re-spawn that participant
//   - Human or participant posts → re-spawn leader
//   - Any other bot → ignore
func handleRoundtableMessage(
	ctx context.Context,
	client *discord.Client,
	mgr *session.Manager,
	sp *spawner.Spawner,
	m *discordgo.MessageCreate,
	sess *session.Session,
	modelBotIDs map[string]string,
	cfg config,
) {
	channelID := m.ChannelID
	leaderBotID := modelBotIDs[sess.LeaderModel()]

	if m.Author.ID == leaderBotID {
		// Leader is on the @Summoner command allowlist.
		if cmd, ok := trigger.Parse(m.Content, client.ID()); ok {
			switch cmd.Type {
			case trigger.CommandSummon:
				handleSummon(ctx, client, mgr, sp, channelID, cmd, cfg.inactivityTimeout)
			case trigger.CommandDismiss:
				handleDismiss(client, mgr, channelID)
			}
			return
		}

		// Re-spawn any participants @mentioned by the leader.
		for _, participantName := range sess.ParticipantNames() {
			participantBotID, ok := modelBotIDs[participantName]
			if !ok {
				continue
			}
			if strings.Contains(m.Content, "<@"+participantBotID+">") ||
				strings.Contains(m.Content, "<@!"+participantBotID+">") {
				am, ok := sess.Model(participantName)
				if !ok {
					continue
				}
				am := am
				go func() {
					if err := sp.Spawn(ctx, am.Name, am.Variant, am.Prompt); err != nil {
						slog.Error("participant spawn error", "model", am.Name, "error", err)
					}
				}()
			}
		}
		sess.ResetTimer(cfg.inactivityTimeout, func() {
			announceInactiveDeparture(client, mgr, channelID, sess.Models())
		})
		return
	}

	// Participant or human posted: re-spawn the leader.
	isOtherBot := mgr.IsAgent(m.Author.ID)
	isParticipant := isOtherBot && m.Author.ID != leaderBotID
	isHuman := !isOtherBot

	if !isHuman && !isParticipant {
		return // unrelated bot — ignore
	}

	leaderModel, ok := sess.Model(sess.LeaderModel())
	if !ok {
		return
	}
	go func() {
		if err := sp.Spawn(ctx, leaderModel.Name, leaderModel.Variant, leaderModel.Prompt); err != nil {
			slog.Error("leader spawn error", "error", err)
		}
	}()
	sess.ResetTimer(cfg.inactivityTimeout, func() {
		announceInactiveDeparture(client, mgr, channelID, sess.Models())
	})
}

func handleRoundtable(
	ctx context.Context,
	client *discord.Client,
	mgr *session.Manager,
	sp *spawner.Spawner,
	channelID string,
	cmd trigger.Command,
	modelBotIDs map[string]string,
	cfg config,
) {
	leaderModel := cmd.Leader
	if leaderModel == "" {
		leaderModel = cfg.roundtableLeader
	}

	// Determine which models to summon: all that have resolved bot IDs.
	allModels := []string{"claude", "gemini", "deepseek"}
	var toSummon []string
	for _, m := range allModels {
		if _, ok := modelBotIDs[m]; ok {
			toSummon = append(toSummon, m)
		}
	}

	if len(toSummon) == 0 {
		_ = client.Send(channelID, "No agent bot tokens configured. Set BTCLAUDE_TOKEN, BTGEMINI_TOKEN, or BTDEEPSEEK_TOKEN.")
		return
	}

	// Build participant display names (for the leader payload).
	var participantDisplayNames []string
	for _, m := range toSummon {
		if m != leaderModel {
			participantDisplayNames = append(participantDisplayNames, agentDisplayName(m, ""))
		}
	}

	leaderDisplayName := agentDisplayName(leaderModel, cmd.Variant)
	leaderPayload := spawner.FormatLeaderPayload(cmd.Prompt, cfg.artifactsDir, participantDisplayNames)
	participantPayload := spawner.FormatParticipantPayload(cmd.Prompt, leaderDisplayName)

	_ = client.Send(channelID, fmt.Sprintf("🎙️ Starting roundtable. **%s** is leading. Summoning participants...", leaderDisplayName))

	sess := mgr.GetOrCreate(channelID)

	for _, modelName := range toSummon {
		variant := ""
		payload := participantPayload
		if modelName == leaderModel {
			variant = cmd.Variant
			payload = leaderPayload
		}
		sess.AddModel(modelName, variant, payload)
		displayName := agentDisplayName(modelName, variant)
		_ = client.Send(channelID, fmt.Sprintf("📡 Summoning **%s**...", displayName))
		modelName := modelName
		go func() {
			if err := sp.Spawn(ctx, modelName, variant, payload); err != nil {
				slog.Error("roundtable spawn error", "model", modelName, "error", err)
			}
		}()
	}

	sess.SetLeader(leaderModel)

	sess.ResetTimer(cfg.inactivityTimeout, func() {
		announceInactiveDeparture(client, mgr, channelID, sess.Models())
	})
}

func handleSummon(
	ctx context.Context,
	client *discord.Client,
	mgr *session.Manager,
	sp *spawner.Spawner,
	channelID string,
	cmd trigger.Command,
	timeout time.Duration,
) {
	sess := mgr.GetOrCreate(channelID)
	payload := spawner.FormatPayload(cmd.Prompt)

	for _, name := range modelsFromSummonCommand(cmd) {
		if sess.HasModel(name) {
			continue
		}
		_ = client.Send(channelID, fmt.Sprintf("📡 Summoning **%s**. Stand by...", agentDisplayName(name, cmd.Variant)))
		sess.AddModel(name, cmd.Variant, payload)
		name := name
		go func() {
			if err := sp.Spawn(ctx, name, cmd.Variant, payload); err != nil {
				slog.Error("initial spawn error", "model", name, "error", err)
			}
		}()
	}

	sess.ResetTimer(timeout, func() {
		announceInactiveDeparture(client, mgr, channelID, sess.Models())
	})
}

func handleDismiss(client *discord.Client, mgr *session.Manager, channelID string) {
	sess := mgr.Get(channelID)
	if sess == nil {
		return
	}
	sess.StopTimer()
	for _, m := range sess.Models() {
		_ = client.Send(channelID, fmt.Sprintf("**%s** is leaving. o7", agentDisplayName(m.Name, m.Variant)))
	}
	mgr.Remove(channelID)
}

func announceInactiveDeparture(client *discord.Client, mgr *session.Manager, channelID string, models []session.ActiveModel) {
	for _, m := range models {
		_ = client.Send(channelID, fmt.Sprintf("**%s** has gone quiet and stepped out.", agentDisplayName(m.Name, m.Variant)))
	}
	mgr.Remove(channelID)
}

func modelsFromSummonCommand(cmd trigger.Command) []string {
	if cmd.Model == "both" {
		return []string{"claude", "gemini"}
	}
	return []string{cmd.Model}
}

func agentDisplayName(model, variant string) string {
	name := map[string]string{
		"claude":   "BTClaude",
		"gemini":   "BTGemini",
		"deepseek": "BTDeepseek",
	}[model]
	if name == "" {
		name = model
	}
	if variant != "" {
		return fmt.Sprintf("%s (%s)", name, variant)
	}
	return name
}

type config struct {
	summonerToken        string
	btClaudeToken        string
	btGeminiToken        string
	btDeepseekToken      string
	workDir              string
	inactivityTimeout    time.Duration
	claudeDefaultModel   string
	geminiDefaultModel   string
	deepseekCmd          string
	deepseekDefaultModel string
	artifactsDir         string
	roundtableLeader     string
}

func loadConfig() config {
	timeout, err := time.ParseDuration(envOr("INACTIVITY_TIMEOUT", "20m"))
	if err != nil {
		slog.Error("invalid INACTIVITY_TIMEOUT", "error", err)
		os.Exit(1)
	}
	workDir := envOr("WORK_DIR", envOr("NFS_MOUNT", "."))
	return config{
		summonerToken:        requireEnv("SUMMONER_TOKEN"),
		btClaudeToken:        os.Getenv("BTCLAUDE_TOKEN"),
		btGeminiToken:        os.Getenv("BTGEMINI_TOKEN"),
		btDeepseekToken:      os.Getenv("BTDEEPSEEK_TOKEN"),
		workDir:              workDir,
		inactivityTimeout:    timeout,
		claudeDefaultModel:   os.Getenv("CLAUDE_DEFAULT_MODEL"),
		geminiDefaultModel:   os.Getenv("GEMINI_DEFAULT_MODEL"),
		deepseekCmd:          envOr("DEEPSEEK_CMD", "claude-ds"),
		deepseekDefaultModel: os.Getenv("DEEPSEEK_DEFAULT_MODEL"),
		artifactsDir:         envOr("ARTIFACTS_DIR", workDir),
		roundtableLeader:     envOr("ROUNDTABLE_LEADER", "claude"),
	}
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 2: Build check**

```bash
go build ./...
```

Expected: exits 0.

- [ ] **Step 3: Run all tests**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 4: Update the k8s manifest**

Replace `deploy/deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: summoner
  labels:
    app: summoner
spec:
  replicas: 1
  selector:
    matchLabels:
      app: summoner
  template:
    metadata:
      labels:
        app: summoner
    spec:
      containers:
        - name: summoner
          image: summoner:latest
          imagePullPolicy: Always
          ports:
            - containerPort: 8080
              name: healthz
          envFrom:
            - secretRef:
                name: summoner-secrets
          env:
            - name: WORK_DIR
              value: /nfs/shared
            - name: ARTIFACTS_DIR
              value: /nfs/shared/roundtable
            - name: INACTIVITY_TIMEOUT
              value: 20m
            - name: ROUNDTABLE_LEADER
              value: claude
          volumeMounts:
            - name: nfs-shared
              mountPath: /nfs/shared
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 30

        - name: discord-mcp-claude
          image: saseq/discord-mcp:latest
          imagePullPolicy: Always
          env:
            - name: DISCORD_TOKEN
              valueFrom:
                secretKeyRef:
                  name: summoner-secrets
                  key: BTCLAUDE_TOKEN
            - name: SPRING_PROFILES_ACTIVE
              value: http
            - name: SERVER_PORT
              value: "8085"

        - name: discord-mcp-gemini
          image: saseq/discord-mcp:latest
          imagePullPolicy: Always
          env:
            - name: DISCORD_TOKEN
              valueFrom:
                secretKeyRef:
                  name: summoner-secrets
                  key: BTGEMINI_TOKEN
            - name: SPRING_PROFILES_ACTIVE
              value: http
            - name: SERVER_PORT
              value: "8086"

        - name: discord-mcp-deepseek
          image: saseq/discord-mcp:latest
          imagePullPolicy: Always
          env:
            - name: DISCORD_TOKEN
              valueFrom:
                secretKeyRef:
                  name: summoner-secrets
                  key: BTDEEPSEEK_TOKEN
            - name: SPRING_PROFILES_ACTIVE
              value: http
            - name: SERVER_PORT
              value: "8087"

      volumes:
        - name: nfs-shared
          nfs:
            server: REPLACE_WITH_NFS_HOST
            path: /shared
---
apiVersion: v1
kind: Service
metadata:
  name: summoner
spec:
  selector:
    app: summoner
  ports:
    - port: 8080
      targetPort: 8080
      name: healthz
  type: ClusterIP
```

To create/update the secret (add Deepseek entries):

```bash
kubectl create secret generic summoner-secrets \
  --from-literal=SUMMONER_TOKEN=<summoner-bot-token> \
  --from-literal=BTCLAUDE_TOKEN=<btclaude-token> \
  --from-literal=BTGEMINI_TOKEN=<btgemini-token> \
  --from-literal=BTDEEPSEEK_TOKEN=<btdeepseek-token> \
  --from-literal=ANTHROPIC_API_KEY=<anthropic-key> \
  --from-literal=GEMINI_API_KEY=<gemini-key> \
  --from-literal=DEEPSEEK_API_KEY=<deepseek-key> \
  --dry-run=client -o yaml | kubectl apply -f -
```

- [ ] **Step 5: Final build and test**

```bash
go build ./... && go test ./...
```

Expected: exits 0, all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/summoner/main.go deploy/deployment.yaml
git commit -m "feat: roundtable orchestration — leader dispatch, deepseek, allowlist"
```

---

## Self-Review

**Spec coverage:**
- ✅ `@Summoner roundtable [leader] [prompt]` triggers a session
- ✅ All configured models auto-summoned at session start
- ✅ Leader gets ReAct-loop prompt; participants get wait-until-asked prompt
- ✅ Leader re-spawned on human or participant message
- ✅ Participant re-spawned when leader @mentions their bot ID
- ✅ Leader bot is on the @Summoner command allowlist (summon mid-session, dismiss)
- ✅ "Last call!" and consensus → write artifacts → `@Summoner dismiss` flow in leader prompt
- ✅ `ARTIFACTS_DIR` passed to leader via prompt; defaults to `WORK_DIR` or `"."` for local use
- ✅ Deepseek via `claude-ds` (configurable via `DEEPSEEK_CMD`)
- ✅ `ROUNDTABLE_LEADER` env var for default leader when not specified in command
- ✅ Existing non-roundtable summon behavior unchanged
- ✅ `NFS_MOUNT` still honoured (via `WORK_DIR` fallback in `loadConfig`)
- ✅ k8s manifest updated with Deepseek discord-mcp sidecar

**Placeholder scan:** None found.

**Type consistency:**
- `spawner.Config.WorkDir` used in Task 4, referenced correctly in Task 5 `main.go`
- `spawner.Config.DeepseekCmd` / `DeepseekDefaultModel` defined in Task 4, wired in Task 5
- `session.Session.Model(name)` defined in Task 2, called in Task 5 `handleRoundtableMessage`
- `session.Session.ParticipantNames()` defined in Task 2, called in Task 5
- `trigger.Command.Leader` defined in Task 1, read in Task 5 `handleRoundtable`
- `trigger.CommandRoundtable` defined in Task 1, matched in Task 5 message handler
- `spawner.FormatLeaderPayload` / `FormatParticipantPayload` defined in Task 3, called in Task 5
- `spawner.FormatPayload` still exists and called in Task 5 `handleSummon` ✅
