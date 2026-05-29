package spawner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
// instructionsDir is the path to check for a LEADER.md override file; empty disables injection.
func FormatLeaderPayload(topic, artifactsDir string, participants []string, instructionsDir string) string {
	participantList := "  - " + strings.Join(participants, "\n  - ")
	base := fmt.Sprintf(leaderTemplate, topic, participantList, artifactsDir)
	if extra := readInstruction(instructionsDir, "LEADER.md"); extra != "" {
		base += "\n\n## Additional Instructions\n\n" + extra
	}
	return base
}

// FormatParticipantPayload returns the -p payload for a roundtable participant.
// leaderDisplayName is the Discord display name of the leader (e.g. "BTClaude").
// instructionsDir is the path to check for a PARTICIPANT.md override file; empty disables injection.
func FormatParticipantPayload(topic, leaderDisplayName, instructionsDir string) string {
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
	base := fmt.Sprintf(tmpl, topic, leaderDisplayName)
	if extra := readInstruction(instructionsDir, "PARTICIPANT.md"); extra != "" {
		base += "\n\n## Additional Instructions\n\n" + extra
	}
	return base
}

// instruction cache — mtime-keyed so stale files are reloaded automatically.
type cachedFile struct {
	content string
	modTime time.Time
}

var (
	fileCacheMu sync.Mutex
	fileCache   = map[string]cachedFile{}
)

// readInstruction returns the contents of name inside dir, or "" if the file
// does not exist or cannot be read. Results are cached by path and reloaded
// whenever the file's modification time changes.
func readInstruction(dir, name string) string {
	if dir == "" {
		return ""
	}
	path := filepath.Join(dir, name)

	info, err := os.Stat(path)
	if err != nil {
		return ""
	}

	fileCacheMu.Lock()
	cached, hit := fileCache[path]
	fileCacheMu.Unlock()

	if hit && !info.ModTime().After(cached.modTime) {
		return cached.content
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	entry := cachedFile{content: string(data), modTime: info.ModTime()}
	fileCacheMu.Lock()
	fileCache[path] = entry
	fileCacheMu.Unlock()

	return entry.content
}
