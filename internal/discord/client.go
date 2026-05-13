package discord

import (
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

// Client wraps a discordgo session for the Summoner bot.
type Client struct {
	s  *discordgo.Session
	id string
}

// New connects to Discord with the given bot token and returns a ready Client.
func New(token string) (*Client, error) {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discord.New: %w", err)
	}
	s.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentDirectMessages

	if err := s.Open(); err != nil {
		return nil, fmt.Errorf("discord Open: %w", err)
	}

	return &Client{s: s, id: s.State.User.ID}, nil
}

// ID returns the Summoner bot's own Discord user ID.
func (c *Client) ID() string { return c.id }

// Send posts a message to a channel.
func (c *Client) Send(channelID, content string) error {
	_, err := c.s.ChannelMessageSend(channelID, content)
	return err
}

// OnMessage registers a handler called for every MessageCreate event.
func (c *Client) OnMessage(fn func(*discordgo.MessageCreate)) {
	c.s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		fn(m)
	})
}

// Close disconnects from Discord.
func (c *Client) Close() {
	if err := c.s.Close(); err != nil {
		slog.Warn("discord close error", "error", err)
	}
}

// LookupBotID returns the Discord user ID for a given bot token.
// Used at startup to resolve the user IDs of BTClaude and BTGemini.
func LookupBotID(token string) (string, error) {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return "", fmt.Errorf("LookupBotID: %w", err)
	}
	u, err := s.User("@me")
	if err != nil {
		return "", fmt.Errorf("LookupBotID user lookup: %w", err)
	}
	return u.ID, nil
}
