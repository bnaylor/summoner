# Stage 1: build Go binary
FROM golang:1.22-alpine AS builder
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

# System deps for claude-code (it shells out to git, etc.)
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# CLI tools — intentionally unpinned, rebuild weekly to stay current
RUN npm install -g @anthropic-ai/claude-code @google/gemini-cli

# MCP configs for summoned agents
# claude-code reads ~/.claude/settings.json
# gemini-cli reads ~/.gemini/settings.json
RUN mkdir -p /root/.claude /root/.gemini
COPY deploy/claude-settings.json /root/.claude/settings.json
COPY deploy/gemini-settings.json /root/.gemini/settings.json

# Summoner binary
COPY --from=builder /build/summoner /usr/local/bin/summoner

ENTRYPOINT ["summoner"]
