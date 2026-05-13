package trigger

import (
	"strings"
)

// CommandType identifies the kind of command parsed from a Discord mention.
type CommandType int

const (
	CommandUnknown CommandType = iota // mention present but unrecognized token
	CommandSummon                     // request to summon a model
	CommandDismiss                    // request to end the active session
)

// Command is the result of parsing a Discord message that mentions the Summoner bot.
type Command struct {
	Type    CommandType
	Model   string // "claude", "gemini", "both"
	Variant string // "opus", "sonnet", "haiku", "pro", "flash", or ""
	Prompt  string
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

	if first == "dismiss" {
		return Command{Type: CommandDismiss}, true
	}

	if first != "claude" && first != "gemini" && first != "both" {
		return Command{Type: CommandUnknown}, true
	}

	cmd := Command{Type: CommandSummon, Model: first}
	rest := tokens[1:]

	if len(rest) > 0 {
		second := strings.ToLower(rest[0])
		isVariant := false
		switch first {
		case "claude":
			isVariant = claudeVariants[second]
		case "gemini":
			isVariant = geminiVariants[second]
		case "both":
			isVariant = claudeVariants[second] || geminiVariants[second]
		}
		if isVariant {
			cmd.Variant = second
			rest = rest[1:]
		}
	}

	cmd.Prompt = strings.Join(rest, " ")
	return cmd, true
}
