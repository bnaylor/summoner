# Summoner Design Spec

**Date:** 2026-05-13

## Overview

Summoner is a standalone, always-on Discord bot service that allows Hermes bots (and humans) to summon frontier model CLI harnesses — Claude Code and Gemini CLI — into a Discord channel for task-bound design consultations. It is a bridging tool, not a data store. No database, no persistence.

The core use case: Hermes bots use cheaper models day-to-day and call in Claude or Gemini when they want a seasoned second opinion on architecture or design decisions. The summoned agent joins the discussion, contributes as a thoughtful design partner, and leaves when the discussion resolves. The Hermes bots produce any resulting artifacts themselves.

---

## Architecture

Three actors:

- **Summoner bot** — the orchestrator. Always-on Discord bot with its own account. Parses summon/dismiss commands, announces arrivals and departures, manages session state, triggers re-spawns on human turns.
- **BTClaude / BTGemini** — the summoned agents. Short-lived CLI processes (`claude`, `gemini`) spawned by Summoner. Each uses its own Discord bot token and Discord MCP to post directly to the channel. Summoner does not relay their output.
- **Hermes bots / humans** — the callers. They @mention Summoner to start and end sessions, and carry on the conversation that drives re-spawns.

Summoner is a single Go binary. Single process, single replica in k8s. No background workers beyond per-session inactivity timer goroutines.

The NFS share (`/nfs/shared`) is mounted read-only in the Summoner pod. Spawned CLIs run in that directory, giving them access to context files the Hermes bots have placed there. Bots are expected to write a context summary to the NFS share and reference it in their opening summon prompt.

---

## Command Syntax

All commands are Discord messages that @mention the Summoner bot.

### Summon

```
@Summoner <model> [variant] [prompt]
```

| Token | Values |
|---|---|
| `model` | `claude`, `gemini`, `both` |
| `variant` | `opus`, `sonnet`, `haiku` (Claude) · `pro`, `flash` (Gemini) · omit for CLI default |
| `prompt` | remainder of message; opening context for the session |

**Examples:**
```
@Summoner claude let's design the caching layer
@Summoner claude opus we need deep reasoning on this tradeoff
@Summoner gemini pro take a look at what we've got in /shared/myproject
@Summoner both we're stuck on an architecture decision, fresh eyes needed
```

### Dismiss

```
@Summoner dismiss
```

Ends the active session immediately. Summoner announces departure for each active model.

### Rules

- One active session per channel. A second summon while a session is active adds the new model rather than starting fresh.
- `dismiss` while no session is active is a quiet no-op.
- Unrecognized syntax gets a brief help reply from Summoner.

---

## Session Lifecycle

```
@Summoner claude opus let's design the caching layer
        │
        ▼
Parse: model=claude, variant=opus, prompt="let's design..."
Post:  "📡 Summoning BTClaude (opus). Stand by..."
Session created, inactivity timer started (default 20m)
        │
        ▼
Spawn: claude --model claude-opus-4-7 -p "<payload>"
  env: DISCORD_BOT_TOKEN=<btclaude token>
  cwd: /nfs/shared
        │
        ▼
BTClaude posts to Discord via Discord MCP (direct, not relayed)
Session records message as agent turn
        │
        ▼
Human/Hermes posts ──► session records, inactivity timer resets
                        spawner re-execs claude with same payload
        │
        ▼
@Summoner dismiss  ──► "BTClaude is leaving. o7"
  OR inactivity    ──► "BTClaude has gone quiet and stepped out."
        │
        ▼
Session removed
```

Re-spawns happen on human/Hermes turns only — not when a summoned agent posts. This prevents agent feedback loops and maps naturally to conversational cadence.

---

## Spawn Payload

The payload passed via `-p` to the spawned CLI is the same on every re-spawn for a session. The summoned agent's continuity comes from reading Discord channel history (via MCP) and the NFS share — not from Summoner replaying messages.

```
You are being summoned as a seasoned architect to join an ongoing technical
discussion on Discord. A colleague has called you in with the following
context: <initial prompt>

Read the recent channel history and any relevant files in your working
directory to get up to speed, then engage as a thoughtful design partner.
Ask clarifying questions, surface tradeoffs, and push back where appropriate.

You are a guest in this conversation — be deliberate, not hasty. Do not
produce implementation artifacts; the team will handle those after consensus.

When you sense the discussion has reached consensus, say so clearly and
indicate you are stepping out.
```

---

## Components

```
cmd/summoner/
  main.go          — entrypoint, Discord event loop, signal handling

internal/
  trigger/
    parser.go      — parses @mention messages; extracts model, variant, prompt
  session/
    manager.go     — in-memory map of channelID → Session
    session.go     — Session struct: active models, inactivity timer
  spawner/
    spawner.go     — execs CLI with payload; sets token env vars
    payload.go     — formats the -p string
  discord/
    client.go      — thin discordgo wrapper (connect, send, register handlers)
```

The trigger parser is pure string logic — no LLM. It scans for the Summoner mention ID, tokenizes the remainder, and extracts model/variant/rest.

The session manager is the coordination hub. It is the only component that knows what models are active in which channel. Re-spawn decisions happen here.

The spawner sets the appropriate Discord token env var before exec'ing the CLI. BTClaude gets its token; BTGemini gets its own. Each spawned process has access only to its own token.

---

## Configuration

All configuration via environment variables:

| Variable | Purpose | Default |
|---|---|---|
| `SUMMONER_TOKEN` | Summoner bot's Discord token | required |
| `BTCLAUDE_TOKEN` | Passed to `claude` spawns as Discord MCP token | required if claude used |
| `BTGEMINI_TOKEN` | Passed to `gemini` spawns as Discord MCP token | required if gemini used |
| `NFS_MOUNT` | Working directory for spawned CLIs | `/nfs/shared` |
| `INACTIVITY_TIMEOUT` | Session idle timeout | `20m` |
| `CLAUDE_DEFAULT_MODEL` | Claude model when no variant specified | CLI default |
| `GEMINI_DEFAULT_MODEL` | Gemini model when no variant specified | CLI default |

---

## Container

The discord-mcp server ([SaseQ/discord-mcp](https://github.com/SaseQ/discord-mcp)) is Java/Spring Boot. Rather than embed Java in the main container, it runs as a per-bot sidecar in HTTP mode. Each spawned CLI connects to its sidecar over localhost.

### Main container (`summoner`)

Multi-stage Dockerfile:

```dockerfile
# Stage 1: build Go binary
FROM golang:1.22 AS builder
WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 go build -o summoner ./cmd/summoner

# Stage 2: runtime
FROM node:22-slim

# Claude Code and Gemini CLI
RUN npm install -g @anthropic-ai/claude-code @google/gemini-cli

# Pre-bake Claude MCP config (discord-mcp sidecar at localhost:8085)
RUN mkdir -p /root/.claude
COPY deploy/claude-settings.json /root/.claude/settings.json

# Pre-bake Gemini MCP config (discord-mcp sidecar at localhost:8086)
COPY deploy/gemini-settings.json /root/.gemini/settings.json

# Summoner binary
COPY --from=builder /build/summoner /usr/local/bin/summoner

ENTRYPOINT ["summoner"]
```

`deploy/claude-settings.json` configures the discord MCP as an HTTP server at `http://localhost:8085/sse`. `deploy/gemini-settings.json` does the same at port 8086.

### Sidecars

Two instances of `saseq/discord-mcp:latest`, one per bot, run alongside the main container in the same pod. They start in HTTP mode (`SPRING_PROFILES_ACTIVE=http`) on different ports.

---

## Deployment

Single-replica k8s Deployment. No HA concerns — sessions are in-memory in one process, and session loss on pod restart is acceptable (short sessions by design).

```yaml
containers:
  - name: summoner
    image: summoner:latest
    envFrom:
      - secretRef:
          name: summoner-secrets
    env:
      - name: NFS_MOUNT
        value: /nfs/shared
    volumeMounts:
      - name: nfs-shared
        mountPath: /nfs/shared
        readOnly: true

  - name: discord-mcp-claude
    image: saseq/discord-mcp:latest
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

volumes:
  - name: nfs-shared
    nfs:
      server: <nfs-host>
      path: /shared
      readOnly: true
```

A simple `GET /healthz → 200` endpoint is sufficient for liveness. discordgo handles reconnection automatically.

---

## What Summoner Is Not

- Not a data store. No SQLite, no PVC for state.
- Not a message relay. Summoned agents post directly; Summoner only posts its own announcements.
- Not a reasoning layer. Trigger detection is @mention parsing, no LLM classifier.
- Not an artifact producer. Consensus and follow-up are the calling bots' responsibility.
