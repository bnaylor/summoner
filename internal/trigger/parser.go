package trigger

import (
	"strings"
)

type CommandType int

const (
	CommandUnknown    CommandType = iota
	CommandSummon
	CommandDismiss
	CommandRoundtable
)

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
