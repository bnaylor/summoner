# Stage 1: build Go binary
FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o summoner ./cmd/summoner

# Stage 2: runtime
# Node LTS is the base — claude-code and gemini-cli are npm globals.
# Rebuild weekly to pick up CLI updates; npm layer is cached separately
# from the Go binary so unchanged code doesn't re-run npm install.
FROM node:22-slim

# System deps for claude-code (it shells out to git, etc.) and claude-ds install
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Non-root user for all spawned agent processes.
# --dangerously-skip-permissions is blocked when running as root,
# so claude/claude-ds run as this user.
RUN useradd -r -u 999 -m -d /home/agent agent

# CLI tools — intentionally unpinned, rebuild weekly to stay current
# gemini-cli is deprecated; antigravity-cli (agy) is its successor and uses
# the same ~/.gemini/ config path.
RUN npm install -g @anthropic-ai/claude-code
RUN curl -fsSL https://antigravity.google/cli/install.sh | bash -s -- --dir /usr/local/bin

# claude-ds: wrapper that routes claude-code at the DeepSeek Anthropic-compatible API.
# Download the script itself; skip the interactive installer.
RUN curl -fsSL https://raw.githubusercontent.com/earchibald/claude-ds/main/claude-ds \
    -o /usr/local/bin/_claude-ds && chmod +x /usr/local/bin/_claude-ds

# HOME-override wrapper so claude-ds and its child `claude` process use
# /home/agent/deepseek-home, keeping BTDeepseek's MCP config (port 8087)
# separate from BTClaude's (port 8085).
RUN printf '#!/bin/sh\nexport HOME=/home/agent/deepseek-home\nexec /usr/local/bin/_claude-ds "$@"\n' \
    > /usr/local/bin/claude-ds && chmod +x /usr/local/bin/claude-ds

# MCP config for agy (BTGemini) — ~/.gemini/settings.json.
# BTClaude and BTDeepseek MCP configs are written to ~/.claude.json at
# container startup by entrypoint.sh (requires runtime API key; can't bake in).
RUN mkdir -p /home/agent/.gemini /home/agent/deepseek-home \
    && chown -R agent:agent /home/agent
COPY deploy/gemini-settings.json /home/agent/.gemini/settings.json
RUN chown /home/agent/.gemini/settings.json agent:agent

# Summoner binary
COPY --from=builder /build/summoner /usr/local/bin/summoner

# Entrypoint writes the claude-ds config from DEEPSEEK_API_KEY at startup,
# then execs summoner. The config cannot be set via env vars alone.
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
