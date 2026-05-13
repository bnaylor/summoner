# Summoner concept

I want to lift the discord relay and sentry out of ~/src/perry/ and make it a single-purpose
service, which is to sit on discord as the "Summoner" and allow my Hermes bots to call on
specific frontier models *via their native cli harnesses* to join discord for task-bound
consulting sessions.

It's my always-on agents being able to call in big guns for help on things (design rountable)
etc without natively using those models directly.

This will be useful specifically within the context of Claude because it is cost-prohibitive
to use Anthropic models with non-Anthropic harnesses at the moment - so they can continue
to use Deepseek, Gemini, local models, etc, and call on Claude for help when needed 
via summoning.

That's really the only use case at the moment, but may as well add support for gemini cli
in order to follow the design "rule of two".

Read the docs in ~/src/perry about how the Sentry and the discord relay work.  
The rest should be pretty clear.

The only real change I'd make is that the Sentry doesn't really need to think quite as
hard here.  A more explicit signal of "let's summon Claude" or "Let's get some help
from Claude Opus" or "Maybe Gemini Pro could help us with this" would be candidates
for spawn triggers.

- Summoner should indicate in the channel when it is spawning an agent
- Summoned agents can be dismissed with a clear signal "ok, thanks for the help!" etc.
- After a period of inactivity (10m? 20m?) the agent should exit regardless.
- Summoner and/or the agent should, in this case, treat the place like an airport and 
announce their departure just so we're not left hanging, thinking they're still around.
- Explicit models should be enabled as part of the request (we need Mythos for this, etc)
- The summoned agent should implement the Perry Architect Roundable protocol
    - /Users/bnaylor/src/perry/docs/agentic_design/architects.txt
    - /Users/bnaylor/src/perry/docs/agentic_design/proposal_architect_roundtable.md
- Unlike prior versions of rountable implementations, I expect to run this in k8s as 
an always-on workload.
