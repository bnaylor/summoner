package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	logLevel := slog.LevelInfo
	if s := os.Getenv("LOG_LEVEL"); s != "" {
		if err := logLevel.UnmarshalText([]byte(s)); err != nil {
			fmt.Fprintf(os.Stderr, "invalid LOG_LEVEL %q, using info\n", s)
		}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

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
		preview := m.Content
		if len(preview) > 120 {
			preview = preview[:120] + "…"
		}
		isSelf := m.Author.ID == client.ID()
		slog.Debug("message event",
			"channel", m.ChannelID,
			"author", m.Author.ID,
			"username", m.Author.Username,
			"is_self", isSelf,
			"is_agent", mgr.IsAgent(m.Author.ID),
			"content_len", len(m.Content),
			"content", preview,
		)
		if isSelf {
			return
		}

		channelID := m.ChannelID
		sess := mgr.Get(channelID)

		// Roundtable sessions need special dispatch.
		if sess != nil && sess.IsRoundtable() {
			slog.Debug("roundtable dispatch", "channel", channelID, "leader", sess.LeaderModel())
			handleRoundtableMessage(ctx, client, mgr, sp, m, sess, modelBotIDs, cfg)
			return
		}

		// @Summoner command — only accepted from humans in non-roundtable context.
		cmd, isSummonerCmd := trigger.Parse(m.Content, client.ID())
		if isSummonerCmd {
			slog.Debug("summoner command parsed", "channel", channelID, "type", cmd.Type, "model", cmd.Model, "leader", cmd.Leader)
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
		if sess == nil {
			slog.Debug("no active session, ignoring", "channel", channelID)
			return
		}
		if mgr.IsAgent(m.Author.ID) {
			slog.Debug("agent message in non-roundtable session, ignoring", "channel", channelID, "author", m.Author.ID)
			return
		}
		models := sess.Models()
		slog.Debug("re-spawning all models", "channel", channelID, "count", len(models))
		for _, model := range models {
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
		http.HandleFunc("/debug/sessions", func(w http.ResponseWriter, r *http.Request) {
			sessions, agentIDs := mgr.Snapshot()
			payload := map[string]any{
				"summoner_id": client.ID(),
				"agent_ids":   agentIDs,
				"sessions":    sessions,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		})
		if err := http.ListenAndServe(":8080", nil); err != nil {
			slog.Error("http server error", "error", err)
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
				model, ok := sess.Model(participantName)
				if !ok {
					continue
				}
				go func() {
					if err := sp.Spawn(ctx, model.Name, model.Variant, model.Prompt); err != nil {
						slog.Error("participant spawn error", "model", model.Name, "error", err)
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
	instructionsDir := filepath.Join(cfg.workDir, "instructions")
	leaderPayload := spawner.FormatLeaderPayload(channelID, cmd.Prompt, cfg.artifactsDir, participantDisplayNames, instructionsDir)
	participantPayload := spawner.FormatParticipantPayload(channelID, cmd.Prompt, leaderDisplayName, instructionsDir)

	_ = client.Send(channelID, fmt.Sprintf("Starting roundtable. **%s** is leading. Summoning participants...", leaderDisplayName))

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
		_ = client.Send(channelID, fmt.Sprintf("Summoning **%s**...", displayName))
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
	payload := spawner.FormatPayload(channelID, cmd.Prompt)

	for _, name := range modelsFromSummonCommand(cmd) {
		if sess.HasModel(name) {
			continue
		}
		_ = client.Send(channelID, fmt.Sprintf("Summoning **%s**. Stand by...", agentDisplayName(name, cmd.Variant)))
		sess.AddModel(name, cmd.Variant, payload)
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
