package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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

	// Resolve agent bot user IDs so we can suppress re-spawns on their messages.
	var agentIDs []string
	for _, entry := range []struct{ name, token string }{
		{"BTClaude", cfg.btClaudeToken},
		{"BTGemini", cfg.btGeminiToken},
	} {
		if entry.token == "" {
			continue
		}
		id, err := discord.LookupBotID(entry.token)
		if err != nil {
			slog.Warn("could not resolve agent bot ID", "agent", entry.name, "error", err)
			continue
		}
		slog.Info("resolved agent ID", "agent", entry.name, "id", id)
		agentIDs = append(agentIDs, id)
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
		NFSMount:           cfg.nfsMount,
		ClaudeDefaultModel: cfg.claudeDefaultModel,
		GeminiDefaultModel: cfg.geminiDefaultModel,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client.OnMessage(func(m *discordgo.MessageCreate) {
		if m.Author.ID == client.ID() {
			return
		}

		sess := mgr.Get(m.ChannelID)
		cmd, isSummonerCmd := trigger.Parse(m.Content, client.ID())

		if isSummonerCmd {
			switch cmd.Type {
			case trigger.CommandSummon:
				handleSummon(ctx, client, mgr, sp, m.ChannelID, cmd, cfg.inactivityTimeout)
			case trigger.CommandDismiss:
				handleDismiss(client, mgr, m.ChannelID)
			default:
				_ = client.Send(m.ChannelID, "Usage: `@Summoner <claude|gemini|both> [variant] [prompt]` or `@Summoner dismiss`")
			}
			return
		}

		if sess == nil || mgr.IsAgent(m.Author.ID) {
			return
		}

		for _, model := range sess.Models() {
			m := model
			go func() {
				if err := sp.Spawn(ctx, m.Name, m.Variant, m.Prompt); err != nil {
					slog.Error("spawn error", "model", m.Name, "error", err)
				}
			}()
		}

		sess.ResetTimer(cfg.inactivityTimeout, func() {
			announceInactiveDeparture(client, mgr, m.ChannelID, sess.Models())
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
	for _, name := range modelsFromCommand(cmd) {
		if !sess.HasModel(name) {
			_ = client.Send(channelID, fmt.Sprintf("📡 Summoning **%s**. Stand by...", agentDisplayName(name, cmd.Variant)))
			sess.AddModel(name, cmd.Variant, cmd.Prompt)
			name := name
			go func() {
				if err := sp.Spawn(ctx, name, cmd.Variant, cmd.Prompt); err != nil {
					slog.Error("initial spawn error", "model", name, "error", err)
				}
			}()
		}
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

func modelsFromCommand(cmd trigger.Command) []string {
	if cmd.Model == "both" {
		return []string{"claude", "gemini"}
	}
	return []string{cmd.Model}
}

func agentDisplayName(model, variant string) string {
	name := map[string]string{"claude": "BTClaude", "gemini": "BTGemini"}[model]
	if variant != "" {
		return fmt.Sprintf("%s (%s)", name, variant)
	}
	return name
}

type config struct {
	summonerToken      string
	btClaudeToken      string
	btGeminiToken      string
	nfsMount           string
	inactivityTimeout  time.Duration
	claudeDefaultModel string
	geminiDefaultModel string
}

func loadConfig() config {
	timeout, err := time.ParseDuration(envOr("INACTIVITY_TIMEOUT", "20m"))
	if err != nil {
		slog.Error("invalid INACTIVITY_TIMEOUT", "error", err)
		os.Exit(1)
	}
	return config{
		summonerToken:      requireEnv("SUMMONER_TOKEN"),
		btClaudeToken:      os.Getenv("BTCLAUDE_TOKEN"),
		btGeminiToken:      os.Getenv("BTGEMINI_TOKEN"),
		nfsMount:           envOr("NFS_MOUNT", "/nfs/shared"),
		inactivityTimeout:  timeout,
		claudeDefaultModel: os.Getenv("CLAUDE_DEFAULT_MODEL"),
		geminiDefaultModel: os.Getenv("GEMINI_DEFAULT_MODEL"),
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
