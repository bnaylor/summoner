# Rethink the Architect Roundtable

One of the most valuable things I've gotten in recent months out of exerimenting with AI coding
and agentic flows and whatnot has been the "Architect Roundtable" that we were doing with Perry
initially.  We've had a few iterations of this, and I've been trying to get my Hermes bots to
do something similar.  That's not going super amazingly, they really like to devolve into classic
bot cascades and getting that to stop has been harder than you might think.

I'd like to take a pause and revisit this whole notion and plot a course forward to keep this
practice front and center.  The different options and half-working schemes are leading me
to not do this, which I think results in worse designs.

## Background
First, let's talk about history.  We've taken a couple of major cracks at this:
* Original discord coordination skill - wasn't too bad, but took too much work on my end and was haphazard
* Perry's design layer - formalize discord-coordination but it was baked into a much bigger flow
* The Summoner - pluck out the architects part from Perry and just keep that - have not really used this yet

Catch up on this concept by reading the main prior artifacts:
* /Users/bnaylor/src/skills/discord-coordination/SKILL.md
* /Users/bnaylor/src/perry/docs/agentic_design/proposal_architect_roundtable.md
* /Users/bnaylor/src/perry/docs/agentic_design/architects.txt
* /Users/bnaylor/src/perry/docs/agentic_design/design-layer-architecture.md
* /Users/bnaylor/src/summoner/docs/concept.md
* /Users/bnaylor/src/summoner/README.md

The Hermes bots also came up with their own proposals, including a NATS-based backchannel and an NFS share:
* ./tmp/*

## Going Forward
I want to revive this in a usable way.  Summoner might be almost the right answer, but I'd like to
stop and think about it and consider all the angles.

This project directory may just be nothing more than a research item; if summoner is pretty close,
the "output" here can just be a decision to tweak that a little and keep going with it.

Or, we could pivot to another solution and build/adapt something new.

Before I list any new requirements, note also some turnkey infrastructure we could use and
should evaluate against the homegrown connection solutions:
* https://github.com/Open-ACP

## Refined requirements
* The basic ideas and flows from the prior efforts are all good and should largely remain intact
* I would like to expand this to support n agents, to start with adding Deepseek to the mix
* The instructions for agents may need a bit more structure; even with just claude and gemini reacting to one another, things could get a little crazy
  * I think we should probably have the human propose a 'leader' for a design session, and that agent can drive the discussion and solicit inputs
  * The leader is responsible for collecting and writing output docs, designs, specs, whatever, OR delegating tasks to other agents.
  * Agents should not *do* anything with code, docs, etc on their own unless explicitly asked by the leader.  Gemini in particular likes to run off and start doing things.
  * Agents should be mindful of spam; let's have the leader drive the discussion and ask agents for their feedback each round.
  * Before closing a discussion, the leader should explicitly confirm consensus and check for any lingering feedback.  "Last call!"
* The summoner model, if I recall correctly, probably did not provide enough continual looping / agency for a leader to drive things.  We need a ReAct-style cycle at least for that agent.

P1: Figure out how to enable the Hermes bots to at least absorb the conversation.  If directly taking chat input is too tempting for them, maybe this happens via a curated summary, or they can just read the output docs also.  It's nice if they have the design context and decisions already made so they don't try to second-guess.
