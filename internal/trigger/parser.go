package trigger

import (
	"strings"
)

// CommandType identifies the kind of command parsed from a Discord mention.
type CommandType int

const (
	CommandUnknown    CommandType = iota // mention present but unrecognized token
	CommandSummon                        // request to summon a model
	CommandDismiss                       // request to end the active session
	CommandRoundtable                    // start a structured multi-model design session
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

	found := false
	for _, m := range []string{mention, mentionBang} {
		if strings.Contains(content, m) {
			content = strings.Replace(content, m, "", 1)
			found = true
			break
		}
	}
	if !found {
		return Command{}, false
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
	// deepseek has no defined variants
	return false
}
